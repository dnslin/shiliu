package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"

	"shiliu/internal/model"
	"shiliu/internal/repository"
)

func setupFeedRepository(t *testing.T) repository.FeedRepository {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewFeedRepository(repo)
}

func setupFeedAndContentRepositories(t *testing.T) (repository.FeedRepository, repository.ContentItemRepository) {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewFeedRepository(repo), repository.NewContentItemRepository(repo)
}

func TestFeedRepository_CreateAndGetByURL(t *testing.T) {
	feedRepo := setupFeedRepository(t)
	ctx := context.Background()
	feed := &model.Feed{
		FeedURL:     "https://example.com/feed.xml",
		Type:        model.FeedTypeRSS,
		FetchStatus: model.FeedFetchStatusIdle,
	}

	require.NoError(t, feedRepo.Create(ctx, feed))
	require.NotZero(t, feed.Id)

	got, err := feedRepo.GetByURL(ctx, feed.FeedURL)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, feed.Id, got.Id)
	assert.Equal(t, feed.FeedURL, got.FeedURL)
	assert.Equal(t, model.FeedTypeRSS, got.Type)
	assert.Equal(t, model.FeedFetchStatusIdle, got.FetchStatus)
	assert.Nil(t, got.FolderID)

	missing, err := feedRepo.GetByURL(ctx, "https://example.com/missing.xml")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestFeedRepository_CreateDuplicateURLFails(t *testing.T) {
	feedRepo := setupFeedRepository(t)
	ctx := context.Background()

	require.NoError(t, feedRepo.Create(ctx, &model.Feed{FeedURL: "https://example.com/feed.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}))
	err := feedRepo.Create(ctx, &model.Feed{FeedURL: "https://example.com/feed.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle})

	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrDuplicatedKey)
}

func TestFeedRepository_UpdateFetchStatePersistsDiagnostics(t *testing.T) {
	feedRepo := setupFeedRepository(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/podcast.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))

	startedAt := time.Now().UTC().Truncate(time.Second)
	fetchedAt := startedAt.Add(2 * time.Second)
	fetchError := "upstream returned 500"
	require.NoError(t, feedRepo.UpdateFetchState(ctx, feed.Id, model.FeedFetchStatusFailed, &startedAt, &fetchedAt, &fetchError))

	got, err := feedRepo.GetByURL(ctx, feed.FeedURL)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.FeedFetchStatusFailed, got.FetchStatus)
	require.NotNil(t, got.FetchStartedAt)
	assert.WithinDuration(t, startedAt, got.FetchStartedAt.UTC(), time.Second)
	require.NotNil(t, got.LastFetchedAt)
	assert.WithinDuration(t, fetchedAt, got.LastFetchedAt.UTC(), time.Second)
	require.NotNil(t, got.LastFetchError)
	assert.Equal(t, fetchError, *got.LastFetchError)
}

func TestFeedRepository_UpdateFetchStateIfOwnedDoesNotOverwriteNewClaim(t *testing.T) {
	feedRepo := setupFeedRepository(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/owned.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))

	oldClaim := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	newClaim := oldClaim.Add(45 * time.Minute)
	require.NoError(t, feedRepo.UpdateFetchState(ctx, feed.Id, model.FeedFetchStatusFetching, &newClaim, nil, nil))
	finishedAt := newClaim.Add(time.Minute)

	owned, err := feedRepo.UpdateFetchStateIfOwned(ctx, feed.Id, oldClaim, model.FeedFetchStatusSuccess, nil, &finishedAt, nil)

	require.NoError(t, err)
	assert.False(t, owned)
	got, loadErr := feedRepo.GetByURL(ctx, feed.FeedURL)
	require.NoError(t, loadErr)
	require.NotNil(t, got)
	assert.Equal(t, model.FeedFetchStatusFetching, got.FetchStatus)
	require.NotNil(t, got.FetchStartedAt)
	assert.WithinDuration(t, newClaim, got.FetchStartedAt.UTC(), time.Second)
	assert.Nil(t, got.LastFetchedAt)
}

func TestFeedRepository_ReleaseFetchClaimIfOwnedDoesNotOverwriteNewClaim(t *testing.T) {
	feedRepo := setupFeedRepository(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/release-owned.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))

	oldClaim := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	newClaim := oldClaim.Add(45 * time.Minute)
	require.NoError(t, feedRepo.UpdateFetchState(ctx, feed.Id, model.FeedFetchStatusFetching, &newClaim, nil, nil))

	owned, err := feedRepo.ReleaseFetchClaimIfOwned(ctx, feed.Id, oldClaim)

	require.NoError(t, err)
	assert.False(t, owned)
	got, loadErr := feedRepo.GetByURL(ctx, feed.FeedURL)
	require.NoError(t, loadErr)
	require.NotNil(t, got)
	assert.Equal(t, model.FeedFetchStatusFetching, got.FetchStatus)
	require.NotNil(t, got.FetchStartedAt)
	assert.WithinDuration(t, newClaim, got.FetchStartedAt.UTC(), time.Second)
}

