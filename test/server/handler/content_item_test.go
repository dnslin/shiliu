package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	v1 "shiliu/api/v1"
	"shiliu/internal/handler"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestContentItemHandler_ListContentItemsReturnsFilteredPage(t *testing.T) {
	fetchedAt := time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC)
	contentService := &fakeContentItemService{
		listResult: &v1.ListContentItemsResponseData{
			Items: []v1.ContentItemListItemData{
				{Id: 42, FeedID: 7, ContentType: "text", Title: "Published newer", AvailableText: "Published newer", FetchedAt: fetchedAt, ProcessingStatus: "unprocessed", MarkedLater: true},
			},
			Page: v1.PageMeta{Page: 2, PageSize: 1, Total: 2},
		},
	}
	contentHandler := handler.NewContentItemHandler(hdl, contentService)
	r := gin.New()
	r.GET("/content-items", contentHandler.ListContentItems)

	obj := newHttpExcept(t, r).GET("/content-items").
		WithQuery("content_type", "text").
		WithQuery("processing_status", "unprocessed").
		WithQuery("mark", "later").
		WithQuery("feed_id", "7").
		WithQuery("tag_id", "3").
		WithQuery("page", "2").
		WithQuery("pageSize", "1").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	data := obj.Value("data").Object()
	page := data.Value("page").Object()
	page.Value("page").IsEqual(2)
	page.Value("pageSize").IsEqual(1)
	page.Value("total").IsEqual(2)
	items := data.Value("items").Array()
	items.Length().IsEqual(1)
	first := items.Value(0).Object()
	first.Value("id").IsEqual(42)
	first.Value("feedId").IsEqual(7)
	first.Value("contentType").IsEqual("text")
	first.Value("title").IsEqual("Published newer")
	first.Value("availableText").IsEqual("Published newer")
	first.Value("processingStatus").IsEqual("unprocessed")
	first.Value("markedLater").IsEqual(true)
	first.Value("favorited").IsEqual(false)

	if contentService.listCalls != 1 {
		t.Fatalf("expected ListContentItems to be called once, got %d", contentService.listCalls)
	}
	if contentService.lastListRequest.ContentType != "text" || contentService.lastListRequest.ProcessingStatus != "unprocessed" || contentService.lastListRequest.Mark != "later" || contentService.lastListRequest.FeedID != "7" || contentService.lastListRequest.TagID != "3" {
		t.Fatalf("handler passed request %#v", contentService.lastListRequest)
	}
	if contentService.lastListRequest.Page.Page != 2 || contentService.lastListRequest.Page.PageSize != 1 {
		t.Fatalf("handler passed page %#v", contentService.lastListRequest.Page)
	}
}

func TestContentItemHandler_ListContentItemsWithKeywordUsesRelevanceAndPagination(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/search-view.xml", Title: "数据库周报", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	base := time.Date(2026, 6, 26, 7, 0, 0, 0, time.UTC)
	newer := base.Add(2 * time.Hour)
	newest := base.Add(3 * time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: feed.Id, DedupeKey: "high-relevance", Type: model.ContentItemTypeText, Title: "SQLite SQLite SQLite 深度调优", AvailableText: "SQLite 查询计划与 SQLite 索引优化", PublishedAt: &base},
		{FeedID: feed.Id, DedupeKey: "newer-lower-relevance", Type: model.ContentItemTypeText, Title: "SQLite 入门", AvailableText: "基础教程", PublishedAt: &newer},
		{FeedID: feed.Id, DedupeKey: "newest-miss", Type: model.ContentItemTypeText, Title: "Postgres 新闻", AvailableText: "Postgres", PublishedAt: &newest},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	firstPage := newHttpExcept(t, r).GET("/content-items").
		WithQuery("keyword", "SQLite").
		WithQuery("page", "1").
		WithQuery("pageSize", "1").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	firstData := firstPage.Value("data").Object()
	firstData.Value("page").Object().Value("total").IsEqual(2)
	firstItems := firstData.Value("items").Array()
	firstItems.Length().IsEqual(1)
	firstItems.Value(0).Object().Value("title").IsEqual("SQLite SQLite SQLite 深度调优")

	secondPage := newHttpExcept(t, r).GET("/content-items").
		WithQuery("keyword", "SQLite").
		WithQuery("page", "2").
		WithQuery("pageSize", "1").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	secondData := secondPage.Value("data").Object()
	secondData.Value("page").Object().Value("total").IsEqual(2)
	secondItems := secondData.Value("items").Array()
	secondItems.Length().IsEqual(1)
	secondItems.Value(0).Object().Value("title").IsEqual("SQLite 入门")
}

