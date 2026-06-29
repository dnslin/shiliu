package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"

	v1 "shiliu/api/v1"
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

	db, feedRepo, contentRepo := setupFeedAndContentRepositoriesWithDB(t)
	_ = db
	return feedRepo, contentRepo
}

func setupFeedAndContentRepositoriesWithDB(t *testing.T) (*gorm.DB, repository.FeedRepository, repository.ContentItemRepository) {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return db, repository.NewFeedRepository(repo), repository.NewContentItemRepository(repo)
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
func TestContentItemRepository_CreateWritesSearchIndex(t *testing.T) {
	db, feedRepo, contentRepo := setupFeedAndContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/search.xml", Title: "工程日报", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "search-item", Type: model.ContentItemTypeText, Title: "SQLite FTS5 入库", AvailableText: "开发者中文字段可以检索"}

	require.NoError(t, contentRepo.Create(ctx, item))

	requireContentSearchMatch(t, db, "SQLite", item.Id)
	requireContentSearchMatch(t, db, "工程日报", item.Id)
	requireContentSearchMatch(t, db, "中文字段", item.Id)
}

func requireContentSearchMatch(t *testing.T, db *gorm.DB, query string, wantID uint) {
	t.Helper()
	var rowID uint
	require.NoError(t, db.Raw(`SELECT rowid FROM content_item_search_index WHERE content_item_search_index MATCH ?`, query).Scan(&rowID).Error)
	assert.Equal(t, wantID, rowID, query)
}
func requireContentSearchMiss(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	var rowID uint
	require.NoError(t, db.Raw(`SELECT rowid FROM content_item_search_index WHERE content_item_search_index MATCH ?`, query).Scan(&rowID).Error)
	assert.Zero(t, rowID, query)
}

func TestContentItemRepository_UpdateSearchTextAndSummaryRefreshSearchIndex(t *testing.T) {
	db, feedRepo, contentRepo := setupFeedAndContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/update-search.xml", Title: "索引源", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "update-search-item", Type: model.ContentItemTypeText, Title: "旧标题", AvailableText: "旧文本"}
	require.NoError(t, contentRepo.Create(ctx, item))
	requireContentSearchMatch(t, db, "旧标题", item.Id)

	require.NoError(t, contentRepo.UpdateSearchText(ctx, item.Id, "FTS5 重建", "新的 中文字段"))

	requireContentSearchMiss(t, db, "旧标题")
	requireContentSearchMatch(t, db, "FTS5", item.Id)
	requireContentSearchMatch(t, db, "中文字段", item.Id)

	require.NoError(t, contentRepo.UpdateAISummarySearchText(ctx, item.Id, "自托管 摘要"))
	requireContentSearchMatch(t, db, "自托管", item.Id)
}

func TestFeedRepository_UpdateTitleRefreshesContentSearchIndex(t *testing.T) {
	db, feedRepo, contentRepo := setupFeedAndContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/update-feed-title.xml", Title: "旧订阅源", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "feed-title-item", Type: model.ContentItemTypeText, Title: "条目", AvailableText: "正文"}
	require.NoError(t, contentRepo.Create(ctx, item))
	requireContentSearchMatch(t, db, "旧订阅源", item.Id)

	require.NoError(t, feedRepo.UpdateTitle(ctx, feed.Id, "新订阅源"))

	requireContentSearchMiss(t, db, "旧订阅源")
	requireContentSearchMatch(t, db, "新订阅源", item.Id)
}

func TestFeedRepository_UpdateTitleSkipsUnchangedTitle(t *testing.T) {
	db, feedRepo, contentRepo := setupFeedAndContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/unchanged-feed-title.xml", Title: "稳定订阅源", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "unchanged-feed-title-item", Type: model.ContentItemTypeText, Title: "条目", AvailableText: "正文"}
	require.NoError(t, contentRepo.Create(ctx, item))
	oldUpdatedAt := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	require.NoError(t, db.Exec(`UPDATE feeds SET updated_at = ? WHERE id = ?`, oldUpdatedAt, feed.Id).Error)

	require.NoError(t, feedRepo.UpdateTitle(ctx, feed.Id, "稳定订阅源"))

	var updatedAt time.Time
	require.NoError(t, db.Raw(`SELECT updated_at FROM feeds WHERE id = ?`, feed.Id).Scan(&updatedAt).Error)
	assert.WithinDuration(t, oldUpdatedAt, updatedAt.UTC(), time.Second)
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

func TestContentItemRepository_UpdateProcessingStatusAndMarksPersistIndependently(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/state-marks.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "state-mark-item", Type: model.ContentItemTypeText, Title: "State mark", AvailableText: "State mark", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, AudioProgressSeconds: 44}
	require.NoError(t, contentRepo.Create(ctx, item))

	require.NoError(t, contentRepo.UpdateProcessingStatus(ctx, item.Id, model.ContentItemProcessingStatusCompleted))
	require.NoError(t, contentRepo.UpdateMark(ctx, item.Id, model.ContentItemMarkFavorite, true))
	require.NoError(t, contentRepo.UpdateMark(ctx, item.Id, model.ContentItemMarkLater, false))

	got, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.ContentItemProcessingStatusCompleted, got.ProcessingStatus)
	assert.False(t, got.MarkedLater)
	assert.True(t, got.Favorited)
	assert.Equal(t, 44, got.AudioProgressSeconds)
}

