package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestAutoSummaryService_RunAutoSummarySkipsWhenDisabled(t *testing.T) {
	ctx := context.Background()
	autoService, _, _, _, chat := newAutoSummaryServiceHarness(t)

	result, err := autoService.RunAutoSummary(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Enabled)
	assert.Zero(t, result.TotalCandidates)
	assert.Zero(t, chat.calls)
}

func TestAutoSummaryService_RunAutoSummaryHonorsContentTypeScope(t *testing.T) {
	cases := []struct {
		name               string
		scope              model.AutoSummaryContentTypeScope
		wantTextProcessed  bool
		wantAudioProcessed bool
		wantSucceeded      int
		wantCalls          int
	}{
		{name: "text", scope: model.AutoSummaryContentTypeScopeText, wantTextProcessed: true, wantSucceeded: 1, wantCalls: 1},
		{name: "audio", scope: model.AutoSummaryContentTypeScopeAudio, wantAudioProcessed: true, wantSucceeded: 1, wantCalls: 1},
		{name: "all", scope: model.AutoSummaryContentTypeScopeAll, wantTextProcessed: true, wantAudioProcessed: true, wantSucceeded: 2, wantCalls: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			autoService, feedRepo, contentRepo, autoConfigRepo, chat := newAutoSummaryServiceHarness(t)
			enabledAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			require.NoError(t, autoConfigRepo.Save(ctx, &model.AutoSummaryConfig{Enabled: true, ContentTypeScope: tc.scope, EnabledAt: &enabledAt}))
			feed := &model.Feed{FeedURL: "https://example.com/auto-summary-" + tc.name + ".xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
			require.NoError(t, feedRepo.Create(ctx, feed))
			textItem := createAutoSummaryServiceItem(t, ctx, contentRepo, feed.Id, "text", model.ContentItemTypeText, enabledAt.Add(time.Minute), stringsForSummary())
			audioItem := createAutoSummaryServiceItem(t, ctx, contentRepo, feed.Id, "audio", model.ContentItemTypeAudio, enabledAt.Add(2*time.Minute), stringsForSummary())

			result, err := autoService.RunAutoSummary(ctx)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Enabled)
			assert.Equal(t, tc.wantSucceeded, result.TotalCandidates)
			assert.Equal(t, tc.wantSucceeded, result.Succeeded)
			assert.Equal(t, tc.wantCalls, chat.calls)
			assertAutoSummaryStatus(t, ctx, contentRepo, textItem.Id, tc.wantTextProcessed)
			assertAutoSummaryStatus(t, ctx, contentRepo, audioItem.Id, tc.wantAudioProcessed)
		})
	}
}

func TestAutoSummaryService_RunAutoSummaryRecordsFailuresAndContinues(t *testing.T) {
	ctx := context.Background()
	autoService, feedRepo, contentRepo, autoConfigRepo, chat := newAutoSummaryServiceHarness(t)
	chat.responses = []chatCompletionResponse{
		{err: context.DeadlineExceeded},
		{content: "## TL;DR\n后续成功摘要"},
	}
	enabledAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, autoConfigRepo.Save(ctx, &model.AutoSummaryConfig{Enabled: true, ContentTypeScope: model.AutoSummaryContentTypeScopeAll, EnabledAt: &enabledAt}))
	feed := &model.Feed{FeedURL: "https://example.com/auto-summary-failures.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	failedItem := createAutoSummaryServiceItem(t, ctx, contentRepo, feed.Id, "failed", model.ContentItemTypeText, enabledAt.Add(time.Minute), stringsForSummary())
	shortItem := createAutoSummaryServiceItem(t, ctx, contentRepo, feed.Id, "short", model.ContentItemTypeText, enabledAt.Add(2*time.Minute), "太短")
	successItem := createAutoSummaryServiceItem(t, ctx, contentRepo, feed.Id, "success", model.ContentItemTypeText, enabledAt.Add(3*time.Minute), stringsForSummary())

	result, err := autoService.RunAutoSummary(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.TotalCandidates)
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 1, result.InsufficientText)
	assert.Equal(t, 2, chat.calls)
	assertContentAISummaryState(t, ctx, contentRepo, failedItem.Id, model.AISummaryStatusFailed)
	assertContentAISummaryState(t, ctx, contentRepo, shortItem.Id, model.AISummaryStatusInsufficientText)
	assertContentAISummaryState(t, ctx, contentRepo, successItem.Id, model.AISummaryStatusSuccess)
}

func newAutoSummaryServiceHarness(t *testing.T) (service.AutoSummaryService, repository.FeedRepository, repository.ContentItemRepository, repository.AutoSummaryConfigRepository, *sequencedChatCompletion) {
	t.Helper()
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := openServiceTestDB(t, logger)
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	aiConfigRepo := repository.NewAIServiceConfigRepository(repo)
	autoConfigRepo := repository.NewAutoSummaryConfigRepository(repo)
	chat := &sequencedChatCompletion{responses: []chatCompletionResponse{{content: "## TL;DR\n自动摘要"}}}
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	contentService := service.NewContentItemService(base, contentRepo, aiConfigRepo, chat)
	require.NoError(t, aiConfigRepo.Save(context.Background(), &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	return service.NewAutoSummaryService(base, autoConfigRepo, contentRepo, contentService), feedRepo, contentRepo, autoConfigRepo, chat
}

func createAutoSummaryServiceItem(t *testing.T, ctx context.Context, contentRepo repository.ContentItemRepository, feedID uint, dedupeKey string, contentType model.ContentItemType, createdAt time.Time, availableText string) *model.ContentItem {
	t.Helper()
	item := &model.ContentItem{
		FeedID:        feedID,
		DedupeKey:     dedupeKey,
		Type:          contentType,
		Title:         dedupeKey,
		AvailableText: availableText,
		CreatedAt:     createdAt,
	}
	require.NoError(t, contentRepo.Create(ctx, item))
	return item
}

func stringsForSummary() string {
	return strings.Repeat("这是一段足够长的可用文本，用来生成结构化中文摘要。", 4)
}

func assertAutoSummaryStatus(t *testing.T, ctx context.Context, contentRepo repository.ContentItemRepository, itemID uint, wantProcessed bool) {
	t.Helper()
	if wantProcessed {
		assertContentAISummaryState(t, ctx, contentRepo, itemID, model.AISummaryStatusSuccess)
		return
	}
	assertContentAISummaryState(t, ctx, contentRepo, itemID, model.AISummaryStatusNone)
}

func assertContentAISummaryState(t *testing.T, ctx context.Context, contentRepo repository.ContentItemRepository, itemID uint, status model.AISummaryStatus) {
	t.Helper()
	item, err := contentRepo.GetByID(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, status, item.AISummaryStatus)
}

type chatCompletionResponse struct {
	content string
	err     error
}

type sequencedChatCompletion struct {
	responses []chatCompletionResponse
	calls     int
}

func (c *sequencedChatCompletion) ChatCompletion(context.Context, model.AIServiceConfig, []service.ChatCompletionMessage) (string, error) {
	c.calls++
	if c.calls <= len(c.responses) {
		response := c.responses[c.calls-1]
		return response.content, response.err
	}
	return c.responses[len(c.responses)-1].content, c.responses[len(c.responses)-1].err
}