func TestContentItemHandler_ListContentItemsWithoutKeywordUsesPublishedOrderAndPagination(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/no-keyword-view.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	base := time.Date(2026, 6, 26, 6, 0, 0, 0, time.UTC)
	middle := base.Add(time.Hour)
	newest := base.Add(2 * time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: feed.Id, DedupeKey: "oldest", Type: model.ContentItemTypeText, Title: "Oldest", AvailableText: "Oldest", PublishedAt: &base},
		{FeedID: feed.Id, DedupeKey: "middle", Type: model.ContentItemTypeText, Title: "Middle", AvailableText: "Middle", PublishedAt: &middle},
		{FeedID: feed.Id, DedupeKey: "newest", Type: model.ContentItemTypeText, Title: "Newest", AvailableText: "Newest", PublishedAt: &newest},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	page := newHttpExcept(t, r).GET("/content-items").
		WithQuery("page", "2").
		WithQuery("pageSize", "1").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	data := page.Value("data").Object()
	data.Value("page").Object().Value("total").IsEqual(3)
	items := data.Value("items").Array()
	items.Length().IsEqual(1)
	items.Value(0).Object().Value("title").IsEqual("Middle")
}

func TestContentItemHandler_ListContentItemsRejectsOversizedKeyword(t *testing.T) {
	r, _, _ := newContentViewTestHarness(t)

	obj := newHttpExcept(t, r).GET("/content-items").
		WithQuery("keyword", strings.Repeat("x", 129)).
		Expect().
		Status(http.StatusBadRequest).
		JSON().
		Object()
	obj.Value("code").IsEqual(3002)
}

func TestContentItemHandler_ListInboxContentItemsAppliesUnprocessedPreset(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/inbox-view.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	base := time.Date(2026, 6, 26, 8, 0, 0, 0, time.UTC)
	completedAt := base.Add(2 * time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: feed.Id, DedupeKey: "inbox-text", Type: model.ContentItemTypeText, Title: "Inbox text", AvailableText: "Inbox text", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, PublishedAt: &base},
		{FeedID: feed.Id, DedupeKey: "completed-text", Type: model.ContentItemTypeText, Title: "Completed text", AvailableText: "Completed text", ProcessingStatus: model.ContentItemProcessingStatusCompleted, PublishedAt: &completedAt},
		{FeedID: feed.Id, DedupeKey: "inbox-audio", Type: model.ContentItemTypeAudio, Title: "Inbox audio", AvailableText: "Inbox audio", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, PublishedAt: &completedAt},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	obj := newHttpExcept(t, r).GET("/content-views/inbox").
		WithQuery("content_type", "text").
		WithQuery("page", "1").
		WithQuery("pageSize", "10").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	data := obj.Value("data").Object()
	items := data.Value("items").Array()
	items.Length().IsEqual(1)
	first := items.Value(0).Object()
	first.Value("title").IsEqual("Inbox text")
	first.Value("processingStatus").IsEqual("unprocessed")
	first.Value("contentType").IsEqual("text")
}

func TestContentItemHandler_ListLaterContentItemsAppliesLaterPresetWithAdditionalStatus(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/later-view.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	base := time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC)
	completedAt := base.Add(2 * time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: feed.Id, DedupeKey: "later-unprocessed", Type: model.ContentItemTypeText, Title: "Later unprocessed", AvailableText: "Later unprocessed", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, PublishedAt: &base},
		{FeedID: feed.Id, DedupeKey: "later-completed", Type: model.ContentItemTypeText, Title: "Later completed", AvailableText: "Later completed", ProcessingStatus: model.ContentItemProcessingStatusCompleted, MarkedLater: true, PublishedAt: &completedAt},
		{FeedID: feed.Id, DedupeKey: "unmarked-unprocessed", Type: model.ContentItemTypeText, Title: "Unmarked unprocessed", AvailableText: "Unmarked unprocessed", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, PublishedAt: &completedAt},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	obj := newHttpExcept(t, r).GET("/content-views/later").
		WithQuery("processing_status", "unprocessed").
		WithQuery("page", "1").
		WithQuery("pageSize", "10").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	items := obj.Value("data").Object().Value("items").Array()
	items.Length().IsEqual(1)
	first := items.Value(0).Object()
	first.Value("title").IsEqual("Later unprocessed")
	first.Value("markedLater").IsEqual(true)
	first.Value("processingStatus").IsEqual("unprocessed")
}

