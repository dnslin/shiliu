package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	v1 "shiliu/api/v1"
	"shiliu/internal/handler"
)

func TestFeedHandler_CreateFeedReturnsCreatedFeed(t *testing.T) {
	params := v1.CreateFeedRequest{FeedURL: " https://EXAMPLE.com:443/podcast.xml#intro "}
	feedService := &fakeFeedService{
		addFeedResult: &v1.CreateFeedResponseData{
			Id:            42,
			FeedURL:       "https://example.com/podcast.xml",
			Type:          "podcast",
			FetchedItems:  2,
			InsertedItems: 2,
		},
	}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds", feedHandler.CreateFeed)

	obj := newHttpExcept(t, r).POST("/feeds").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
	data := obj.Value("data").Object()
	data.Value("id").IsEqual(42)
	data.Value("feedUrl").IsEqual("https://example.com/podcast.xml")
	data.Value("type").IsEqual("podcast")
	data.Value("fetchedItems").IsEqual(2)
	data.Value("insertedItems").IsEqual(2)

	if feedService.addFeedCalls != 1 {
		t.Fatalf("expected AddFeed to be called once, got %d", feedService.addFeedCalls)
	}
	if feedService.lastAddFeedRequest == nil || feedService.lastAddFeedRequest.FeedURL != params.FeedURL {
		t.Fatalf("handler passed request %#v, want feedUrl %q", feedService.lastAddFeedRequest, params.FeedURL)
	}
}

func TestFeedHandler_CreateFeedMapsParseFailure(t *testing.T) {
	params := v1.CreateFeedRequest{FeedURL: "https://example.com/not-feed.xml"}
	feedService := &fakeFeedService{addFeedErr: v1.ErrFeedParseFailed}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds", feedHandler.CreateFeed)

	obj := newHttpExcept(t, r).POST("/feeds").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusUnprocessableEntity).
		JSON().
		Object()
	obj.Value("code").IsEqual(2003)
	obj.Value("message").IsEqual("feed parse failed")
}

func TestFeedHandler_CreateFeedMapsDuplicateConflict(t *testing.T) {
	params := v1.CreateFeedRequest{FeedURL: "https://example.com/articles.xml"}
	feedService := &fakeFeedService{addFeedErr: v1.ErrFeedAlreadyExists}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds", feedHandler.CreateFeed)

	obj := newHttpExcept(t, r).POST("/feeds").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusConflict).
		JSON().
		Object()
	obj.Value("code").IsEqual(2004)
	obj.Value("message").IsEqual("feed already exists")
}

type fakeFeedService struct {
	addFeedCalls       int
	lastAddFeedRequest *v1.CreateFeedRequest
	addFeedResult      *v1.CreateFeedResponseData
	addFeedErr         error
}

func (f *fakeFeedService) CreateFeed(_ context.Context, req *v1.CreateFeedRequest) (*v1.CreateFeedResponseData, error) {
	f.addFeedCalls++
	f.lastAddFeedRequest = req
	return f.addFeedResult, f.addFeedErr
}
