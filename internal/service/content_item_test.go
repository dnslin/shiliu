package service_test

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
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
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, nil, nil)

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

func TestContentItemService_ExportObsidianMarkdownIncludesMetadataAndSuccessSummary(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	tagRepo := repository.NewTagRepository(repo)
	folderRepo := repository.NewFolderRepository(repo)
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, nil, nil)

	folder := &model.Folder{Name: "Engineering"}
	require.NoError(t, folderRepo.Create(ctx, folder))
	feed := &model.Feed{
		FeedURL:     "https://example.com/feed.xml",
		Title:       "Example Feed",
		Type:        model.FeedTypeRSS,
		FetchStatus: model.FeedFetchStatusIdle,
		FolderID:    &folder.Id,
	}
	require.NoError(t, feedRepo.Create(ctx, feed))
	publishedAt := time.Date(2026, 7, 2, 8, 30, 0, 0, time.UTC)
	generatedAt := publishedAt.Add(time.Hour)
	item := &model.ContentItem{
		FeedID:               feed.Id,
		DedupeKey:            "export-success",
		Type:                 model.ContentItemTypeText,
		Title:                "Export me",
		AvailableText:        "This is available text.",
		PublishedAt:          &publishedAt,
		AISummaryStatus:      model.AISummaryStatusSuccess,
		AISummaryMarkdown:    "## TL;DR\nExport summary",
		AISummaryGeneratedAt: &generatedAt,
	}
	require.NoError(t, contentRepo.Create(ctx, item))
	goTag := &model.Tag{Name: "go"}
	sqliteTag := &model.Tag{Name: "sqlite"}
	require.NoError(t, tagRepo.Create(ctx, sqliteTag))
	require.NoError(t, tagRepo.Create(ctx, goTag))
	require.NoError(t, contentRepo.AssignTags(ctx, item.Id, []uint{sqliteTag.Id, goTag.Id}))

	result, err := svc.ExportObsidianMarkdown(ctx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, item.Id, result.ContentItemID)
	assert.Equal(t, "Export me.md", result.Filename)
	assert.Contains(t, result.Markdown, "# Export me")
	assert.Contains(t, result.Markdown, "- 标题：Export me")
	assert.Contains(t, result.Markdown, "- 链接：https://example.com/feed.xml")
	assert.Contains(t, result.Markdown, "- 订阅源：Example Feed")
	assert.Contains(t, result.Markdown, "- 发布时间：2026-07-02T08:30:00Z")
	assert.Contains(t, result.Markdown, "- 内容类型：text")
	assert.Contains(t, result.Markdown, "- 标签：go, sqlite")
	assert.Contains(t, result.Markdown, "- 订阅源文件夹：Engineering")
	assert.Contains(t, result.Markdown, "## AI 摘要\n\n## TL;DR\nExport summary")
	assert.Contains(t, result.Markdown, "## 可用文本摘录\n\nThis is available text.")
	assert.NotContains(t, result.Markdown, "已截断，请打开原文链接查看完整内容")
}

func TestContentItemService_ExportObsidianMarkdownMapsSummaryStates(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, nil, nil)

	feed := &model.Feed{FeedURL: "https://example.com/summary-states.xml", Title: "Summary states", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))

	cases := []struct {
		name       string
		status     model.AISummaryStatus
		summary    string
		summaryErr string
		want       string
	}{
		{name: "none", status: model.AISummaryStatusNone, want: "## AI 摘要\n\n未生成"},
		{name: "pending", status: model.AISummaryStatusPending, want: "## AI 摘要\n\n正在生成"},
		{name: "failed", status: model.AISummaryStatusFailed, summaryErr: "AI timeout", want: "## AI 摘要\n\n生成失败：AI timeout"},
		{name: "insufficient", status: model.AISummaryStatusInsufficientText, summaryErr: "too short", want: "## AI 摘要\n\n可用文本不足"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			item := &model.ContentItem{
				FeedID:            feed.Id,
				DedupeKey:         "export-" + tt.name,
				Type:              model.ContentItemTypeText,
				Title:             "Export " + tt.name,
				AvailableText:     "state text",
				AISummaryStatus:   tt.status,
				AISummaryMarkdown: tt.summary,
				AISummaryError:    tt.summaryErr,
			}
			require.NoError(t, contentRepo.Create(ctx, item))

			result, err := svc.ExportObsidianMarkdown(ctx, item.Id)

			require.NoError(t, err)
			assert.Contains(t, result.Markdown, tt.want)
		})
	}
}