func TestContentItemHandler_ListFavoriteContentItemsAppliesFavoritePresetWithAdditionalType(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/favorite-view.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	newer := base.Add(time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: feed.Id, DedupeKey: "favorite-text", Type: model.ContentItemTypeText, Title: "Favorite text", AvailableText: "Favorite text", Favorited: true, PublishedAt: &base},
		{FeedID: feed.Id, DedupeKey: "favorite-audio", Type: model.ContentItemTypeAudio, Title: "Favorite audio", AvailableText: "Favorite audio", Favorited: true, PublishedAt: &newer},
		{FeedID: feed.Id, DedupeKey: "plain-text", Type: model.ContentItemTypeText, Title: "Plain text", AvailableText: "Plain text", PublishedAt: &newer},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	obj := newHttpExcept(t, r).GET("/content-views/favorite").
		WithQuery("content_type", "text").
		WithQuery("page", "1").
		WithQuery("pageSize", "10").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	items := obj.Value("data").Object().Value("items").Array()
	items.Length().IsEqual(1)
	first := items.Value(0).Object()
	first.Value("title").IsEqual("Favorite text")
	first.Value("favorited").IsEqual(true)
	first.Value("contentType").IsEqual("text")
}

func TestContentItemHandler_ListCompletedContentItemsAppliesCompletedPresetWithAdditionalMark(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/completed-view.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	base := time.Date(2026, 6, 26, 11, 0, 0, 0, time.UTC)
	newer := base.Add(time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: feed.Id, DedupeKey: "completed-favorite", Type: model.ContentItemTypeText, Title: "Completed favorite", AvailableText: "Completed favorite", ProcessingStatus: model.ContentItemProcessingStatusCompleted, Favorited: true, PublishedAt: &base},
		{FeedID: feed.Id, DedupeKey: "completed-plain", Type: model.ContentItemTypeText, Title: "Completed plain", AvailableText: "Completed plain", ProcessingStatus: model.ContentItemProcessingStatusCompleted, PublishedAt: &newer},
		{FeedID: feed.Id, DedupeKey: "unprocessed-favorite", Type: model.ContentItemTypeText, Title: "Unprocessed favorite", AvailableText: "Unprocessed favorite", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, Favorited: true, PublishedAt: &newer},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	obj := newHttpExcept(t, r).GET("/content-views/completed").
		WithQuery("mark", "favorite").
		WithQuery("page", "1").
		WithQuery("pageSize", "10").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	items := obj.Value("data").Object().Value("items").Array()
	items.Length().IsEqual(1)
	first := items.Value(0).Object()
	first.Value("title").IsEqual("Completed favorite")
	first.Value("processingStatus").IsEqual("completed")
	first.Value("favorited").IsEqual(true)
}