func TestContentItemRepository_CreateAndGetByFeedDedupeKey(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/articles.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	publishedAt := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	fetchedAt := time.Now().UTC().Truncate(time.Second)
	item := &model.ContentItem{
		FeedID:               feed.Id,
		DedupeKey:            "article-1",
		Type:                 model.ContentItemTypeText,
		Title:                "Article 1",
		Description:          "<p>Raw description</p>",
		Content:              "<article>Raw content</article>",
		ShowNotes:            "Raw notes",
		DescriptionSafe:      "Raw description",
		ContentSafe:          "Raw content",
		ShowNotesSafe:        "Raw notes",
		AvailableText:        "Article 1 Raw description Raw content",
		PublishedAt:          &publishedAt,
		FetchedAt:            fetchedAt,
		AudioProgressSeconds: 0,
	}

	require.NoError(t, contentRepo.Create(ctx, item))
	require.NotZero(t, item.Id)

	got, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, item.DedupeKey)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, item.Id, got.Id)
	assert.Equal(t, feed.Id, got.FeedID)
	assert.Equal(t, model.ContentItemTypeText, got.Type)
	assert.Equal(t, "Article 1", got.Title)
	assert.Equal(t, "Raw description", got.DescriptionSafe)
	assert.Equal(t, "Raw content", got.ContentSafe)
	assert.Equal(t, "Raw notes", got.ShowNotesSafe)
	assert.Equal(t, "Article 1 Raw description Raw content", got.AvailableText)
	require.NotNil(t, got.PublishedAt)
	assert.WithinDuration(t, publishedAt, got.PublishedAt.UTC(), time.Second)
	assert.WithinDuration(t, fetchedAt, got.FetchedAt.UTC(), time.Second)
	assert.Zero(t, got.AudioProgressSeconds)

	missing, err := contentRepo.GetByFeedAndDedupeKey(ctx, feed.Id, "missing")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestContentItemRepository_EnforcesFeedScopedDedupeAndForeignKey(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	firstFeed := &model.Feed{FeedURL: "https://example.com/first.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	secondFeed := &model.Feed{FeedURL: "https://example.com/second.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, firstFeed))
	require.NoError(t, feedRepo.Create(ctx, secondFeed))

	createItem := func(feedID uint, title string) *model.ContentItem {
		return &model.ContentItem{FeedID: feedID, DedupeKey: "shared-guid", Type: model.ContentItemTypeAudio, Title: title, AvailableText: title}
	}

	require.NoError(t, contentRepo.Create(ctx, createItem(firstFeed.Id, "First feed episode")))
	duplicateErr := contentRepo.Create(ctx, createItem(firstFeed.Id, "Duplicate episode"))
	require.Error(t, duplicateErr)
	assert.ErrorIs(t, duplicateErr, gorm.ErrDuplicatedKey)
	require.NoError(t, contentRepo.Create(ctx, createItem(secondFeed.Id, "Second feed episode")))

	orphanErr := contentRepo.Create(ctx, createItem(secondFeed.Id+1, "Orphan episode"))
	require.Error(t, orphanErr)
}

func TestContentItemRepository_UpdateAudioProgressPersists(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/audio.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "episode-progress", Type: model.ContentItemTypeAudio, Title: "Episode", AvailableText: "Episode"}
	require.NoError(t, contentRepo.Create(ctx, item))

	require.NoError(t, contentRepo.UpdateAudioProgress(ctx, item.Id, 372))

	got, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 372, got.AudioProgressSeconds)
}

func TestContentItemRepository_ListByFeedIDFiltersAndOrdersItems(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	firstFeed := &model.Feed{FeedURL: "https://example.com/list.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	secondFeed := &model.Feed{FeedURL: "https://example.com/other-list.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, firstFeed))
	require.NoError(t, feedRepo.Create(ctx, secondFeed))
	older := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)
	newer := older.Add(time.Hour)
	require.NoError(t, contentRepo.Create(ctx, &model.ContentItem{FeedID: firstFeed.Id, DedupeKey: "older", Type: model.ContentItemTypeText, Title: "Older", AvailableText: "Older", PublishedAt: &older}))
	require.NoError(t, contentRepo.Create(ctx, &model.ContentItem{FeedID: firstFeed.Id, DedupeKey: "newer", Type: model.ContentItemTypeText, Title: "Newer", AvailableText: "Newer", PublishedAt: &newer}))
	require.NoError(t, contentRepo.Create(ctx, &model.ContentItem{FeedID: secondFeed.Id, DedupeKey: "other", Type: model.ContentItemTypeText, Title: "Other", AvailableText: "Other", PublishedAt: &newer}))

	items, err := contentRepo.ListByFeedID(ctx, firstFeed.Id, 10)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "Newer", items[0].Title)
	assert.Equal(t, "Older", items[1].Title)
}