func TestContentItemService_ExportObsidianMarkdownTruncatesAvailableTextByRunes(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, nil, nil)

	feed := &model.Feed{FeedURL: "https://example.com/truncate.xml", Title: "Truncate", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	longText := strings.Repeat("界", 2001)
	item := &model.ContentItem{
		FeedID:          feed.Id,
		DedupeKey:       "export-truncated",
		Type:            model.ContentItemTypeText,
		Title:           "Export truncated",
		AvailableText:   longText,
		AISummaryStatus: model.AISummaryStatusNone,
	}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.ExportObsidianMarkdown(ctx, item.Id)

	require.NoError(t, err)
	assert.Contains(t, result.Markdown, "## 可用文本摘录\n\n"+strings.Repeat("界", 2000))
	assert.NotContains(t, result.Markdown, strings.Repeat("界", 2001))
	assert.Contains(t, result.Markdown, "已截断，请打开原文链接查看完整内容")

	short := &model.ContentItem{
		FeedID:          feed.Id,
		DedupeKey:       "export-short",
		Type:            model.ContentItemTypeText,
		Title:           "Export short",
		AvailableText:   strings.Repeat("界", 2000),
		AISummaryStatus: model.AISummaryStatusNone,
	}
	require.NoError(t, contentRepo.Create(ctx, short))

	shortResult, err := svc.ExportObsidianMarkdown(ctx, short.Id)

	require.NoError(t, err)
	assert.Contains(t, shortResult.Markdown, "## 可用文本摘录\n\n"+strings.Repeat("界", 2000))
	assert.NotContains(t, shortResult.Markdown, "已截断，请打开原文链接查看完整内容")
}

func TestContentItemService_UpdateProcessingStatusUsesRepositoryAndReturnsDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	contentRepo := mock_repository.NewMockContentItemRepository(ctrl)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), contentRepo, nil, nil)
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
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), contentRepo, nil, nil)
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
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), contentRepo, nil, nil)
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
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), repo, nil, nil)
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
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), repo, nil, nil)

	result, err := svc.ListContentItems(context.Background(), &v1.ListContentItemsRequest{
		Page: v1.PageRequest{Page: math.MaxInt, PageSize: v1.MaxPageSize},
	})

	require.NoError(t, err)
	require.Equal(t, v1.MaxPageSize, repo.limit)
	require.Equal(t, (math.MaxInt/v1.MaxPageSize)*v1.MaxPageSize, repo.offset)
	require.Equal(t, math.MaxInt/v1.MaxPageSize+1, result.Page.Page)
	require.Equal(t, v1.MaxPageSize, result.Page.PageSize)
}

func TestContentItemService_ListContentItemsParsesTagIDFilter(t *testing.T) {
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := &contentItemRepositorySpy{}
	svc := service.NewContentItemService(service.NewService(nil, logger, nil, nil), repo, nil, nil)

	_, err := svc.ListContentItems(context.Background(), &v1.ListContentItemsRequest{TagID: "42"})

	require.NoError(t, err)
	require.NotNil(t, repo.filter.TagID)
	assert.Equal(t, uint(42), *repo.filter.TagID)
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

func (r *contentItemRepositorySpy) GetExportDataByID(context.Context, uint) (*repository.ContentItemExportData, error) {
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

func (r *contentItemRepositorySpy) ListAutoSummaryCandidates(context.Context, repository.AutoSummaryCandidateFilter, int) ([]*model.ContentItem, error) {
	return nil, nil
}

func (r *contentItemRepositorySpy) AssignTags(context.Context, uint, []uint) error { return nil }

func (r *contentItemRepositorySpy) RemoveTags(context.Context, uint, []uint) error { return nil }

func (r *contentItemRepositorySpy) UpdateProcessingStatus(context.Context, uint, model.ContentItemProcessingStatus) error {
	return nil
}

func (r *contentItemRepositorySpy) UpdateMark(context.Context, uint, model.ContentItemMark, bool) error {
	return nil
}

func (r *contentItemRepositorySpy) UpdateAudioProgress(context.Context, uint, int) error { return nil }
func (r *contentItemRepositorySpy) UpdateSearchText(context.Context, uint, string, string) error {
	return nil
}

func (r *contentItemRepositorySpy) UpdateAISummarySearchText(context.Context, uint, string) error {
	return nil
}

func (r *contentItemRepositorySpy) ClaimAISummary(context.Context, uint, []model.AISummaryStatus) error {
	return nil
}

func (r *contentItemRepositorySpy) UpdateAISummary(context.Context, uint, model.AISummaryStatus, string, *time.Time, string) error {
	return nil
}

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

func (r *audioProgressFreshDetailRepository) GetExportDataByID(context.Context, uint) (*repository.ContentItemExportData, error) {
	return nil, v1.ErrNotFound
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

func (r *audioProgressFreshDetailRepository) ListAutoSummaryCandidates(context.Context, repository.AutoSummaryCandidateFilter, int) ([]*model.ContentItem, error) {
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

func (r *audioProgressFreshDetailRepository) UpdateSearchText(context.Context, uint, string, string) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) UpdateAISummarySearchText(context.Context, uint, string) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) ClaimAISummary(context.Context, uint, []model.AISummaryStatus) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) UpdateAISummary(context.Context, uint, model.AISummaryStatus, string, *time.Time, string) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) AssignTags(context.Context, uint, []uint) error {
	return nil
}

