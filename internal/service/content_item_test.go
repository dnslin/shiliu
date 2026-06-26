package service_test

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
	mock_repository "shiliu/test/mocks/repository"
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

func TestContentItemService_UpdateProcessingStatusUsesRepositoryAndReturnsDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	contentRepo := mock_repository.NewMockContentItemRepository(ctrl)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), contentRepo)
	ctx := context.Background()
	item := &model.ContentItem{Id: 42, FeedID: 7, Type: model.ContentItemTypeAudio, Title: "Episode", ProcessingStatus: model.ContentItemProcessingStatusCompleted, MarkedLater: true, Favorited: true, AudioProgressSeconds: 91}

	contentRepo.EXPECT().UpdateProcessingStatus(ctx, uint(42), model.ContentItemProcessingStatusCompleted).Return(nil)
	contentRepo.EXPECT().GetByID(ctx, uint(42)).Return(item, nil)

	result, err := svc.UpdateProcessingStatus(ctx, 42, &v1.UpdateContentItemProcessingStatusRequest{ProcessingStatus: "completed"})

	require.NoError(t, err)
	assert.Equal(t, "completed", result.ProcessingStatus)
	assert.True(t, result.MarkedLater)
	assert.True(t, result.Favorited)
	assert.Equal(t, 91, result.AudioProgressSeconds)
}

func TestContentItemService_UpdateMarkPreservesIndependentState(t *testing.T) {
	ctrl := gomock.NewController(t)
	contentRepo := mock_repository.NewMockContentItemRepository(ctrl)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), contentRepo)
	ctx := context.Background()
	marked := false
	item := &model.ContentItem{Id: 42, FeedID: 7, Type: model.ContentItemTypeText, Title: "Article", ProcessingStatus: model.ContentItemProcessingStatusCompleted, MarkedLater: true, Favorited: false, AudioProgressSeconds: 91}

	contentRepo.EXPECT().UpdateMark(ctx, uint(42), model.ContentItemMarkFavorite, false).Return(nil)
	contentRepo.EXPECT().GetByID(ctx, uint(42)).Return(item, nil)

	result, err := svc.UpdateMark(ctx, 42, model.ContentItemMarkFavorite, &v1.UpdateContentItemMarkRequest{Marked: &marked})

	require.NoError(t, err)
	assert.Equal(t, "completed", result.ProcessingStatus)
	assert.True(t, result.MarkedLater)
	assert.False(t, result.Favorited)
	assert.Equal(t, 91, result.AudioProgressSeconds)
}

func TestContentItemService_UpdateAudioProgressPersistsOnlyAudioItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	contentRepo := mock_repository.NewMockContentItemRepository(ctrl)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), contentRepo)
	ctx := context.Background()
	audioProgress := 123
	textProgress := 321
	audio := &model.ContentItem{Id: 42, FeedID: 7, Type: model.ContentItemTypeAudio, Title: "Episode", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true}
	text := &model.ContentItem{Id: 43, FeedID: 7, Type: model.ContentItemTypeText, Title: "Article", ProcessingStatus: model.ContentItemProcessingStatusCompleted}
	updatedAudio := *audio
	updatedAudio.AudioProgressSeconds = audioProgress

	contentRepo.EXPECT().GetByID(ctx, uint(42)).Return(audio, nil)
	contentRepo.EXPECT().UpdateAudioProgress(ctx, uint(42), 123).Return(nil)
	contentRepo.EXPECT().GetByID(ctx, uint(42)).Return(&updatedAudio, nil)
	contentRepo.EXPECT().GetByID(ctx, uint(43)).Return(text, nil)

	result, err := svc.UpdateAudioProgress(ctx, 42, &v1.UpdateContentItemAudioProgressRequest{AudioProgressSeconds: &audioProgress})
	require.NoError(t, err)
	assert.Equal(t, "unprocessed", result.ProcessingStatus)
	assert.True(t, result.MarkedLater)
	assert.Equal(t, 123, result.AudioProgressSeconds)

	_, err = svc.UpdateAudioProgress(ctx, 43, &v1.UpdateContentItemAudioProgressRequest{AudioProgressSeconds: &textProgress})
	assert.ErrorIs(t, err, v1.ErrBadRequest)
}

