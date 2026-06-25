package service_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
	"shiliu/pkg/log"
)

func TestFeedFetchServiceFetchFeedStoresSanitizedPodcastItemThroughInjectedFetcher(t *testing.T) {
	ctx := context.Background()
	fetcher := newFixtureFetcher(t, map[string]string{
		"https://example.com/podcast.xml": "podcast_malicious.xml",
	})
	svc, feedRepo, contentRepo := newFeedFetchHarness(t, fetcher)
	feed := &model.Feed{
		FeedURL: " HTTPS://EXAMPLE.com:443/podcast.xml#fragment ",
		Type:    model.FeedTypePodcast,
	}
	require.NoError(t, feedRepo.Create(ctx, feed))

	result, err := svc.FetchFeed(ctx, feed)

	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/podcast.xml"}, fetcher.requests)
	assert.Equal(t, uint(feed.Id), result.FeedID)
	assert.Equal(t, "https://example.com/podcast.xml", result.FeedURL)
	assert.Equal(t, 1, result.FetchedItems)
	assert.Equal(t, 1, result.InsertedItems)
	assert.Equal(t, 0, result.SkippedExistingItems)

	got, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "episode-guid")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.ContentItemTypeAudio, got.Type)
	assert.Equal(t, "Episode 1", got.Title)
	assert.Contains(t, got.Description, "onclick")
	assert.NotContains(t, got.DescriptionSafe, "onclick")
	assert.NotContains(t, got.DescriptionSafe, "<script")
	assert.Contains(t, got.Content, "javascript:alert")
	assert.NotContains(t, got.ContentSafe, "javascript:")
	assert.NotContains(t, got.ContentSafe, "iframe")
	assert.NotContains(t, got.ContentSafe, "<script")
	assert.Contains(t, got.ContentSafe, "Safe content")
	assert.NotContains(t, got.ShowNotesSafe, "<script")
	assert.Equal(t, "Safe content link bad link", got.AvailableText)
	require.NotNil(t, got.PublishedAt)
	assert.Equal(t, time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC), got.PublishedAt.UTC())
}

func TestFeedServiceCreateFeedFetchesParsesAndPersistsFeedAndItems(t *testing.T) {
	ctx := context.Background()
	fetcher := newFixtureFetcher(t, map[string]string{
		"https://example.com/articles.xml": "rss_initial.xml",
	})
	svc, feedRepo, contentRepo := newFeedServiceHarness(t, fetcher)

	result, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: " HTTPS://EXAMPLE.com:443/articles.xml#fragment "})

	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/articles.xml"}, fetcher.requests)
	require.NotNil(t, result)
	assert.NotZero(t, result.Id)
	assert.Equal(t, "https://example.com/articles.xml", result.FeedURL)
	assert.Equal(t, string(model.FeedTypeRSS), result.Type)
	assert.Equal(t, 1, result.FetchedItems)
	assert.Equal(t, 1, result.InsertedItems)

	feed, err := feedRepo.GetByURL(ctx, "https://example.com/articles.xml")
	require.NoError(t, err)
	require.NotNil(t, feed)
	assert.Equal(t, result.Id, feed.Id)
	assert.Equal(t, model.FeedTypeRSS, feed.Type)
	assert.Equal(t, model.FeedFetchStatusSuccess, feed.FetchStatus)
	require.NotNil(t, feed.LastFetchedAt)
	assert.Nil(t, feed.FetchStartedAt)
	assert.Nil(t, feed.LastFetchError)

	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Stable title", items[0].Title)
	assert.Equal(t, model.ContentItemTypeText, items[0].Type)
	assert.Equal(t, "Original content", items[0].AvailableText)
}