func TestContentItemHandler_ListFeedContentItemsAppliesFeedPresetWithAdditionalMark(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	targetFeed := &model.Feed{FeedURL: "https://example.com/feed-detail.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	otherFeed := &model.Feed{FeedURL: "https://example.com/other-feed-detail.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, targetFeed); err != nil {
		t.Fatalf("create target feed: %v", err)
	}
	if err := feedRepo.Create(ctx, otherFeed); err != nil {
		t.Fatalf("create other feed: %v", err)
	}
	base := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	newer := base.Add(time.Hour)
	for _, item := range []*model.ContentItem{
		{FeedID: targetFeed.Id, DedupeKey: "target-favorite", Type: model.ContentItemTypeText, Title: "Target favorite", AvailableText: "Target favorite", Favorited: true, PublishedAt: &base},
		{FeedID: targetFeed.Id, DedupeKey: "target-plain", Type: model.ContentItemTypeText, Title: "Target plain", AvailableText: "Target plain", PublishedAt: &newer},
		{FeedID: otherFeed.Id, DedupeKey: "other-favorite", Type: model.ContentItemTypeText, Title: "Other favorite", AvailableText: "Other favorite", Favorited: true, PublishedAt: &newer},
	} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	obj := newHttpExcept(t, r).GET("/feeds/{id}/content-items", targetFeed.Id).
		WithQuery("mark", "favorite").
		WithQuery("page", "1").
		WithQuery("pageSize", "10").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	items := obj.Value("data").Object().Value("items").Array()
	items.Length().IsEqual(1)
	first := items.Value(0).Object()
	first.Value("title").IsEqual("Target favorite")
	first.Value("feedId").IsEqual(targetFeed.Id)
	first.Value("favorited").IsEqual(true)
}

func TestContentItemHandler_UpdateProcessingStatusTogglesWithoutChangingMarksOrProgress(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/status-update.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "status-audio", Type: model.ContentItemTypeAudio, Title: "Status audio", AvailableText: "Status audio", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true, Favorited: true, AudioProgressSeconds: 91}
	if err := contentRepo.Create(ctx, item); err != nil {
		t.Fatalf("create content item: %v", err)
	}

	obj := newHttpExcept(t, r).PUT("/content-items/{id}/processing-status", item.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"processingStatus": "completed"}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	data := obj.Value("data").Object()
	data.Value("id").IsEqual(item.Id)
	data.Value("processingStatus").IsEqual("completed")
	data.Value("markedLater").IsEqual(true)
	data.Value("favorited").IsEqual(true)
	data.Value("audioProgressSeconds").IsEqual(91)

	got, err := contentRepo.GetByID(ctx, item.Id)
	if err != nil {
		t.Fatalf("get content item: %v", err)
	}
	if got.ProcessingStatus != model.ContentItemProcessingStatusCompleted || !got.MarkedLater || !got.Favorited || got.AudioProgressSeconds != 91 {
		t.Fatalf("unexpected persisted item: %#v", got)
	}

	reverted := newHttpExcept(t, r).PUT("/content-items/{id}/processing-status", item.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"processingStatus": "unprocessed"}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Value("data").Object()
	reverted.Value("processingStatus").IsEqual("unprocessed")
	reverted.Value("markedLater").IsEqual(true)
	reverted.Value("favorited").IsEqual(true)
	reverted.Value("audioProgressSeconds").IsEqual(91)
	got, err = contentRepo.GetByID(ctx, item.Id)
	if err != nil {
		t.Fatalf("get reverted content item: %v", err)
	}
	if got.ProcessingStatus != model.ContentItemProcessingStatusUnprocessed || !got.MarkedLater || !got.Favorited || got.AudioProgressSeconds != 91 {
		t.Fatalf("unexpected reverted item: %#v", got)
	}
}

func TestContentItemHandler_UpdateMarkSetsAndClearsMarksIndependently(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/mark-update.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "mark-text", Type: model.ContentItemTypeText, Title: "Mark text", AvailableText: "Mark text", ProcessingStatus: model.ContentItemProcessingStatusCompleted, Favorited: true, AudioProgressSeconds: 42}
	if err := contentRepo.Create(ctx, item); err != nil {
		t.Fatalf("create content item: %v", err)
	}

	marked := newHttpExcept(t, r).PUT("/content-items/{id}/marks/{mark}", item.Id, "later").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]bool{"marked": true}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Value("data").Object()
	marked.Value("processingStatus").IsEqual("completed")
	marked.Value("markedLater").IsEqual(true)
	marked.Value("favorited").IsEqual(true)
	marked.Value("audioProgressSeconds").IsEqual(42)

	cleared := newHttpExcept(t, r).PUT("/content-items/{id}/marks/{mark}", item.Id, "favorite").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]bool{"marked": false}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Value("data").Object()
	cleared.Value("processingStatus").IsEqual("completed")
	cleared.Value("markedLater").IsEqual(true)
	cleared.Value("favorited").IsEqual(false)
	cleared.Value("audioProgressSeconds").IsEqual(42)

	got, err := contentRepo.GetByID(ctx, item.Id)
	if err != nil {
		t.Fatalf("get content item: %v", err)
	}
	if got.ProcessingStatus != model.ContentItemProcessingStatusCompleted || !got.MarkedLater || got.Favorited || got.AudioProgressSeconds != 42 {
		t.Fatalf("unexpected persisted item: %#v", got)
	}
}