func TestContentItemService_UpdateAudioProgressReturnsFreshDetail(t *testing.T) {
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := &audioProgressFreshDetailRepository{
		item: model.ContentItem{Id: 42, FeedID: 7, Type: model.ContentItemTypeAudio, Title: "Episode", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Favorited: false, AudioProgressSeconds: 10},
	}
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), repo)
	progress := 123

	result, err := svc.UpdateAudioProgress(context.Background(), 42, &v1.UpdateContentItemAudioProgressRequest{AudioProgressSeconds: &progress})

	require.NoError(t, err)
	assert.Equal(t, "completed", result.ProcessingStatus)
	assert.False(t, result.MarkedLater)
	assert.True(t, result.Favorited)
	assert.Equal(t, 123, result.AudioProgressSeconds)
}

func TestContentItemService_ListContentItemsReportsClampedPageMetadata(t *testing.T) {
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := &contentItemRepositorySpy{}
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), repo)

	result, err := svc.ListContentItems(context.Background(), &v1.ListContentItemsRequest{
		Page: v1.PageRequest{Page: math.MaxInt, PageSize: v1.MaxPageSize},
	})

	require.NoError(t, err)
	require.Equal(t, v1.MaxPageSize, repo.limit)
	require.Equal(t, (math.MaxInt/v1.MaxPageSize)*v1.MaxPageSize, repo.offset)
	require.Equal(t, math.MaxInt/v1.MaxPageSize+1, result.Page.Page)
	require.Equal(t, v1.MaxPageSize, result.Page.PageSize)
}

type contentItemRepositorySpy struct {
	filter repository.ContentItemListFilter
	limit  int
	offset int
}

func (r *contentItemRepositorySpy) Create(context.Context, *model.ContentItem) error { return nil }

func (r *contentItemRepositorySpy) GetByID(context.Context, uint) (*model.ContentItem, error) {
	return nil, v1.ErrNotFound
}

func (r *contentItemRepositorySpy) GetByFeedAndDedupeKey(context.Context, uint, string) (*model.ContentItem, error) {
	return nil, nil
}

func (r *contentItemRepositorySpy) List(_ context.Context, filter repository.ContentItemListFilter, limit int, offset int) ([]*model.ContentItem, int64, error) {
	r.filter = filter
	r.limit = limit
	r.offset = offset
	return nil, 0, nil
}

func (r *contentItemRepositorySpy) ListByFeedID(context.Context, uint, int) ([]*model.ContentItem, error) {
	return nil, nil
}

func (r *contentItemRepositorySpy) UpdateProcessingStatus(context.Context, uint, model.ContentItemProcessingStatus) error {
	return nil
}

func (r *contentItemRepositorySpy) UpdateMark(context.Context, uint, model.ContentItemMark, bool) error {
	return nil
}

func (r *contentItemRepositorySpy) UpdateAudioProgress(context.Context, uint, int) error { return nil }

type audioProgressFreshDetailRepository struct {
	item model.ContentItem
}

func (r *audioProgressFreshDetailRepository) Create(context.Context, *model.ContentItem) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) GetByID(_ context.Context, id uint) (*model.ContentItem, error) {
	if id != r.item.Id {
		return nil, v1.ErrNotFound
	}
	item := r.item
	return &item, nil
}

func (r *audioProgressFreshDetailRepository) GetByFeedAndDedupeKey(context.Context, uint, string) (*model.ContentItem, error) {
	return nil, nil
}

func (r *audioProgressFreshDetailRepository) List(context.Context, repository.ContentItemListFilter, int, int) ([]*model.ContentItem, int64, error) {
	return nil, 0, nil
}

func (r *audioProgressFreshDetailRepository) ListByFeedID(context.Context, uint, int) ([]*model.ContentItem, error) {
	return nil, nil
}

func (r *audioProgressFreshDetailRepository) UpdateProcessingStatus(context.Context, uint, model.ContentItemProcessingStatus) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) UpdateMark(context.Context, uint, model.ContentItemMark, bool) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) UpdateAudioProgress(_ context.Context, id uint, progressSeconds int) error {
	if id != r.item.Id {
		return v1.ErrNotFound
	}
	r.item.AudioProgressSeconds = progressSeconds
	r.item.ProcessingStatus = model.ContentItemProcessingStatusCompleted
	r.item.MarkedLater = false
	r.item.Favorited = true
	return nil
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