func TestFeedServiceCreateFeedParseFailureDoesNotCreateFeed(t *testing.T) {
	ctx := context.Background()
	fetcher := newFixtureFetcher(t, map[string]string{
		"https://example.com/not-feed.xml": "not_feed.xml",
	})
	svc, feedRepo, _ := newFeedServiceHarness(t, fetcher)

	result, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: "https://example.com/not-feed.xml"})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, v1.ErrFeedParseFailed)
	assert.Equal(t, []string{"https://example.com/not-feed.xml"}, fetcher.requests)
	feeds, listErr := feedRepo.List(ctx)
	require.NoError(t, listErr)
	assert.Empty(t, feeds)
}

func TestFeedServiceCreateFeedFetchFailureDoesNotCreateFeed(t *testing.T) {
	ctx := context.Background()
	fetcher := &errorFetcher{err: fmt.Errorf("network unavailable")}
	svc, feedRepo, _ := newFeedServiceHarness(t, fetcher)

	result, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: "https://example.com/down.xml"})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, v1.ErrFeedFetchFailed)
	assert.Equal(t, []string{"https://example.com/down.xml"}, fetcher.requests)
	feeds, listErr := feedRepo.List(ctx)
	require.NoError(t, listErr)
	assert.Empty(t, feeds)
}

func TestFeedServiceCreateFeedPreservesFetcherInvalidURLError(t *testing.T) {
	ctx := context.Background()
	fetcher := &errorFetcher{err: v1.ErrFeedInvalidURL}
	svc, feedRepo, _ := newFeedServiceHarness(t, fetcher)

	result, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: "http://127.0.0.1/feed.xml"})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, v1.ErrFeedInvalidURL)
	assert.NotErrorIs(t, err, v1.ErrFeedFetchFailed)
	feeds, listErr := feedRepo.List(ctx)
	require.NoError(t, listErr)
	assert.Empty(t, feeds)
}

func TestFeedServiceCreateFeedDuplicateNormalizedURLReturnsConflictWithoutFetching(t *testing.T) {
	ctx := context.Background()
	fetcher := newFixtureFetcher(t, map[string]string{
		"https://example.com/articles.xml": "rss_initial.xml",
	})
	svc, feedRepo, _ := newFeedServiceHarness(t, fetcher)

	created, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: "https://example.com/articles.xml"})
	require.NoError(t, err)
	require.NotNil(t, created)
	fetcher.requests = nil

	duplicate, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: " HTTPS://EXAMPLE.com:443/articles.xml#fragment "})

	assert.Nil(t, duplicate)
	assert.ErrorIs(t, err, v1.ErrFeedAlreadyExists)
	assert.Empty(t, fetcher.requests)
	feeds, listErr := feedRepo.List(ctx)
	require.NoError(t, listErr)
	require.Len(t, feeds, 1)
	assert.Equal(t, created.Id, feeds[0].Id)
}

func TestFeedServiceCreateFeedDetectsPodcastRSSSemanticsWithoutAudioEnclosure(t *testing.T) {
	ctx := context.Background()
	fetcher := newFixtureFetcher(t, map[string]string{
		"https://example.com/semantic-podcast.xml": "podcast_semantic_no_audio.xml",
	})
	svc, feedRepo, contentRepo := newFeedServiceHarness(t, fetcher)

	created, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: "https://example.com/semantic-podcast.xml"})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, string(model.FeedTypePodcast), created.Type)
	feed, err := feedRepo.GetByURL(ctx, "https://example.com/semantic-podcast.xml")
	require.NoError(t, err)
	require.NotNil(t, feed)
	assert.Equal(t, model.FeedTypePodcast, feed.Type)
	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, model.ContentItemTypeText, items[0].Type)
}

func TestNormalizeFeedURLCanonicalizesSafeURLParts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "http default port", raw: " HTTP://Example.COM:80/feed.xml#fragment ", want: "http://example.com/feed.xml"},
		{name: "https default port", raw: "https://Example.COM:443/feed.xml", want: "https://example.com/feed.xml"},
		{name: "non default port", raw: "https://Example.COM:8443/feed.xml#section", want: "https://example.com:8443/feed.xml"},
		{name: "trailing dns dot", raw: "https://Example.COM./feed.xml", want: "https://example.com/feed.xml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.NormalizeFeedURL(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := service.NormalizeFeedURL("example.com/feed.xml")
	assert.ErrorIs(t, err, v1.ErrFeedInvalidURL)
	_, err = service.NormalizeFeedURL("https://alice:secret@example.com/feed.xml")
	assert.ErrorIs(t, err, v1.ErrFeedInvalidURL)
}