func TestContentItemHandler_UpdateAudioProgressPersistsOnlyAudioWithoutChangingStatus(t *testing.T) {
	r, feedRepo, contentRepo := newContentViewTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/audio-progress.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	audio := &model.ContentItem{FeedID: feed.Id, DedupeKey: "audio-progress", Type: model.ContentItemTypeAudio, Title: "Audio progress", AvailableText: "Audio progress", ProcessingStatus: model.ContentItemProcessingStatusUnprocessed, MarkedLater: true}
	text := &model.ContentItem{FeedID: feed.Id, DedupeKey: "text-progress", Type: model.ContentItemTypeText, Title: "Text progress", AvailableText: "Text progress", ProcessingStatus: model.ContentItemProcessingStatusCompleted}
	if err := contentRepo.Create(ctx, audio); err != nil {
		t.Fatalf("create audio item: %v", err)
	}
	if err := contentRepo.Create(ctx, text); err != nil {
		t.Fatalf("create text item: %v", err)
	}

	updated := newHttpExcept(t, r).PUT("/content-items/{id}/audio-progress", audio.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]int{"audioProgressSeconds": 123}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Value("data").Object()
	updated.Value("processingStatus").IsEqual("unprocessed")
	updated.Value("markedLater").IsEqual(true)
	updated.Value("audioProgressSeconds").IsEqual(123)

	newHttpExcept(t, r).PUT("/content-items/{id}/audio-progress", text.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]int{"audioProgressSeconds": 321}).
		Expect().
		Status(http.StatusBadRequest)

	gotAudio, err := contentRepo.GetByID(ctx, audio.Id)
	if err != nil {
		t.Fatalf("get audio item: %v", err)
	}
	gotText, err := contentRepo.GetByID(ctx, text.Id)
	if err != nil {
		t.Fatalf("get text item: %v", err)
	}
	if gotAudio.AudioProgressSeconds != 123 || gotAudio.ProcessingStatus != model.ContentItemProcessingStatusUnprocessed {
		t.Fatalf("unexpected audio item: %#v", gotAudio)
	}
	if gotText.AudioProgressSeconds != 0 || gotText.ProcessingStatus != model.ContentItemProcessingStatusCompleted {
		t.Fatalf("unexpected text item: %#v", gotText)
	}
}

func newContentViewTestHarness(t *testing.T) (*gin.Engine, repository.FeedRepository, repository.ContentItemRepository) {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "content-view-handler.db") + "?_busy_timeout=5000"
	conf.Set("data.db.user.driver", "sqlite")
	conf.Set("data.db.user.dsn", dsn)
	conf.Set("data.db.user.debug", false)
	runContentViewHandlerMigrations(t, dsn)
	db := repository.NewDB(conf, logger)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sql db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sql db: %v", err)
		}
	})
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	contentHandler := handler.NewContentItemHandler(hdl, service.NewContentItemService(base, contentRepo, nil, nil))
	r := gin.New()
	r.GET("/content-items", contentHandler.ListContentItems)
	r.GET("/content-views/inbox", contentHandler.ListInboxContentItems)
	r.GET("/content-views/later", contentHandler.ListLaterContentItems)
	r.GET("/content-views/favorite", contentHandler.ListFavoriteContentItems)
	r.GET("/content-views/completed", contentHandler.ListCompletedContentItems)
	r.GET("/feeds/:id/content-items", contentHandler.ListFeedContentItems)
	r.PUT("/content-items/:id/processing-status", contentHandler.UpdateContentItemProcessingStatus)
	r.PUT("/content-items/:id/marks/:mark", contentHandler.UpdateContentItemMark)
	r.PUT("/content-items/:id/audio-progress", contentHandler.UpdateContentItemAudioProgress)
	return r, feedRepo, contentRepo
}

