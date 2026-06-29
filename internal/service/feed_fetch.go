package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	contentutil "shiliu/pkg/content"
)

const (
	initialFetchItemLimit = 50
	maxFeedResponseBytes  = int64(10 << 20)
	staleFeedFetchAfter   = 30 * time.Minute
)

type FetchResultStatus string

const (
	FetchResultStatusSuccess FetchResultStatus = "success"
	FetchResultStatusSkipped FetchResultStatus = "skipped"
	FetchResultStatusFailed  FetchResultStatus = "failed"
)

// Fetcher is the network boundary for retrieving feed XML. Tests inject this
// interface so feed parsing and persistence run without real network access.
type Fetcher interface {
	Fetch(ctx context.Context, feedURL string) ([]byte, error)
}

type hostResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type HTTPFetcher struct {
	client             *http.Client
	resolver           hostResolver
	resolveBeforeFetch bool
}

func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	return newHTTPFetcher(client, net.DefaultResolver)
}

func newHTTPFetcher(client *http.Client, resolver hostResolver) *HTTPFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	clientCopy := *client
	safeTransport, resolveBeforeFetch := safeFeedTransport(clientCopy.Transport, resolver)
	clientCopy.Transport = safeTransport
	previousCheckRedirect := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateHTTPFeedURL(req.URL); err != nil {
			return err
		}
		if resolveBeforeFetch {
			if _, err := resolvePublicHostIPs(req.Context(), resolver, req.URL.Hostname()); err != nil {
				return err
			}
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &HTTPFetcher{client: &clientCopy, resolver: resolver, resolveBeforeFetch: resolveBeforeFetch}
}

