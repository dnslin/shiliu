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

func TestTagHandler_CRUDLifecycleAndConflict(t *testing.T) {
	r, _, _, tagRepo, _ := newTagHandlerTestHarness(t)

	created := newHttpExcept(t, r).POST("/tags").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"name": "sqlite"}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	created.Value("code").IsEqual(0)
	created.Value("data").Object().Value("name").IsEqual("sqlite")

	newHttpExcept(t, r).POST("/tags").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"name": "sqlite"}).
		Expect().
		Status(http.StatusConflict).
		JSON().
		Object().
		Value("code").IsEqual(4001)

	tags, err := tagRepo.List(context.Background())
	if err != nil || len(tags) != 1 {
		t.Fatalf("load created tag: tags=%#v err=%v", tags, err)
	}
	tagID := tags[0].Id

	list := newHttpExcept(t, r).GET("/tags").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	list.Value("data").Object().Value("total").IsEqual(1)
	list.Value("data").Object().Value("items").Array().Value(0).Object().Value("name").IsEqual("sqlite")

	renamed := newHttpExcept(t, r).PUT("/tags/{id}", tagID).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{"name": "postgres"}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	renamed.Value("data").Object().Value("name").IsEqual("postgres")

	newHttpExcept(t, r).DELETE("/tags/{id}", tagID).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().
		Value("code").IsEqual(0)

	empty := newHttpExcept(t, r).GET("/tags").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	empty.Value("data").Object().Value("total").IsEqual(0)
}

func TestTagHandler_AssignsRemovesFiltersAndDeletesWithoutDeletingContentItem(t *testing.T) {
	r, feedRepo, contentRepo, tagRepo, db := newTagHandlerTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/tag-handler.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "tag-handler-item", Type: model.ContentItemTypeText, Title: "Tagged handler item", AvailableText: "Tagged handler item"}
	if err := contentRepo.Create(ctx, item); err != nil {
		t.Fatalf("create content item: %v", err)
	}
	sqliteTag := &model.Tag{Name: "sqlite"}
	goTag := &model.Tag{Name: "go"}
	if err := tagRepo.Create(ctx, sqliteTag); err != nil {
		t.Fatalf("create sqlite tag: %v", err)
	}
	if err := tagRepo.Create(ctx, goTag); err != nil {
		t.Fatalf("create go tag: %v", err)
	}

	newHttpExcept(t, r).PUT("/content-items/{id}/tags", item.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string][]uint{"tagIds": {sqliteTag.Id, goTag.Id}}).
		Expect().
		Status(http.StatusOK)

	sqliteFiltered := newHttpExcept(t, r).GET("/content-items").
		WithQuery("tag_id", sqliteTag.Id).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	sqliteFiltered.Value("data").Object().Value("items").Array().Length().IsEqual(1)
	sqliteFiltered.Value("data").Object().Value("items").Array().Value(0).Object().Value("id").IsEqual(item.Id)

	newHttpExcept(t, r).DELETE("/content-items/{id}/tags", item.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string][]uint{"tagIds": {sqliteTag.Id}}).
		Expect().
		Status(http.StatusOK)

	newHttpExcept(t, r).GET("/content-items").
		WithQuery("tag_id", sqliteTag.Id).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().
		Value("data").Object().Value("items").Array().Length().IsEqual(0)

	newHttpExcept(t, r).GET("/content-items").
		WithQuery("tag_id", goTag.Id).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().
		Value("data").Object().Value("items").Array().Length().IsEqual(1)

	newHttpExcept(t, r).DELETE("/tags/{id}", goTag.Id).
		Expect().
		Status(http.StatusOK)
	if _, err := contentRepo.GetByID(ctx, item.Id); err != nil {
		t.Fatalf("content item must remain after deleting tag: %v", err)
	}
	var relationCount int64
	if err := db.Table("content_item_tags").Where("content_item_id = ?", item.Id).Count(&relationCount).Error; err != nil {
		t.Fatalf("count tag relations: %v", err)
	}
	if relationCount != 0 {
		t.Fatalf("expected deleted tag relations to be cleared, got %d", relationCount)
	}
}

func TestTagHandler_AssignRejectsMissingTagID(t *testing.T) {
	r, feedRepo, contentRepo, _, _ := newTagHandlerTestHarness(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/tag-missing.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	if err := feedRepo.Create(ctx, feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "missing-tag-item", Type: model.ContentItemTypeText, Title: "Missing tag item", AvailableText: "Missing tag item"}
	if err := contentRepo.Create(ctx, item); err != nil {
		t.Fatalf("create content item: %v", err)
	}

	obj := newHttpExcept(t, r).PUT("/content-items/{id}/tags", item.Id).
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string][]uint{"tagIds": {999999}}).
		Expect().
		Status(http.StatusNotFound).
		JSON().
		Object()
	obj.Value("code").IsEqual(4002)
}

func newTagHandlerTestHarness(t *testing.T) (*gin.Engine, repository.FeedRepository, repository.ContentItemRepository, repository.TagRepository, *gorm.DB) {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "tag-handler.db") + "?_busy_timeout=5000"
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
	tagRepo := repository.NewTagRepository(repo)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	contentHandler := handler.NewContentItemHandler(hdl, service.NewContentItemService(base, contentRepo))
	tagHandler := handler.NewTagHandler(hdl, service.NewTagService(base, tagRepo, contentRepo))
	r := gin.New()
	r.GET("/content-items", contentHandler.ListContentItems)
	r.GET("/tags", tagHandler.ListTags)
	r.POST("/tags", tagHandler.CreateTag)
	r.PUT("/tags/:id", tagHandler.RenameTag)
	r.DELETE("/tags/:id", tagHandler.DeleteTag)
	r.PUT("/content-items/:id/tags", tagHandler.AssignContentItemTags)
	r.DELETE("/content-items/:id/tags", tagHandler.RemoveContentItemTags)
	return r, feedRepo, contentRepo, tagRepo, db
}