func TestHTTPFetcherRejectsUnsafeTargetsRedirectsAndOversizedBodies(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects private target before transport", func(t *testing.T) {
		called := false
		fetcher := service.NewHTTPFetcher(&http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		})})

		_, err := fetcher.Fetch(ctx, "http://127.0.0.1/feed.xml")

		assert.ErrorIs(t, err, v1.ErrFeedInvalidURL)
		assert.False(t, called)
	})

	t.Run("rejects redirect to private target", func(t *testing.T) {
		calls := 0
		fetcher := service.NewHTTPFetcher(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://169.254.169.254/latest/meta-data"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		})})

		_, err := fetcher.Fetch(ctx, "https://example.com/feed.xml")

		assert.ErrorIs(t, err, v1.ErrFeedInvalidURL)
		assert.Equal(t, 1, calls)
	})

	t.Run("rejects oversized body", func(t *testing.T) {
		fetcher := service.NewHTTPFetcher(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    http.StatusOK,
				ContentLength: 1 << 30,
				Body:          io.NopCloser(strings.NewReader("ok")),
				Request:       req,
			}, nil
		})})

		_, err := fetcher.Fetch(ctx, "https://example.com/feed.xml")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})
}

func TestFeedFetchServiceRejectsWellFormedXMLThatIsNotRSS(t *testing.T) {
	ctx := context.Background()
	feedURL := "https://example.com/not-feed.xml"
	fetcher := newFixtureFetcher(t, map[string]string{feedURL: "not_feed.xml"})
	svc, feedRepo, contentRepo := newFeedFetchHarness(t, fetcher)
	feed := &model.Feed{FeedURL: feedURL, Type: model.FeedTypeRSS}
	require.NoError(t, feedRepo.Create(ctx, feed))

	result, err := svc.FetchFeed(ctx, feed)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, v1.ErrFeedParseFailed)
	items, listErr := contentRepo.ListByFeedID(ctx, feed.Id, 10)
	require.NoError(t, listErr)
	assert.Empty(t, items)
}

func TestFeedFetchServiceFirstFetchPersistsOnlyNewest50Items(t *testing.T) {
	ctx := context.Background()
	fetcher := newFixtureFetcher(t, map[string]string{
		"https://example.com/articles.xml": "rss_many_items.xml",
	})
	svc, feedRepo, contentRepo := newFeedFetchHarness(t, fetcher)
	feed := &model.Feed{FeedURL: "https://example.com/articles.xml", Type: model.FeedTypeRSS}
	require.NoError(t, feedRepo.Create(ctx, feed))

	result, err := svc.FetchFeed(ctx, feed)

	require.NoError(t, err)
	assert.Equal(t, 55, result.FetchedItems)
	assert.Equal(t, 50, result.InsertedItems)
	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 100)
	require.NoError(t, err)
	require.Len(t, items, 50)
	assert.Equal(t, "Article 55", items[0].Title)
	assert.Equal(t, "Article 06", items[49].Title)
	old, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "many-05")
	require.NoError(t, err)
	assert.Nil(t, old)
	second, err := svc.FetchFeed(ctx, feed)
	require.NoError(t, err)
	assert.Equal(t, 0, second.InsertedItems)
	assert.Equal(t, 50, second.SkippedExistingItems)
	items, err = contentRepo.ListByFeedID(ctx, feed.Id, 100)
	require.NoError(t, err)
	require.Len(t, items, 50)
	old, err = contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "many-05")
	require.NoError(t, err)
	assert.Nil(t, old)
}