func TestRunContentViewHandlerMigrationsUsesRepositoryRoot(t *testing.T) {
	originalCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tempCwd := t.TempDir()
	t.Cleanup(func() {
		if err := os.Chdir(originalCwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(tempCwd); err != nil {
		t.Fatalf("change cwd: %v", err)
	}

	dsn := filepath.Join(t.TempDir(), "content-view-cwd.db") + "?_busy_timeout=5000"
	runContentViewHandlerMigrations(t, dsn)
}

func runContentViewHandlerMigrations(t *testing.T, dsn string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "migration-test.yml")
	content := fmt.Sprintf("data:\n  db:\n    user:\n      driver: sqlite\n      dsn: %q\n      debug: false\n", dsn)
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write migration config: %v", err)
	}
	cmd := exec.Command("go", "run", "./cmd/migration", "-conf", configPath, "-direction", "up", "-path", "migrations")
	cmd.Dir = contentViewHandlerRepoRoot(t)
	cmd.Env = append(os.Environ(), "APP_CONF="+configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run migrations: %v\n%s", err, output)
	}
}

func contentViewHandlerRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve content view handler test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func TestContentItemHandler_GetContentItemReturnsDetail(t *testing.T) {
	fetchedAt := time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC)
	contentService := &fakeContentItemService{
		detailResult: &v1.ContentItemDetailResponseData{
			Id:                   42,
			FeedID:               7,
			ContentType:          "text",
			Title:                "Detail item",
			DescriptionSafe:      "Safe description",
			ContentSafe:          "Safe content",
			ShowNotesSafe:        "Safe notes",
			AvailableText:        "Safe content",
			FetchedAt:            fetchedAt,
			ProcessingStatus:     "completed",
			MarkedLater:          true,
			Favorited:            true,
			AudioProgressSeconds: 15,
		},
	}
	contentHandler := handler.NewContentItemHandler(hdl, contentService)
	r := gin.New()
	r.GET("/content-items/:id", contentHandler.GetContentItem)

	obj := newHttpExcept(t, r).GET("/content-items/42").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	data := obj.Value("data").Object()
	data.Value("id").IsEqual(42)
	data.Value("feedId").IsEqual(7)
	data.Value("contentType").IsEqual("text")
	data.Value("title").IsEqual("Detail item")
	data.Value("descriptionSafe").IsEqual("Safe description")
	data.Value("contentSafe").IsEqual("Safe content")
	data.Value("showNotesSafe").IsEqual("Safe notes")
	data.Value("processingStatus").IsEqual("completed")
	data.Value("markedLater").IsEqual(true)
	data.Value("favorited").IsEqual(true)
	data.Value("audioProgressSeconds").IsEqual(15)

	if contentService.detailCalls != 1 || contentService.lastDetailID != 42 {
		t.Fatalf("expected GetContentItem(42), got calls=%d id=%d", contentService.detailCalls, contentService.lastDetailID)
	}
}

func TestContentItemHandler_GenerateAISummaryTriggersService(t *testing.T) {
	generatedAt := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	contentService := &fakeContentItemService{
		summaryResult: &v1.AISummaryResponseData{
			ContentItemID: 42,
			State:         "success",
			Markdown:      "## TL;DR\n自托管 摘要",
			GeneratedAt:   &generatedAt,
		},
	}
	contentHandler := handler.NewContentItemHandler(hdl, contentService)
	r := gin.New()
	r.POST("/content-items/:id/ai-summary", contentHandler.GenerateAISummary)

	obj := newHttpExcept(t, r).POST("/content-items/42/ai-summary").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	data := obj.Value("data").Object()
	data.Value("contentItemId").IsEqual(42)
	data.Value("state").IsEqual("success")
	data.Value("markdown").IsEqual("## TL;DR\n自托管 摘要")
	if contentService.summaryCalls != 1 || contentService.lastSummaryID != 42 {
		t.Fatalf("expected GenerateAISummary(42), got calls=%d id=%d", contentService.summaryCalls, contentService.lastSummaryID)
	}
}