func (r *audioProgressFreshDetailRepository) RemoveTags(context.Context, uint, []uint) error {
	return nil
}
func TestContentItemService_GetContentItemReturnsDetail(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, nil, nil)

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

func TestContentItemService_GenerateAISummaryStoresMarkdownAndRefreshesSearch(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	markdown := "## TL;DR\n自托管 摘要 搜索\n\n## 要点\n- 内容对开发者有价值\n\n## 对开发者 / 信息重度用户的价值\n可沉淀。\n\n## 原文信息\n标题：AI Summary"
	chat := &recordingChatCompletion{content: markdown}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)

	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/service-ai-summary.xml", Title: "AI 周报", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "service-summary", Type: model.ContentItemTypeText, Title: "AI Summary", AvailableText: strings.Repeat("可用文本包含足够上下文，适合生成可靠的结构化中文摘要。", 4)}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GenerateAISummary(ctx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, item.Id, result.ContentItemID)
	assert.Equal(t, "success", result.State)
	assert.Equal(t, markdown, result.Markdown)
	require.NotNil(t, result.GeneratedAt)
	assert.Equal(t, "", result.Error)
	require.Len(t, chat.messages, 2)
	assert.Contains(t, chat.messages[0].Content, "固定结构化 Markdown")
	assert.Contains(t, chat.messages[1].Content, item.AvailableText)

	stored, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AISummaryStatusSuccess, stored.AISummaryStatus)
	assert.Equal(t, markdown, stored.AISummaryMarkdown)
	var rowID uint
	require.NoError(t, db.Raw(`SELECT rowid FROM content_item_search_index WHERE content_item_search_index MATCH ?`, "自托管").Scan(&rowID).Error)
	assert.Equal(t, item.Id, rowID)
}

func TestContentItemService_GenerateAISummaryMarksInsufficientTextAsTerminal(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{content: "## TL;DR\n不应调用"}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/short-summary.xml", Title: "短文本", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "short-summary", Type: model.ContentItemTypeAudio, Title: "Short", AvailableText: "太短"}
	require.NoError(t, contentRepo.Create(ctx, item))

	first, err := svc.GenerateAISummary(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "insufficient_text", first.State)
	assert.Contains(t, first.Message, "可用文本不足")
	assert.Empty(t, chat.messages)

	second, err := svc.GenerateAISummary(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "insufficient_text", second.State)
	assert.Contains(t, second.Message, "不可重试")
	assert.Empty(t, chat.messages)
}

func TestContentItemService_GenerateAISummaryReturnsPendingWithoutCallingModel(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{content: "## TL;DR\n不应调用"}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/pending-summary.xml", Title: "Pending", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "pending-summary", Type: model.ContentItemTypeText, Title: "Pending", AvailableText: strings.Repeat("足够的可用文本。", 20), AISummaryStatus: model.AISummaryStatusPending}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GenerateAISummary(ctx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, "pending", result.State)
	assert.Equal(t, "正在生成", result.Message)
	assert.Empty(t, chat.messages)
}

func TestContentItemService_GenerateAISummaryRecordsCanceledGenerationAfterRequestContextCanceled(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	requestCtx, cancelRequest := context.WithCancel(ctx)
	chat := &cancelingChatCompletion{cancel: cancelRequest}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/canceled-summary.xml", Title: "Canceled", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "canceled-summary", Type: model.ContentItemTypeText, Title: "Canceled", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GenerateAISummary(requestCtx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, "failed", result.State)
	assert.Contains(t, result.Error, "超时")
	stored, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AISummaryStatusFailed, stored.AISummaryStatus)
	assert.Contains(t, stored.AISummaryError, "超时")
}

