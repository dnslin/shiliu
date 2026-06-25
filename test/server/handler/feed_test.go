package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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

func TestFeedHandler_CreateFeedPassesHTTPRequestContext(t *testing.T) {
	params := []byte(`{"feedUrl":"https://example.com/articles.xml"}`)
	feedService := &fakeFeedService{addFeedResult: &v1.CreateFeedResponseData{Id: 1, FeedURL: "https://example.com/articles.xml", Type: "rss"}}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds", feedHandler.CreateFeed)

	type requestContextKey struct{}
	baseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestCtx := context.WithValue(baseCtx, requestContextKey{}, "request-context")
	req := httptest.NewRequest(http.MethodPost, "/feeds", bytes.NewReader(params)).WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
	if _, ok := feedService.lastAddFeedContext.(*gin.Context); ok {
		t.Fatalf("handler passed gin.Context; want the underlying HTTP request context")
	}
	if got := feedService.lastAddFeedContext.Value(requestContextKey{}); got != "request-context" {
		t.Fatalf("handler passed context value %v, want request-context", got)
	}
	if feedService.lastAddFeedContext.Done() == nil {
		t.Fatalf("handler passed a context without cancellation")
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

func TestFeedHandler_RefreshFeedsReturnsSummary(t *testing.T) {
	feedService := &fakeFeedService{
		refreshFeedsResult: &v1.RefreshFeedsResponseData{
			Total:     2,
			Refreshed: 1,
			Skipped:   1,
			Items: []v1.RefreshFeedResponseData{
				{FeedID: 1, FeedURL: "https://example.com/a.xml", Status: "success", FetchedItems: 2, InsertedItems: 1, SkippedExistingItems: 1},
				{FeedID: 2, FeedURL: "https://example.com/b.xml", Status: "skipped", Message: "feed fetch already in progress; skipped"},
			},
		},
	}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds/refresh", feedHandler.RefreshFeeds)

	obj := newHttpExcept(t, r).POST("/feeds/refresh").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	data := obj.Value("data").Object()
	data.Value("total").IsEqual(2)
	data.Value("refreshed").IsEqual(1)
	data.Value("skipped").IsEqual(1)
	data.Value("failed").IsEqual(0)
	items := data.Value("items").Array()
	items.Length().IsEqual(2)
	items.Value(1).Object().Value("status").IsEqual("skipped")
	items.Value(1).Object().Value("message").String().Contains("already in progress")

	if feedService.refreshFeedsCalls != 1 {
		t.Fatalf("expected RefreshFeeds to be called once, got %d", feedService.refreshFeedsCalls)
	}
}

func TestFeedHandler_RefreshFeedReturnsSkippedResult(t *testing.T) {
	feedService := &fakeFeedService{
		refreshFeedResult: &v1.RefreshFeedResponseData{
			FeedID:  42,
			FeedURL: "https://example.com/podcast.xml",
			Status:  "skipped",
			Message: "feed fetch already in progress; skipped",
		},
	}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds/:id/refresh", feedHandler.RefreshFeed)

	obj := newHttpExcept(t, r).POST("/feeds/42/refresh").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	data := obj.Value("data").Object()
	data.Value("feedId").IsEqual(42)
	data.Value("feedUrl").IsEqual("https://example.com/podcast.xml")
	data.Value("status").IsEqual("skipped")
	data.Value("message").String().Contains("already in progress")

	if feedService.refreshFeedCalls != 1 || feedService.lastRefreshFeedID != 42 {
		t.Fatalf("expected RefreshFeed(42), got calls=%d id=%d", feedService.refreshFeedCalls, feedService.lastRefreshFeedID)
	}
}

func TestFeedHandler_RefreshFeedRejectsIDAboveSQLiteRange(t *testing.T) {
	feedService := &fakeFeedService{}
	feedHandler := handler.NewFeedHandler(hdl, feedService)
	r := gin.New()
	r.POST("/feeds/:id/refresh", feedHandler.RefreshFeed)

	obj := newHttpExcept(t, r).POST("/feeds/9223372036854775808/refresh").
		Expect().
		Status(http.StatusBadRequest).
		JSON().
		Object()
	obj.Value("code").IsEqual(400)
	obj.Value("message").IsEqual("Bad Request")

	if feedService.refreshFeedCalls != 0 {
		t.Fatalf("expected RefreshFeed not to be called, got %d calls", feedService.refreshFeedCalls)
	}
}

type fakeFeedService struct {
	addFeedCalls       int
	lastAddFeedContext context.Context
	lastAddFeedRequest *v1.CreateFeedRequest
	addFeedResult      *v1.CreateFeedResponseData
	addFeedErr         error

	refreshFeedsCalls  int
	refreshFeedsResult *v1.RefreshFeedsResponseData
	refreshFeedsErr    error

	refreshFeedCalls  int
	lastRefreshFeedID uint
	refreshFeedResult *v1.RefreshFeedResponseData
	refreshFeedErr    error
}

func (f *fakeFeedService) CreateFeed(ctx context.Context, req *v1.CreateFeedRequest) (*v1.CreateFeedResponseData, error) {
	f.addFeedCalls++
	f.lastAddFeedContext = ctx
	f.lastAddFeedRequest = req
	return f.addFeedResult, f.addFeedErr
}

func (f *fakeFeedService) RefreshFeeds(ctx context.Context) (*v1.RefreshFeedsResponseData, error) {
	f.refreshFeedsCalls++
	f.lastAddFeedContext = ctx
	return f.refreshFeedsResult, f.refreshFeedsErr
}

func (f *fakeFeedService) RefreshFeed(ctx context.Context, feedID uint) (*v1.RefreshFeedResponseData, error) {
	f.refreshFeedCalls++
	f.lastAddFeedContext = ctx
	f.lastRefreshFeedID = feedID
	return f.refreshFeedResult, f.refreshFeedErr
}