func TestContentItemHandler_GenerateAISummaryMapsInProgressAsOK(t *testing.T) {
	contentService := &fakeContentItemService{
		summaryResult: &v1.AISummaryResponseData{ContentItemID: 42, State: "pending", Message: "正在生成"},
	}
	contentHandler := handler.NewContentItemHandler(hdl, contentService)
	r := gin.New()
	r.POST("/content-items/:id/ai-summary", contentHandler.GenerateAISummary)

	obj := newHttpExcept(t, r).POST("/content-items/42/ai-summary").
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	obj.Value("data").Object().Value("state").IsEqual("pending")
	obj.Value("data").Object().Value("message").IsEqual("正在生成")
}

func TestContentItemHandler_GenerateAISummaryMapsFailedStateAsOK(t *testing.T) {
	contentService := &fakeContentItemService{
		summaryResult: &v1.AISummaryResponseData{ContentItemID: 42, State: "failed", Error: "AI 摘要生成超时", Message: "AI 摘要生成超时"},
	}
	contentHandler := handler.NewContentItemHandler(hdl, contentService)
	r := gin.New()
	r.POST("/content-items/:id/ai-summary", contentHandler.GenerateAISummary)

	obj := newHttpExcept(t, r).POST("/content-items/42/ai-summary").
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	data := obj.Value("data").Object()
	data.Value("state").IsEqual("failed")
	data.Value("error").IsEqual("AI 摘要生成超时")
	data.Value("message").IsEqual("AI 摘要生成超时")
}

func TestContentItemHandler_GenerateAISummaryRealServiceStates(t *testing.T) {
	r, feedRepo, contentRepo, configRepo, chat := newContentAISummaryHandlerTestHarness(t)
	ctx := context.Background()
	if err := configRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}); err != nil {
		t.Fatalf("save AI config: %v", err)
	}
	feed := &model.Feed{FeedURL: "https://example.com/handler-ai-summary.xml", Title: "AI 周报", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	successItem := &model.ContentItem{FeedID: feed.Id, DedupeKey: "handler-success", Type: model.ContentItemTypeText, Title: "Success", AvailableText: strings.Repeat("足够的可用文本。", 20)}
	pendingItem := &model.ContentItem{FeedID: feed.Id, DedupeKey: "handler-pending", Type: model.ContentItemTypeText, Title: "Pending", AvailableText: strings.Repeat("足够的可用文本。", 20), AISummaryStatus: model.AISummaryStatusPending}
	shortItem := &model.ContentItem{FeedID: feed.Id, DedupeKey: "handler-short", Type: model.ContentItemTypeAudio, Title: "Short", AvailableText: "太短"}
	for _, item := range []*model.ContentItem{successItem, pendingItem, shortItem} {
		if err := contentRepo.Create(ctx, item); err != nil {
			t.Fatalf("create content item %s: %v", item.DedupeKey, err)
		}
	}

	success := newHttpExcept(t, r).POST("/content-items/{id}/ai-summary", successItem.Id).
		Expect().Status(http.StatusOK).JSON().Object()
	success.Value("data").Object().Value("state").IsEqual("success")
	success.Value("data").Object().Value("markdown").IsEqual("## TL;DR\nhandler 摘要")

	pending := newHttpExcept(t, r).POST("/content-items/{id}/ai-summary", pendingItem.Id).
		Expect().Status(http.StatusOK).JSON().Object()
	pending.Value("data").Object().Value("state").IsEqual("pending")
	pending.Value("data").Object().Value("message").IsEqual("正在生成")

	insufficient := newHttpExcept(t, r).POST("/content-items/{id}/ai-summary", shortItem.Id).
		Expect().Status(http.StatusOK).JSON().Object()
	insufficient.Value("data").Object().Value("state").IsEqual("insufficient_text")
	insufficient.Value("data").Object().Value("message").String().Contains("可用文本不足")
	if chat.calls != 1 {
		t.Fatalf("expected only success item to call chat completion, got %d", chat.calls)
	}
}