func safeFeedTransport(transport http.RoundTripper, resolver hostResolver) (http.RoundTripper, bool) {
	if transport != nil {
		base, ok := transport.(*http.Transport)
		if !ok {
			return transport, false
		}
		clone := base.Clone()
		clone.DialContext = publicDialContext(resolver)
		return clone, true
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = publicDialContext(resolver)
	return base, true
}

func publicDialContext(resolver hostResolver) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolvePublicHostIPs(ctx, resolver, host)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, ipAddr := range ips {
			if !ipMatchesNetwork(network, ipAddr.IP) {
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipAddr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("no %s address for %s", network, host)
	}
}

func ipMatchesNetwork(network string, ip net.IP) bool {
	switch network {
	case "tcp4":
		return ip.To4() != nil
	case "tcp6":
		return ip.To4() == nil
	default:
		return true
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, feedURL string) ([]byte, error) {
	parsedURL, err := url.Parse(feedURL)
	if err != nil {
		return nil, v1.ErrFeedInvalidURL
	}
	if err := validateHTTPFeedURL(parsedURL); err != nil {
		return nil, err
	}
	if f.resolveBeforeFetch {
		if _, err := resolvePublicHostIPs(ctx, f.resolver, parsedURL.Hostname()); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected feed status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxFeedResponseBytes {
		return nil, fmt.Errorf("feed response too large: %d bytes", resp.ContentLength)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxFeedResponseBytes {
		return nil, fmt.Errorf("feed response too large: exceeds %d bytes", maxFeedResponseBytes)
	}
	return body, nil
}

type FeedFetchService interface {
	FetchFeed(ctx context.Context, feed *model.Feed) (*FetchFeedResult, error)
}

type FetchFeedResult struct {
	FeedID               uint
	FeedURL              string
	Status               FetchResultStatus
	Message              string
	FetchedItems         int
	InsertedItems        int
	SkippedExistingItems int
}

func NewFeedFetchService(
	service *Service,
	feedRepo repository.FeedRepository,
	contentRepo repository.ContentItemRepository,
	fetcher Fetcher,
) FeedFetchService {
	return &feedFetchService{
		Service:     service,
		feedRepo:    feedRepo,
		contentRepo: contentRepo,
		fetcher:     fetcher,
	}
}

type feedFetchService struct {
	feedRepo    repository.FeedRepository
	contentRepo repository.ContentItemRepository
	fetcher     Fetcher
	*Service
}

func (s *feedFetchService) FetchFeed(ctx context.Context, feed *model.Feed) (*FetchFeedResult, error) {
	if feed == nil || feed.Id == 0 {
		return nil, v1.ErrBadRequest
	}
	feedURL, err := NormalizeFeedURL(feed.FeedURL)
	if err != nil {
		return nil, err
	}
	if s.fetcher == nil {
		return nil, v1.ErrFeedFetchFailed
	}
	startedAt := time.Now().UTC()
	acquired, err := s.feedRepo.ClaimFetch(ctx, feed.Id, startedAt, startedAt.Add(-staleFeedFetchAfter))
	if err != nil {
		return nil, err
	}
	if !acquired {
		return skippedFetchResult(feed.Id, feedURL, "feed fetch already in progress; skipped"), nil
	}
	workCtx := context.WithoutCancel(ctx)
	body, err := s.fetcher.Fetch(ctx, feedURL)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, s.releaseFetchClaim(workCtx, feed.Id, startedAt, err)
		}
		return s.finishFetchFailure(workCtx, feed.Id, feedURL, startedAt, mapFeedFetchError(err))
	}
	parsed, err := parseRSSFeed(body)
	if err != nil {
		fetchErr := fmt.Errorf("%w: %v", v1.ErrFeedParseFailed, err)
		return s.finishFetchFailure(workCtx, feed.Id, feedURL, startedAt, fetchErr)
	}
	result := &FetchFeedResult{FeedID: feed.Id, FeedURL: feedURL, Status: FetchResultStatusSuccess, FetchedItems: len(parsed.Items)}
	fetchedAt := time.Now().UTC()
	feedTitle := parsed.Title
	err = s.tm.Transaction(workCtx, func(ctx context.Context) error {
		if err := s.feedRepo.UpdateTitle(ctx, feed.Id, feedTitle); err != nil {
			return err
		}
		inserted, skippedExisting, err := persistParsedContentItems(ctx, s.contentRepo, feed.Id, parsed.Items, fetchedAt)
		if err != nil {
			return err
		}
		owned, err := s.feedRepo.UpdateFetchStateIfOwned(ctx, feed.Id, startedAt, model.FeedFetchStatusSuccess, nil, &fetchedAt, nil)
		if err != nil {
			return err
		}
		if !owned {
			return v1.ErrFeedFetchInProgress
		}
		result.InsertedItems = inserted
		result.SkippedExistingItems = skippedExisting
		return nil
	})
	if err != nil {
		if errors.Is(err, v1.ErrFeedFetchInProgress) {
			return skippedFetchResult(feed.Id, feedURL, "feed fetch ownership lost; skipped"), nil
		}
		return s.finishFetchFailure(workCtx, feed.Id, feedURL, startedAt, err)
	}
	return result, nil
}

func skippedFetchResult(feedID uint, feedURL string, message string) *FetchFeedResult {
	return &FetchFeedResult{
		FeedID:  feedID,
		FeedURL: feedURL,
		Status:  FetchResultStatusSkipped,
		Message: message,
	}
}

func (s *feedFetchService) releaseFetchClaim(ctx context.Context, feedID uint, claimedFetchStartedAt time.Time, cause error) error {
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	owned, err := s.feedRepo.ReleaseFetchClaimIfOwned(finishCtx, feedID, claimedFetchStartedAt)
	if err != nil {
		return errors.Join(cause, err)
	}
	if !owned {
		return errors.Join(cause, v1.ErrFeedFetchInProgress)
	}
	return cause
}

func (s *feedFetchService) finishFetchFailure(ctx context.Context, feedID uint, feedURL string, claimedFetchStartedAt time.Time, fetchErr error) (*FetchFeedResult, error) {
	finishedAt := time.Now().UTC()
	message := fetchErr.Error()
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	owned, err := s.feedRepo.UpdateFetchStateIfOwned(finishCtx, feedID, claimedFetchStartedAt, model.FeedFetchStatusFailed, nil, &finishedAt, &message)
	if err != nil {
		return nil, err
	}
	if !owned {
		return skippedFetchResult(feedID, feedURL, "feed fetch ownership lost; skipped"), nil
	}
	return nil, fetchErr
}

func itemsToPersist(items []parsedFeedItem) []parsedFeedItem {
	if len(items) <= initialFetchItemLimit {
		return items
	}
	return newestFeedItems(items)
}

func newestFeedItems(items []parsedFeedItem) []parsedFeedItem {
	selected := append([]parsedFeedItem(nil), items...)
	sort.SliceStable(selected, func(i, j int) bool {
		left, right := selected[i].PublishedAt, selected[j].PublishedAt
		switch {
		case left != nil && right != nil:
			return left.After(*right)
		case left != nil:
			return true
		case right != nil:
			return false
		default:
			return false
		}
	})
	return selected[:initialFetchItemLimit]
}

func NormalizeFeedURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", v1.ErrFeedInvalidURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", v1.ErrFeedInvalidURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.User != nil {
		return "", v1.ErrFeedInvalidURL
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" {
		return "", v1.ErrFeedInvalidURL
	}
	port := parsed.Port()
	if isDefaultPort(parsed.Scheme, port) {
		port = ""
	}
	parsed.Host = formatHostPort(host, port)
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateHTTPFeedURL(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return v1.ErrFeedInvalidURL
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || host == "localhost" {
		return v1.ErrFeedInvalidURL
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedFeedIP(ip) {
		return v1.ErrFeedInvalidURL
	}
	return nil
}

func resolvePublicHostIPs(ctx context.Context, resolver hostResolver, host string) ([]net.IPAddr, error) {
	normalizedHost := strings.TrimSuffix(strings.ToLower(host), ".")
	if normalizedHost == "" || normalizedHost == "localhost" {
		return nil, v1.ErrFeedInvalidURL
	}
	if ip := net.ParseIP(normalizedHost); ip != nil {
		if isBlockedFeedIP(ip) {
			return nil, v1.ErrFeedInvalidURL
		}
		return []net.IPAddr{{IP: ip}}, nil
	}
	ips, err := resolver.LookupIPAddr(ctx, normalizedHost)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, v1.ErrFeedInvalidURL
	}
	for _, ipAddr := range ips {
		if isBlockedFeedIP(ipAddr.IP) {
			return nil, v1.ErrFeedInvalidURL
		}
	}
	return ips, nil
}

func isBlockedFeedIP(ip net.IP) bool {
	return ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

func isDefaultPort(scheme string, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}

func formatHostPort(host string, port string) string {
	if port != "" {
		return net.JoinHostPort(host, port)
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

type parsedFeed struct {
	Type  model.FeedType
	Title string
	Items []parsedFeedItem
}

type parsedFeedItem struct {
	GUID        string
	Link        string
	Title       string
	Description string
	Content     string
	ShowNotes   string
	PublishedAt *time.Time
	Enclosures  []parsedEnclosure
}

type parsedEnclosure struct {
	URL  string
	Type string
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	XMLName        xml.Name            `xml:"channel"`
	Title          string              `xml:"title"`
	ITunesAuthor   string              `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd author"`
	ITunesCategory []rssITunesCategory `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd category"`
	ITunesExplicit string              `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd explicit"`
	ITunesImage    rssITunesImage      `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
	ITunesOwner    rssITunesOwner      `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd owner"`
	ITunesType     string              `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd type"`
	Items          []rssItem           `xml:"item"`
}

type rssITunesCategory struct {
	Text string `xml:"text,attr"`
}

type rssITunesImage struct {
	Href string `xml:"href,attr"`
}

type rssITunesOwner struct {
	Name  string `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd name"`
	Email string `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd email"`
}

type rssItem struct {
	GUID           string         `xml:"guid"`
	Title          string         `xml:"title"`
	Link           string         `xml:"link"`
	Description    string         `xml:"description"`
	ContentEncoded string         `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	ITunesSummary  string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd summary"`
	PubDate        string         `xml:"pubDate"`
	Enclosures     []rssEnclosure `xml:"enclosure"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

func parseRSSFeed(body []byte) (*parsedFeed, error) {
	var doc rssDocument
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc.XMLName.Local != "rss" || doc.Channel.XMLName.Local != "channel" {
		return nil, errors.New("unsupported rss feed")
	}
	items := make([]parsedFeedItem, 0, len(doc.Channel.Items))
	for _, item := range doc.Channel.Items {
		parsed := parsedFeedItem{
			GUID:        strings.TrimSpace(item.GUID),
			Link:        strings.TrimSpace(item.Link),
			Title:       strings.TrimSpace(item.Title),
			Description: strings.TrimSpace(item.Description),
			Content:     strings.TrimSpace(item.ContentEncoded),
			ShowNotes:   strings.TrimSpace(item.ITunesSummary),
			PublishedAt: parseFeedTime(item.PubDate),
			Enclosures:  make([]parsedEnclosure, 0, len(item.Enclosures)),
		}
		for _, enclosure := range item.Enclosures {
			parsed.Enclosures = append(parsed.Enclosures, parsedEnclosure{
				URL:  strings.TrimSpace(enclosure.URL),
				Type: strings.TrimSpace(enclosure.Type),
			})
		}
		items = append(items, parsed)
	}
	return &parsedFeed{Title: strings.TrimSpace(doc.Channel.Title), Type: feedTypeForRSSChannel(doc.Channel, items), Items: items}, nil
}

func parseFeedTime(raw string) *time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if parsed, err := mail.ParseDate(trimmed); err == nil {
		utc := parsed.UTC()
		return &utc
	}
	layouts := [...]string{time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func buildContentItem(feedID uint, item parsedFeedItem, fetchedAt time.Time) (*model.ContentItem, bool) {
	dedupeKey, ok := dedupeKeyForItem(item)
	if !ok {
		return nil, false
	}
	descriptionSafe := contentutil.SanitizeHTML(item.Description)
	contentSafe := contentutil.SanitizeHTML(item.Content)
	showNotesSafe := contentutil.SanitizeHTML(item.ShowNotes)
	return &model.ContentItem{
		FeedID:          feedID,
		DedupeKey:       dedupeKey,
		Type:            contentItemType(item.Enclosures),
		Title:           item.Title,
		Description:     item.Description,
		Content:         item.Content,
		ShowNotes:       item.ShowNotes,
		DescriptionSafe: descriptionSafe,
		ContentSafe:     contentSafe,
		ShowNotesSafe:   showNotesSafe,
		AvailableText: contentutil.AvailableText(contentutil.TextFields{
			Content:     contentSafe,
			ShowNotes:   showNotesSafe,
			Description: descriptionSafe,
			Title:       item.Title,
		}),
		PublishedAt:          item.PublishedAt,
		FetchedAt:            fetchedAt,
		AudioProgressSeconds: 0,
	}, true
}

func feedTypeForRSSChannel(channel rssChannel, items []parsedFeedItem) model.FeedType {
	if channelHasPodcastSemantics(channel) || itemsHaveAudioEnclosure(items) {
		return model.FeedTypePodcast
	}
	return model.FeedTypeRSS
}

func channelHasPodcastSemantics(channel rssChannel) bool {
	if strings.TrimSpace(channel.ITunesAuthor) != "" || strings.TrimSpace(channel.ITunesExplicit) != "" || strings.TrimSpace(channel.ITunesImage.Href) != "" || strings.TrimSpace(channel.ITunesType) != "" {
		return true
	}
	if strings.TrimSpace(channel.ITunesOwner.Name) != "" || strings.TrimSpace(channel.ITunesOwner.Email) != "" {
		return true
	}
	for _, category := range channel.ITunesCategory {
		if strings.TrimSpace(category.Text) != "" {
			return true
		}
	}
	return false
}

func itemsHaveAudioEnclosure(items []parsedFeedItem) bool {
	for _, item := range items {
		for _, enclosure := range item.Enclosures {
			if isAudioEnclosure(enclosure) {
				return true
			}
		}
	}
	return false
}

func contentItemType(enclosures []parsedEnclosure) model.ContentItemType {
	for _, enclosure := range enclosures {
		if isAudioEnclosure(enclosure) {
			return model.ContentItemTypeAudio
		}
	}
	return model.ContentItemTypeText
}

func isAudioEnclosure(enclosure parsedEnclosure) bool {
	return strings.HasPrefix(strings.ToLower(enclosure.Type), "audio/")
}

func dedupeKeyForItem(item parsedFeedItem) (string, bool) {
	if item.GUID != "" {
		return item.GUID, true
	}
	if item.Link != "" {
		return item.Link, true
	}
	if item.Title == "" || item.PublishedAt == nil {
		return "", false
	}
	hash := sha256.Sum256([]byte(item.Title + "\n" + item.PublishedAt.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(hash[:]), true
}
