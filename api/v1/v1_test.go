package v1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParsePageRequestDefaults(t *testing.T) {
	ctx := newTestContext("/items")

	page := ParsePageRequest(ctx)
	limit, offset := page.LimitOffset()

	require.Equal(t, 1, page.Page)
	require.Equal(t, 20, page.PageSize)
	require.Equal(t, 20, limit)
	require.Equal(t, 0, offset)
}

func TestParsePageRequestBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		target       string
		wantPage     int
		wantPageSize int
		wantLimit    int
		wantOffset   int
	}{
		{name: "page starts at one", target: "/items?page=1&pageSize=20", wantPage: 1, wantPageSize: 20, wantLimit: 20, wantOffset: 0},
		{name: "page below one normalizes to one", target: "/items?page=0&pageSize=20", wantPage: 1, wantPageSize: 20, wantLimit: 20, wantOffset: 0},
		{name: "page size below one uses default", target: "/items?page=2&pageSize=0", wantPage: 2, wantPageSize: 20, wantLimit: 20, wantOffset: 20},
		{name: "page size above max is capped", target: "/items?page=3&pageSize=101", wantPage: 3, wantPageSize: 100, wantLimit: 100, wantOffset: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.target)

			page := ParsePageRequest(ctx)
			limit, offset := page.LimitOffset()

			require.Equal(t, tt.wantPage, page.Page)
			require.Equal(t, tt.wantPageSize, page.PageSize)
			require.Equal(t, tt.wantLimit, limit)
			require.Equal(t, tt.wantOffset, offset)
		})
	}
}

func TestParsePageRequestMalformedFieldsFallbackIndependently(t *testing.T) {
	ctx := newTestContext("/items?page=bad&pageSize=bad")

	page := ParsePageRequest(ctx)
	limit, offset := page.LimitOffset()

	require.Equal(t, 1, page.Page)
	require.Equal(t, 20, page.PageSize)
	require.Equal(t, 20, limit)
	require.Equal(t, 0, offset)
}

func TestNewPageData(t *testing.T) {
	items := []string{"a", "b"}
	page := PageRequest{Page: 2, PageSize: 10}

	data := NewPageData(items, page, 25)

	require.Equal(t, items, data.Items)
	require.Equal(t, PageMeta{Page: 2, PageSize: 10, Total: 25}, data.Page)
}

func TestNewPageDataKeepsMetadataWhenTotalIsZero(t *testing.T) {
	items := []string{}
	page := PageRequest{Page: 1, PageSize: 20}

	data := NewPageData(items, page, 0)

	require.Equal(t, items, data.Items)
	require.Equal(t, PageMeta{Page: 1, PageSize: 20, Total: 0}, data.Page)
}

func TestHandleSuccessNilDataReturnsEnvelope(t *testing.T) {
	ctx, recorder := newResponseTestContext()

	HandleSuccess(ctx, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"code":0,"message":"ok","data":{}}`, recorder.Body.String())
}

func TestHandleErrorRegisteredErrorReturnsBusinessCodeAndHTTPStatus(t *testing.T) {
	ctx, recorder := newResponseTestContext()

	HandleError(ctx, http.StatusBadRequest, ErrFeedInvalidURL, nil)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"code":2001,"message":"invalid feed url","data":{}}`, recorder.Body.String())
}

func TestHandleErrorUnknownErrorReturnsUnknownEnvelopeAndHTTPStatus(t *testing.T) {
	ctx, recorder := newResponseTestContext()

	HandleError(ctx, http.StatusTeapot, errors.New("not registered"), nil)

	require.Equal(t, http.StatusTeapot, recorder.Code)
	require.JSONEq(t, `{"code":500,"message":"unknown error","data":{}}`, recorder.Body.String())
}

func newTestContext(target string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Request = req
	return ctx
}

func newResponseTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return ctx, recorder
}
