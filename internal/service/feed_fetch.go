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

	"gorm.io/gorm"
)

const (
	initialFetchItemLimit = 50
	maxFeedResponseBytes  = int64(10 << 20)
)

// Fetcher is the network boundary for retrieving feed XML. Tests inject this
// interface so feed parsing and persistence run without real network access.
type Fetcher interface {
	Fetch(ctx context.Context, feedURL string) ([]byte, error)
}

type HTTPFetcher struct {
	client *http.Client
}

func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	previousCheckRedirect := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateHTTPFeedURL(req.URL); err != nil {
			return err
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &HTTPFetcher{client: &clientCopy}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, feedURL string) ([]byte, error) {
	parsedURL, err := url.Parse(feedURL)
	if err != nil {
		return nil, v1.ErrFeedInvalidURL
	}
	if err := validateHTTPFeedURL(parsedURL); err != nil {
		return nil, err
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
	FetchedItems         int
	InsertedItems        int
	SkippedExistingItems int
}

func NewFeedFetchService(
	service *Service,
	contentRepo repository.ContentItemRepository,
	fetcher Fetcher,
) FeedFetchService {
	return &feedFetchService{
		Service:     service,
		contentRepo: contentRepo,
		fetcher:     fetcher,
	}
}

type feedFetchService struct {
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
	body, err := s.fetcher.Fetch(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", v1.ErrFeedFetchFailed, err)
	}
	items, err := parseRSSFeed(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", v1.ErrFeedParseFailed, err)
	}
	result := &FetchFeedResult{FeedID: feed.Id, FeedURL: feedURL, FetchedItems: len(items)}
	selectedItems, err := s.itemsToPersist(ctx, feed.Id, items)
	if err != nil {
		return nil, err
	}

	fetchedAt := time.Now().UTC()
	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		for _, item := range selectedItems {
			contentItem, ok := buildContentItem(feed.Id, feed.Type, item, fetchedAt)
			if !ok {
				continue
			}
			existing, err := s.contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, contentItem.DedupeKey)
			if err != nil {
				return err
			}
			if existing != nil {
				result.SkippedExistingItems++
				continue
			}
			if err := s.contentRepo.Create(ctx, contentItem); err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					result.SkippedExistingItems++
					continue
				}
				return err
			}
			result.InsertedItems++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *feedFetchService) itemsToPersist(ctx context.Context, feedID uint, items []parsedFeedItem) ([]parsedFeedItem, error) {
	existing, err := s.contentRepo.ListByFeedID(ctx, feedID, initialFetchItemLimit)
	if err != nil {
		return nil, err
	}
	if len(items) <= initialFetchItemLimit {
		return items, nil
	}
	if len(existing) == 0 || len(existing) >= initialFetchItemLimit {
		return newestFeedItems(items), nil
	}
	return items, nil
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
	host := strings.ToLower(parsed.Hostname())
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
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
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

func parseRSSFeed(body []byte) ([]parsedFeedItem, error) {
	var doc rssDocument
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
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
	return items, nil
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

func buildContentItem(feedID uint, feedType model.FeedType, item parsedFeedItem, fetchedAt time.Time) (*model.ContentItem, bool) {
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
		Type:            contentItemType(feedType, item.Enclosures),
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

func contentItemType(feedType model.FeedType, enclosures []parsedEnclosure) model.ContentItemType {
	if feedType == model.FeedTypePodcast {
		return model.ContentItemTypeAudio
	}
	for _, enclosure := range enclosures {
		if strings.HasPrefix(strings.ToLower(enclosure.Type), "audio/") {
			return model.ContentItemTypeAudio
		}
	}
	return model.ContentItemTypeText
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