func TestContentItemService_GenerateAISummaryConcurrentRequestReturnsPendingWithoutDuplicateModelCall(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := newBlockingChatCompletion("## TL;DR\n并发首个摘要")
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/concurrent-summary.xml", Title: "Concurrent", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "concurrent-summary", Type: model.ContentItemTypeText, Title: "Concurrent", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	require.NoError(t, contentRepo.Create(ctx, item))

	firstResult := make(chan *v1.AISummaryResponseData, 1)
	firstErr := make(chan error, 1)
	go func() {
		result, err := svc.GenerateAISummary(ctx, item.Id)
		firstResult <- result
		firstErr <- err
	}()
	<-chat.started

	second, err := svc.GenerateAISummary(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "pending", second.State)
	assert.Equal(t, "正在生成", second.Message)
	assert.Equal(t, int32(1), chat.calls.Load())

	close(chat.release)
	require.NoError(t, <-firstErr)
	assert.Equal(t, "success", (<-firstResult).State)
	assert.Equal(t, int32(1), chat.calls.Load())
}

func TestContentItemService_GenerateAISummaryRecordsFailureAndManualRetryOverwrites(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{err: context.DeadlineExceeded}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/retry-summary.xml", Title: "Retry", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "retry-summary", Type: model.ContentItemTypeText, Title: "Retry", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	require.NoError(t, contentRepo.Create(ctx, item))

	failed, err := svc.GenerateAISummary(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "failed", failed.State)
	assert.Contains(t, failed.Error, "超时")
	assert.Equal(t, 1, chat.calls)
	storedFailed, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AISummaryStatusFailed, storedFailed.AISummaryStatus)
	assert.Contains(t, storedFailed.AISummaryError, "超时")

	chat.err = nil
	chat.content = "## TL;DR\n重试成功 摘要"
	success, err := svc.GenerateAISummary(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "success", success.State)
	assert.Equal(t, "## TL;DR\n重试成功 摘要", success.Markdown)
	assert.Equal(t, 2, chat.calls)
	storedSuccess, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AISummaryStatusSuccess, storedSuccess.AISummaryStatus)
	assert.Equal(t, "", storedSuccess.AISummaryError)
}

func TestContentItemService_GenerateAISummaryRecordsEmptyResponseAsFailure(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{content: "   "}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/empty-summary.xml", Title: "Empty", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "empty-summary", Type: model.ContentItemTypeText, Title: "Empty", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GenerateAISummary(ctx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, "failed", result.State)
	assert.Contains(t, result.Error, "响应为空")
	stored, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AISummaryStatusFailed, stored.AISummaryStatus)
	assert.Contains(t, stored.AISummaryError, "响应为空")
}

func TestContentItemService_GenerateAISummaryOverwritesExistingSuccess(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{content: "## TL;DR\n新摘要"}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/overwrite-summary.xml", Title: "Overwrite", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	oldGeneratedAt := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "overwrite-summary", Type: model.ContentItemTypeText, Title: "Overwrite", AvailableText: strings.Repeat("足够的可用文本。", 20), AISummaryStatus: model.AISummaryStatusSuccess, AISummaryMarkdown: "## TL;DR\n旧摘要", AISummaryGeneratedAt: &oldGeneratedAt, AISummaryError: "old error"}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GenerateAISummary(ctx, item.Id)

	require.NoError(t, err)
	assert.Equal(t, "success", result.State)
	assert.Equal(t, "## TL;DR\n新摘要", result.Markdown)
	stored, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, "## TL;DR\n新摘要", stored.AISummaryMarkdown)
	assert.Equal(t, "", stored.AISummaryError)
	require.NotNil(t, stored.AISummaryGeneratedAt)
	assert.True(t, stored.AISummaryGeneratedAt.After(oldGeneratedAt))
}

