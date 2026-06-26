package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	v1 "shiliu/api/v1"
	"shiliu/internal/handler"
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
