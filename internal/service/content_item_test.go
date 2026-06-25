package service_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestContentItemService_ListContentItemsReturnsFilteredPage(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo)

	primaryFeed := &model.Feed{FeedURL: "https://example.com/service-content-list.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	otherFeed := &model.Feed{FeedURL: "https://example.com/service-content-list-other.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, primaryFeed))
	require.NoError(t, feedRepo.Create(ctx, otherFeed))

	base := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	for _, item := range []*model.ContentItem{
		{FeedID: primaryFeed.Id, DedupeKey: "published-newer", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Published newer", AvailableText: "Published newer", PublishedAt: ptrTime(base.Add(2 * time.Hour)), FetchedAt: base.Add(time.Hour)},
		{FeedID: primaryFeed.Id, DedupeKey: "fetched-fallback", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Fetched fallback", AvailableText: "Fetched fallback", FetchedAt: base.Add(3 * time.Hour)},
		{FeedID: primaryFeed.Id, DedupeKey: "favorite", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, Favorited: true, Title: "Favorite", AvailableText: "Favorite", PublishedAt: ptrTime(base.Add(4 * time.Hour)), FetchedAt: base.Add(4 * time.Hour)},
		{FeedID: otherFeed.Id, DedupeKey: "other-feed", Type: model.ContentItemTypeText, ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Title: "Other feed", AvailableText: "Other feed", PublishedAt: ptrTime(base.Add(5 * time.Hour)), FetchedAt: base.Add(5 * time.Hour)},
	} {
		require.NoError(t, contentRepo.Create(ctx, item))
	}

	result, err := svc.ListContentItems(ctx, &v1.ListContentItemsRequest{
		ContentType:      "text",
		ProcessingStatus: "unprocessed",
		Mark:             "later",
		FeedID:           strconv.FormatUint(uint64(primaryFeed.Id), 10),
		Page:             v1.PageRequest{Page: 2, PageSize: 1},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.Page.Page)
	assert.Equal(t, 1, result.Page.PageSize)
	require.Equal(t, int64(2), result.Page.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "Published newer", result.Items[0].Title)
	assert.Equal(t, "text", result.Items[0].ContentType)
	assert.Equal(t, "unprocessed", result.Items[0].ProcessingStatus)
	assert.True(t, result.Items[0].MarkedLater)
	assert.False(t, result.Items[0].Favorited)
}

func TestContentItemService_GetContentItemReturnsDetail(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo)

	feed := &model.Feed{FeedURL: "https://example.com/service-content-detail.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	publishedAt := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	fetchedAt := publishedAt.Add(time.Hour)
	item := &model.ContentItem{
		FeedID:               feed.Id,
		DedupeKey:            "detail-item",
		Type:                 model.ContentItemTypeText,
		Title:                "Detail item",
		DescriptionSafe:      "Safe description",
		ContentSafe:          "Safe content",
		ShowNotesSafe:        "Safe notes",
		AvailableText:        "Safe content",
		PublishedAt:          &publishedAt,
		FetchedAt:            fetchedAt,
		ProcessingStatus:     model.ContentItemProcessingStatusCompleted,
		MarkedLater:          true,
		Favorited:            true,
		AudioProgressSeconds: 15,
	}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GetContentItem(ctx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, item.Id, result.Id)
	assert.Equal(t, feed.Id, result.FeedID)
	assert.Equal(t, "text", result.ContentType)
	assert.Equal(t, "Detail item", result.Title)
	assert.Equal(t, "Safe description", result.DescriptionSafe)
	assert.Equal(t, "Safe content", result.ContentSafe)
	assert.Equal(t, "Safe notes", result.ShowNotesSafe)
	assert.Equal(t, "completed", result.ProcessingStatus)
	assert.True(t, result.MarkedLater)
	assert.True(t, result.Favorited)
	assert.Equal(t, 15, result.AudioProgressSeconds)
}
func ptrTime(t time.Time) *time.Time {
	return &t
}