func TestContentItemService_GenerateAutoAISummaryProcessesOnlyNone(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{content: "## TL;DR\n自动摘要"}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), contentRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/auto-summary-only-none.xml", Title: "Auto", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))

	noneItem := &model.ContentItem{FeedID: feed.Id, DedupeKey: "auto-none", Type: model.ContentItemTypeText, Title: "Auto none", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	require.NoError(t, contentRepo.Create(ctx, noneItem))
	processed, err := svc.GenerateAutoAISummary(ctx, noneItem.Id)
	require.NoError(t, err)
	require.NotNil(t, processed)
	assert.False(t, processed.Skipped)
	require.NotNil(t, processed.Response)
	assert.Equal(t, "success", processed.Response.State)
	assert.Equal(t, 1, chat.calls)

	for _, tc := range []struct {
		status   model.AISummaryStatus
		markdown string
		errorMsg string
	}{
		{status: model.AISummaryStatusSuccess, markdown: "## TL;DR\n已有摘要"},
		{status: model.AISummaryStatusFailed, errorMsg: "AI 摘要生成超时"},
		{status: model.AISummaryStatusInsufficientText, errorMsg: "可用文本不足"},
		{status: model.AISummaryStatusPending},
	} {
		item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "auto-" + string(tc.status), Type: model.ContentItemTypeText, Title: string(tc.status), AvailableText: strings.Repeat("足够的可用文本。", 20), AISummaryStatus: tc.status, AISummaryMarkdown: tc.markdown, AISummaryError: tc.errorMsg}
		require.NoError(t, contentRepo.Create(ctx, item))

		result, err := svc.GenerateAutoAISummary(ctx, item.Id)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Skipped, tc.status)
		stored, err := contentRepo.GetByID(ctx, item.Id)
		require.NoError(t, err)
		assert.Equal(t, tc.status, stored.AISummaryStatus)
		assert.Equal(t, tc.markdown, stored.AISummaryMarkdown)
		assert.Equal(t, tc.errorMsg, stored.AISummaryError)
	}
	assert.Equal(t, 1, chat.calls)
}

func TestContentItemService_GenerateAutoAISummaryDoesNotOverwriteRaceToSuccess(t *testing.T) {
	ctx := context.Background()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	raceRepo := &raceToSuccessContentRepository{ContentItemRepository: contentRepo}
	configRepo := repository.NewAIServiceConfigRepository(repo)
	chat := &recordingChatCompletion{content: "## TL;DR\n不应调用"}
	svc := service.NewContentItemService(service.NewService(repository.NewTransaction(repo), logger, nil, nil), raceRepo, configRepo, chat)
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	feed := &model.Feed{FeedURL: "https://example.com/auto-summary-race.xml", Title: "Auto race", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "auto-race", Type: model.ContentItemTypeText, Title: "Auto race", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	require.NoError(t, contentRepo.Create(ctx, item))

	result, err := svc.GenerateAutoAISummary(ctx, item.Id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Skipped)
	assert.Equal(t, 0, chat.calls)
	stored, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AISummaryStatusSuccess, stored.AISummaryStatus)
	assert.Equal(t, "## TL;DR\n手动成功", stored.AISummaryMarkdown)
}

type recordingChatCompletion struct {
	content  string
	err      error
	messages []service.ChatCompletionMessage
	calls    int
}

func (c *recordingChatCompletion) ChatCompletion(_ context.Context, _ model.AIServiceConfig, messages []service.ChatCompletionMessage) (string, error) {
	c.calls++
	c.messages = append([]service.ChatCompletionMessage(nil), messages...)
	return c.content, c.err
}

type raceToSuccessContentRepository struct {
	repository.ContentItemRepository
	raced bool
}

func (r *raceToSuccessContentRepository) GetByID(ctx context.Context, id uint) (*model.ContentItem, error) {
	item, err := r.ContentItemRepository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.raced {
		generatedAt := time.Now().UTC()
		if err := r.ContentItemRepository.UpdateAISummary(ctx, id, model.AISummaryStatusSuccess, "## TL;DR\n手动成功", &generatedAt, ""); err != nil {
			return nil, err
		}
		item.AISummaryStatus = model.AISummaryStatusNone
		item.AISummaryMarkdown = ""
		item.AISummaryGeneratedAt = nil
		item.AISummaryError = ""
		r.raced = true
	}
	return item, nil
}

type cancelingChatCompletion struct {
	cancel context.CancelFunc
}

func (c *cancelingChatCompletion) ChatCompletion(ctx context.Context, _ model.AIServiceConfig, _ []service.ChatCompletionMessage) (string, error) {
	c.cancel()
	<-ctx.Done()
	return "", ctx.Err()
}

type blockingChatCompletion struct {
	content string
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func newBlockingChatCompletion(content string) *blockingChatCompletion {
	return &blockingChatCompletion{
		content: content,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (c *blockingChatCompletion) ChatCompletion(_ context.Context, _ model.AIServiceConfig, _ []service.ChatCompletionMessage) (string, error) {
	call := c.calls.Add(1)
	if call == 1 {
		close(c.started)
		<-c.release
	}
	return c.content, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