func newContentAISummaryHandlerTestHarness(t *testing.T) (*gin.Engine, repository.FeedRepository, repository.ContentItemRepository, repository.AIServiceConfigRepository, *handlerRecordingChatCompletion) {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "content-ai-summary-handler.db") + "?_busy_timeout=5000"
	conf.Set("data.db.user.driver", "sqlite")
	conf.Set("data.db.user.dsn", dsn)
	conf.Set("data.db.user.debug", false)
	runContentViewHandlerMigrations(t, dsn)
	db := repository.NewDB(conf, logger)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sql db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sql db: %v", err)
		}
	})
	repo := repository.NewRepository(logger, db)
	feedRepo := repository.NewFeedRepository(repo)
	contentRepo := repository.NewContentItemRepository(repo)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	chat := &handlerRecordingChatCompletion{content: "## TL;DR\nhandler 摘要"}
	contentHandler := handler.NewContentItemHandler(hdl, service.NewContentItemService(base, contentRepo, configRepo, chat))
	r := gin.New()
	r.POST("/content-items/:id/ai-summary", contentHandler.GenerateAISummary)
	return r, feedRepo, contentRepo, configRepo, chat
}

type handlerRecordingChatCompletion struct {
	content string
	calls   int
}

func (c *handlerRecordingChatCompletion) ChatCompletion(_ context.Context, _ model.AIServiceConfig, _ []service.ChatCompletionMessage) (string, error) {
	c.calls++
	return c.content, nil
}

type fakeContentItemService struct {
	listResult      *v1.ListContentItemsResponseData
	listErr         error
	listCalls       int
	lastListContext context.Context
	lastListRequest *v1.ListContentItemsRequest
	detailResult    *v1.ContentItemDetailResponseData
	detailErr       error
	detailCalls     int
	lastDetailID    uint
	statusResult    *v1.ContentItemDetailResponseData
	statusErr       error
	statusCalls     int
	lastStatusID    uint
	lastStatusReq   *v1.UpdateContentItemProcessingStatusRequest
	markResult      *v1.ContentItemDetailResponseData
	markErr         error
	markCalls       int
	lastMarkID      uint
	lastMark        model.ContentItemMark
	lastMarkReq     *v1.UpdateContentItemMarkRequest
	audioResult     *v1.ContentItemDetailResponseData
	audioErr        error
	audioCalls      int
	lastAudioID     uint
	lastAudioReq    *v1.UpdateContentItemAudioProgressRequest
	summaryResult   *v1.AISummaryResponseData
	summaryErr      error
	summaryCalls    int
	lastSummaryID   uint
}

func (s *fakeContentItemService) ListContentItems(ctx context.Context, req *v1.ListContentItemsRequest) (*v1.ListContentItemsResponseData, error) {
	s.listCalls++
	s.lastListContext = ctx
	s.lastListRequest = req
	return s.listResult, s.listErr
}

func (s *fakeContentItemService) UpdateProcessingStatus(_ context.Context, id uint, req *v1.UpdateContentItemProcessingStatusRequest) (*v1.ContentItemDetailResponseData, error) {
	s.statusCalls++
	s.lastStatusID = id
	s.lastStatusReq = req
	return s.statusResult, s.statusErr
}

func (s *fakeContentItemService) UpdateMark(_ context.Context, id uint, mark model.ContentItemMark, req *v1.UpdateContentItemMarkRequest) (*v1.ContentItemDetailResponseData, error) {
	s.markCalls++
	s.lastMarkID = id
	s.lastMark = mark
	s.lastMarkReq = req
	return s.markResult, s.markErr
}

func (s *fakeContentItemService) UpdateAudioProgress(_ context.Context, id uint, req *v1.UpdateContentItemAudioProgressRequest) (*v1.ContentItemDetailResponseData, error) {
	s.audioCalls++
	s.lastAudioID = id
	s.lastAudioReq = req
	return s.audioResult, s.audioErr
}

func (s *fakeContentItemService) GetContentItem(_ context.Context, id uint) (*v1.ContentItemDetailResponseData, error) {
	s.detailCalls++
	s.lastDetailID = id
	return s.detailResult, s.detailErr
}

func (s *fakeContentItemService) GenerateAISummary(_ context.Context, id uint) (*v1.AISummaryResponseData, error) {
	s.summaryCalls++
	s.lastSummaryID = id
	return s.summaryResult, s.summaryErr
}

func (s *fakeContentItemService) GenerateAutoAISummary(context.Context, uint) (*service.AutoAISummaryGenerationResult, error) {
	return nil, nil
}