func TestFeedFetchServiceFirstFetchCapDoesNotBackfillOldItemsWhenSelectedItemsAreSkipped(t *testing.T) {
	ctx := context.Background()
	feedURL := "https://example.com/skipped-cap.xml"
	fetcher := newFixtureFetcher(t, map[string]string{feedURL: "rss_many_items_missing_dedupe.xml"})
	svc, feedRepo, contentRepo := newFeedFetchHarness(t, fetcher)
	feed := &model.Feed{FeedURL: feedURL, Type: model.FeedTypeRSS}
	require.NoError(t, feedRepo.Create(ctx, feed))

	first, err := svc.FetchFeed(ctx, feed)
	require.NoError(t, err)
	assert.Equal(t, 55, first.FetchedItems)
	assert.Equal(t, 35, first.InsertedItems)
	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 100)
	require.NoError(t, err)
	require.Len(t, items, 35)
	old, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "skip-05")
	require.NoError(t, err)
	assert.Nil(t, old)

	second, err := svc.FetchFeed(ctx, feed)
	require.NoError(t, err)
	assert.Equal(t, 0, second.InsertedItems)
	assert.Equal(t, 35, second.SkippedExistingItems)
	items, err = contentRepo.ListByFeedID(ctx, feed.Id, 100)
	require.NoError(t, err)
	require.Len(t, items, 35)
	old, err = contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "skip-05")
	require.NoError(t, err)
	assert.Nil(t, old)
}

func TestFeedFetchServiceSecondFetchInsertsOnlyNewItemsAndDoesNotUpdateExisting(t *testing.T) {
	ctx := context.Background()
	feedURL := "https://example.com/incremental.xml"
	fetcher := newFixtureFetcher(t, map[string]string{feedURL: "rss_initial.xml"})
	svc, feedRepo, contentRepo := newFeedFetchHarness(t, fetcher)
	feed := &model.Feed{FeedURL: feedURL, Type: model.FeedTypeRSS}
	require.NoError(t, feedRepo.Create(ctx, feed))

	first, err := svc.FetchFeed(ctx, feed)
	require.NoError(t, err)
	assert.Equal(t, 1, first.InsertedItems)
	original, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "stable-guid")
	require.NoError(t, err)
	require.NotNil(t, original)

	fetcher.responses[feedURL] = readFixture(t, "rss_incremental.xml")
	second, err := svc.FetchFeed(ctx, feed)

	require.NoError(t, err)
	assert.Equal(t, 2, second.FetchedItems)
	assert.Equal(t, 1, second.InsertedItems)
	assert.Equal(t, 1, second.SkippedExistingItems)
	unchanged, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "stable-guid")
	require.NoError(t, err)
	require.NotNil(t, unchanged)
	assert.Equal(t, original.Id, unchanged.Id)
	assert.Equal(t, "Stable title", unchanged.Title)
	assert.Equal(t, "Original content", unchanged.AvailableText)
	inserted, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "new-guid")
	require.NoError(t, err)
	require.NotNil(t, inserted)
	assert.Equal(t, "New title", inserted.Title)
}

