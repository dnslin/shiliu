package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	if contentService.lastListRequest.ContentType != "text" || contentService.lastListRequest.ProcessingStatus != "unprocessed" || contentService.lastListRequest.Mark != "later" || contentService.lastListRequest.FeedID != "7" {
		t.Fatalf("handler passed request %#v", contentService.lastListRequest)
	}
	if contentService.lastListRequest.Page.Page != 2 || contentService.lastListRequest.Page.PageSize != 1 {
		t.Fatalf("handler passed page %#v", contentService.lastListRequest.Page)
	}
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
	contentHandler := handler.NewContentItemHandler(hdl, service.NewContentItemService(base, contentRepo))
	r := gin.New()
	r.GET("/content-views/inbox", contentHandler.ListInboxContentItems)
	r.GET("/content-views/later", contentHandler.ListLaterContentItems)
	r.GET("/content-views/favorite", contentHandler.ListFavoriteContentItems)
	r.GET("/content-views/completed", contentHandler.ListCompletedContentItems)
	r.GET("/feeds/:id/content-items", contentHandler.ListFeedContentItems)
	return r, feedRepo, contentRepo
}

func runContentViewHandlerMigrations(t *testing.T, dsn string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "migration-test.yml")
	content := fmt.Sprintf("data:\n  db:\n    user:\n      driver: sqlite\n      dsn: %q\n      debug: false\n", dsn)
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write migration config: %v", err)
	}
	cmd := exec.Command("go", "run", "./cmd/migration", "-conf", configPath, "-direction", "up", "-path", "migrations")
	cmd.Dir = filepath.Join("..", "..", "..")
	cmd.Env = append(os.Environ(), "APP_CONF="+configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run migrations: %v\n%s", err, output)
	}
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
}

func (s *fakeContentItemService) ListContentItems(ctx context.Context, req *v1.ListContentItemsRequest) (*v1.ListContentItemsResponseData, error) {
	s.listCalls++
	s.lastListContext = ctx
	s.lastListRequest = req
	return s.listResult, s.listErr
}

func (s *fakeContentItemService) GetContentItem(_ context.Context, id uint) (*v1.ContentItemDetailResponseData, error) {
	s.detailCalls++
	s.lastDetailID = id
	return s.detailResult, s.detailErr
}
