package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"shiliu/internal/handler"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestFolderHandler_CRUDAssignClearDeleteAndFilter(t *testing.T) {
	r, feedRepo, contentRepo, folderRepo, db := newFolderHandlerTestHarness(t)
	ctx := context.Background()

	created := newHttpExcept(t, r).POST("/folders").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"name": "Engineering"}).
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	created.Value("code").IsEqual(0)
	folderID := uint(created.Value("data").Object().Value("id").Number().Raw())
	created.Value("data").Object().Value("name").IsEqual("Engineering")

	newHttpExcept(t, r).POST("/folders").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"name": "Engineering"}).
		Expect().
		Status(http.StatusConflict).
		JSON().Object().Value("code").IsEqual(4003)

	list := newHttpExcept(t, r).GET("/folders").
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	list.Value("data").Object().Value("total").IsEqual(1)
	list.Value("data").Object().Value("items").Array().Value(0).Object().Value("name").IsEqual("Engineering")

	renamed := newHttpExcept(t, r).PUT("/folders/{id}", folderID).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"name": "Research"}).
		Expect().
		Status(http.StatusOK).
		JSON().Object()
	renamed.Value("data").Object().Value("name").IsEqual("Research")

	targetFeed := &model.Feed{FeedURL: "https://example.com/folder-target.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	otherFeed := &model.Feed{FeedURL: "https://example.com/folder-other.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, targetFeed); err != nil {
		t.Fatalf("create target feed: %v", err)
	}
	if err := feedRepo.Create(ctx, otherFeed); err != nil {
		t.Fatalf("create other feed: %v", err)
	}
	targetItem := &model.ContentItem{FeedID: targetFeed.Id, DedupeKey: "target", Type: model.ContentItemTypeText, Title: "Target", AvailableText: "Target"}
	otherItem := &model.ContentItem{FeedID: otherFeed.Id, DedupeKey: "other", Type: model.ContentItemTypeText, Title: "Other", AvailableText: "Other"}
	if err := contentRepo.Create(ctx, targetItem); err != nil {
		t.Fatalf("create target item: %v", err)
	}
	if err := contentRepo.Create(ctx, otherItem); err != nil {
		t.Fatalf("create other item: %v", err)
	}

	newHttpExcept(t, r).PUT("/feeds/{id}/folder", targetFeed.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]uint{"folderId": folderID}).
		Expect().
		Status(http.StatusOK).
		JSON().Object().Value("code").IsEqual(0)

	filtered := newHttpExcept(t, r).GET("/content-items").
		WithQuery("folder_id", folderID).
		Expect().
		Status(http.StatusOK).
		JSON().Object().Value("data").Object()
	filtered.Value("page").Object().Value("total").IsEqual(1)
	filtered.Value("items").Array().Value(0).Object().Value("id").IsEqual(targetItem.Id)

	newHttpExcept(t, r).PUT("/feeds/{id}/folder", targetFeed.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]*uint{"folderId": nil}).
		Expect().
		Status(http.StatusOK).
		JSON().Object().Value("code").IsEqual(0)
	clearedFeed, err := feedRepo.GetByID(ctx, targetFeed.Id)
	if err != nil {
		t.Fatalf("load cleared feed: %v", err)
	}
	if clearedFeed.FolderID != nil {
		t.Fatalf("expected cleared folder id, got %v", *clearedFeed.FolderID)
	}

	newHttpExcept(t, r).PUT("/feeds/{id}/folder", targetFeed.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]uint{"folderId": folderID}).
		Expect().
		Status(http.StatusOK)
	newHttpExcept(t, r).DELETE("/folders/{id}", folderID).
		Expect().
		Status(http.StatusOK).
		JSON().Object().Value("code").IsEqual(0)

	deletedFeed, err := feedRepo.GetByID(ctx, targetFeed.Id)
	if err != nil {
		t.Fatalf("load feed after folder delete: %v", err)
	}
	if deletedFeed.FolderID != nil {
		t.Fatalf("expected folder delete to clear feed folder id, got %v", *deletedFeed.FolderID)
	}
	if _, err := contentRepo.GetByID(ctx, targetItem.Id); err != nil {
		t.Fatalf("content item should remain after folder delete: %v", err)
	}
	var residualFeeds int64
	if err := db.Model(&model.Feed{}).Where("folder_id = ?", folderID).Count(&residualFeeds).Error; err != nil {
		t.Fatalf("count residual folder refs: %v", err)
	}
	if residualFeeds != 0 {
		t.Fatalf("expected no residual folder refs, got %d", residualFeeds)
	}
	folders, err := folderRepo.List(ctx)
	if err != nil || len(folders) != 0 {
		t.Fatalf("expected empty folder list after delete, folders=%#v err=%v", folders, err)
	}
}

func TestFolderHandler_AssignRejectsMissingFolder(t *testing.T) {
	r, feedRepo, _, _, _ := newFolderHandlerTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/missing-folder.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	newHttpExcept(t, r).PUT("/feeds/{id}/folder", feed.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]uint{"folderId": 999999}).
		Expect().
		Status(http.StatusNotFound).
		JSON().Object().Value("code").IsEqual(4004)
}

func newFolderHandlerTestHarness(t *testing.T) (*gin.Engine, repository.FeedRepository, repository.ContentItemRepository, repository.FolderRepository, *gorm.DB) {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "folder-handler.db") + "?_busy_timeout=5000"
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
	folderRepo := repository.NewFolderRepository(repo)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	contentHandler := handler.NewContentItemHandler(hdl, service.NewContentItemService(base, contentRepo, nil, nil))
	folderHandler := handler.NewFolderHandler(hdl, service.NewFolderService(base, folderRepo, feedRepo))
	r := gin.New()
	r.GET("/content-items", contentHandler.ListContentItems)
	r.GET("/folders", folderHandler.ListFolders)
	r.POST("/folders", folderHandler.CreateFolder)
	r.PUT("/folders/:id", folderHandler.RenameFolder)
	r.DELETE("/folders/:id", folderHandler.DeleteFolder)
	r.PUT("/feeds/:id/folder", folderHandler.AssignFeedFolder)
	return r, feedRepo, contentRepo, folderRepo, db
}