func TestFeedFetchServiceDedupeKeyPriorityUsesGUIDThenLinkThenTitlePublishedHash(t *testing.T) {
	ctx := context.Background()
	feedURL := "https://example.com/dedupe.xml"
	fetcher := newFixtureFetcher(t, map[string]string{feedURL: "dedupe_priority_initial.xml"})
	svc, feedRepo, contentRepo := newFeedFetchHarness(t, fetcher)
	feed := &model.Feed{FeedURL: feedURL, Type: model.FeedTypeRSS}
	require.NoError(t, feedRepo.Create(ctx, feed))

	first, err := svc.FetchFeed(ctx, feed)
	require.NoError(t, err)
	assert.Equal(t, 3, first.InsertedItems)
	guidItem, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "guid-wins")
	require.NoError(t, err)
	require.NotNil(t, guidItem)
	assert.Equal(t, "Guid wins", guidItem.Title)
	linkItem, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "https://example.com/link-wins")
	require.NoError(t, err)
	require.NotNil(t, linkItem)
	assert.Equal(t, "Link wins", linkItem.Title)
	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 10)
	require.NoError(t, err)
	hashItem := itemByTitle(items, "Hash wins")
	require.NotNil(t, hashItem)
	assert.NotEmpty(t, hashItem.DedupeKey)
	assert.NotEqual(t, "Hash wins", hashItem.DedupeKey)

	fetcher.responses[feedURL] = readFixture(t, "dedupe_priority_changed.xml")
	second, err := svc.FetchFeed(ctx, feed)

	require.NoError(t, err)
	assert.Equal(t, 0, second.InsertedItems)
	assert.Equal(t, 3, second.SkippedExistingItems)
	guidItem, err = contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "guid-wins")
	require.NoError(t, err)
	require.NotNil(t, guidItem)
	assert.Equal(t, "Guid wins", guidItem.Title)
	linkItem, err = contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "https://example.com/link-wins")
	require.NoError(t, err)
	require.NotNil(t, linkItem)
	assert.Equal(t, "Link wins", linkItem.Title)
	unchangedHashItem, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, hashItem.DedupeKey)
	require.NoError(t, err)
	require.NotNil(t, unchangedHashItem)
	assert.Equal(t, "Hash original", unchangedHashItem.AvailableText)
}

func itemByTitle(items []*model.ContentItem, title string) *model.ContentItem {
	for _, item := range items {
		if item.Title == title {
			return item
		}
	}
	return nil
}

type fixtureFetcher struct {
	t         *testing.T
	responses map[string][]byte
	requests  []string
}

func newFixtureFetcher(t *testing.T, fixtures map[string]string) *fixtureFetcher {
	t.Helper()
	responses := make(map[string][]byte, len(fixtures))
	for feedURL, name := range fixtures {
		responses[feedURL] = readFixture(t, name)
	}
	return &fixtureFetcher{t: t, responses: responses}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return content
}

func (f *fixtureFetcher) Fetch(_ context.Context, feedURL string) ([]byte, error) {
	f.requests = append(f.requests, feedURL)
	content, ok := f.responses[feedURL]
	if !ok {
		return nil, fmt.Errorf("unexpected fetch url %q", feedURL)
	}
	return content, nil
}

type errorFetcher struct {
	err      error
	requests []string
}

func (f *errorFetcher) Fetch(_ context.Context, feedURL string) ([]byte, error) {
	f.requests = append(f.requests, feedURL)
	return nil, f.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newFeedFetchHarness(t *testing.T, fetcher service.Fetcher) (service.FeedFetchService, repository.FeedRepository, repository.ContentItemRepository) {
	t.Helper()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	return service.NewFeedFetchService(base, contentRepo, fetcher), feedRepo, contentRepo
}

func newFeedServiceHarness(t *testing.T, fetcher service.Fetcher) (service.FeedService, repository.FeedRepository, repository.ContentItemRepository) {
	t.Helper()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	return service.NewFeedService(base, feedRepo, contentRepo, fetcher), feedRepo, contentRepo
}

func openServiceTestDB(t *testing.T, logger *log.Logger) *gorm.DB {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "shiliu-service-test.db") + "?_busy_timeout=5000"
	conf.Set("data.db.user.driver", "sqlite")
	conf.Set("data.db.user.dsn", dsn)
	conf.Set("data.db.user.debug", false)
	runServiceMigrations(t, dsn)
	db := repository.NewDB(conf, logger)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	return db
}

func runServiceMigrations(t *testing.T, dsn string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "migration-test.yml")
	content := fmt.Sprintf("data:\n  db:\n    user:\n      driver: sqlite\n      dsn: %q\n      debug: false\n", dsn)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	cmd := exec.Command("go", "run", "./cmd/migration", "-conf", configPath, "-direction", "up", "-path", "migrations")
	cmd.Dir = repoRoot(t)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func newObservedLogger(level zapcore.LevelEnabler) (*log.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return &log.Logger{Logger: zap.New(core)}, logs
}