func TestContentItemRepository_UpdateStateAndMarksRejectInvalidTargets(t *testing.T) {
	_, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()

	assert.ErrorIs(t, contentRepo.UpdateProcessingStatus(ctx, 0, model.ContentItemProcessingStatusCompleted), v1.ErrBadRequest)
	assert.ErrorIs(t, contentRepo.UpdateProcessingStatus(ctx, 999, model.ContentItemProcessingStatusCompleted), v1.ErrNotFound)
	assert.ErrorIs(t, contentRepo.UpdateProcessingStatus(ctx, 999, model.ContentItemProcessingStatus("archived")), v1.ErrBadRequest)
	assert.ErrorIs(t, contentRepo.UpdateMark(ctx, 0, model.ContentItemMarkLater, true), v1.ErrBadRequest)
	assert.ErrorIs(t, contentRepo.UpdateMark(ctx, 999, model.ContentItemMarkLater, true), v1.ErrNotFound)
	assert.ErrorIs(t, contentRepo.UpdateMark(ctx, 999, model.ContentItemMark("read"), true), v1.ErrBadRequest)
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

func TestFeedRepository_DeleteCascadesContentItems(t *testing.T) {
	db, feedRepo, contentRepo := setupFeedAndContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/delete.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	relatedFeed := &model.Feed{FeedURL: "https://example.com/keep.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	require.NoError(t, feedRepo.Create(ctx, relatedFeed))
	deletedItem := &model.ContentItem{FeedID: feed.Id, DedupeKey: "deleted-episode", Type: model.ContentItemTypeAudio, Title: "Deleted", AvailableText: "Deleted", AudioProgressSeconds: 372}
	keptItem := &model.ContentItem{FeedID: relatedFeed.Id, DedupeKey: "kept-article", Type: model.ContentItemTypeText, Title: "Kept", AvailableText: "Kept"}
	require.NoError(t, contentRepo.Create(ctx, deletedItem))
	require.NoError(t, contentRepo.Create(ctx, keptItem))
	requireContentSearchMatch(t, db, "Deleted", deletedItem.Id)
	requireContentSearchMatch(t, db, "Kept", keptItem.Id)

	require.NoError(t, feedRepo.Delete(ctx, feed.Id))

	_, err := feedRepo.GetByID(ctx, feed.Id)
	assert.ErrorIs(t, err, v1.ErrNotFound)
	_, err = contentRepo.GetByID(ctx, deletedItem.Id)
	assert.ErrorIs(t, err, v1.ErrNotFound)
	requireContentSearchMiss(t, db, "Deleted")
	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 0)
	require.NoError(t, err)
	assert.Empty(t, items)
	kept, err := contentRepo.GetByID(ctx, keptItem.Id)
	require.NoError(t, err)
	assert.Equal(t, relatedFeed.Id, kept.FeedID)
	requireContentSearchMatch(t, db, "Kept", keptItem.Id)
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

func TestContentItemRepository_ListFiltersAndOrdersByPublishedAtFallback(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	primaryFeed := &model.Feed{FeedURL: "https://example.com/content-list.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	otherFeed := &model.Feed{FeedURL: "https://example.com/content-list-other.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, primaryFeed))
	require.NoError(t, feedRepo.Create(ctx, otherFeed))

	base := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	textType := model.ContentItemTypeText
	unprocessed := model.ContentItemProcessingStatusUnprocessed
	later := model.ContentItemMarkLater
	feedID := primaryFeed.Id

	items := []*model.ContentItem{
		{FeedID: primaryFeed.Id, DedupeKey: "published-newer", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Published newer", AvailableText: "Published newer", PublishedAt: timePtr(base.Add(2 * time.Hour)), FetchedAt: base.Add(time.Hour)},
		{FeedID: primaryFeed.Id, DedupeKey: "fetched-fallback", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Fetched fallback", AvailableText: "Fetched fallback", FetchedAt: base.Add(3 * time.Hour)},
		{FeedID: primaryFeed.Id, DedupeKey: "completed", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusCompleted, MarkedLater: true, Title: "Completed", AvailableText: "Completed", PublishedAt: timePtr(base.Add(4 * time.Hour)), FetchedAt: base.Add(4 * time.Hour)},
		{FeedID: primaryFeed.Id, DedupeKey: "favorite", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, Favorited: true, Title: "Favorite", AvailableText: "Favorite", PublishedAt: timePtr(base.Add(5 * time.Hour)), FetchedAt: base.Add(5 * time.Hour)},
		{FeedID: primaryFeed.Id, DedupeKey: "audio", Type: model.ContentItemTypeAudio, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Audio", AvailableText: "Audio", PublishedAt: timePtr(base.Add(6 * time.Hour)), FetchedAt: base.Add(6 * time.Hour)},
		{FeedID: otherFeed.Id, DedupeKey: "other-feed", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Other feed", AvailableText: "Other feed", PublishedAt: timePtr(base.Add(7 * time.Hour)), FetchedAt: base.Add(7 * time.Hour)},
	}
	for _, item := range items {
		require.NoError(t, contentRepo.Create(ctx, item))
	}

	filter := repository.ContentItemListFilter{
		ContentType:      &textType,
		ProcessingStatus: &unprocessed,
		Mark:             &later,
		FeedID:           &feedID,
	}

	all, total, err := contentRepo.List(ctx, filter, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, all, 2)
	assert.Equal(t, "Fetched fallback", all[0].Title)
	assert.Equal(t, "Published newer", all[1].Title)

	page, total, err := contentRepo.List(ctx, filter, 1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, page, 1)
	assert.Equal(t, "Published newer", page[0].Title)
}

func TestContentItemRepository_ListKeepsValidPreEpochPublishedAtOrdering(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/pre-epoch.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))

	preEpoch := time.Date(1969, 12, 31, 23, 0, 0, 0, time.UTC)
	postEpoch := time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, contentRepo.Create(ctx, &model.ContentItem{FeedID: feed.Id, DedupeKey: "pre-epoch", Type: model.ContentItemTypeText, Title: "Pre epoch", AvailableText: "Pre epoch", PublishedAt: &preEpoch, FetchedAt: time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)}))
	require.NoError(t, contentRepo.Create(ctx, &model.ContentItem{FeedID: feed.Id, DedupeKey: "post-epoch", Type: model.ContentItemTypeText, Title: "Post epoch", AvailableText: "Post epoch", PublishedAt: &postEpoch, FetchedAt: time.Date(2025, 6, 25, 10, 0, 0, 0, time.UTC)}))

	items, total, err := contentRepo.List(ctx, repository.ContentItemListFilter{FeedID: &feed.Id}, 10, 0)

	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	assert.Equal(t, "Post epoch", items[0].Title)
	assert.Equal(t, "Pre epoch", items[1].Title)
}

func TestContentItemRepository_ListLoadsOnlyListFields(t *testing.T) {
	feedRepo, contentRepo := setupFeedAndContentRepositories(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/list-projection.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{
		FeedID:          feed.Id,
		DedupeKey:       "projection-item",
		Type:            model.ContentItemTypeText,
		Title:           "Projection item",
		DescriptionSafe: "Safe description should be detail-only",
		ContentSafe:     "Safe content should be detail-only",
		ShowNotesSafe:   "Safe notes should be detail-only",
		AvailableText:   "Projection item",
	}
	require.NoError(t, contentRepo.Create(ctx, item))

	items, total, err := contentRepo.List(ctx, repository.ContentItemListFilter{FeedID: &feed.Id}, 10, 0)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "Projection item", items[0].Title)
	assert.Empty(t, items[0].DescriptionSafe)
	assert.Empty(t, items[0].ContentSafe)
	assert.Empty(t, items[0].ShowNotesSafe)

	detail, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "Safe content should be detail-only", detail.ContentSafe)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
