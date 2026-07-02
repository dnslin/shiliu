package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"shiliu/pkg/log"
)

func TestRequestLogMiddlewareRedactsPasswordFields(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := &log.Logger{Logger: zap.New(core)}

	r := gin.New()
	r.Use(RequestLogMiddleware(logger))
	r.POST("/user/password", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	requestBody := []byte(`{"oldPassword":"old-password-12","newPassword":"new-password-12","username":"testuser"}`)
	request := httptest.NewRequest(http.MethodPost, "/user/password", bytes.NewReader(requestBody))
	response := httptest.NewRecorder()

	r.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	entries := logs.FilterMessage("Request").All()
	require.Len(t, entries, 1)
	params, ok := entries[0].ContextMap()["request_params"].(string)
	require.True(t, ok)
	assert.NotContains(t, params, "old-password-12")
	assert.NotContains(t, params, "new-password-12")
	assert.Contains(t, params, `"oldPassword":"[REDACTED]"`)
	assert.Contains(t, params, `"newPassword":"[REDACTED]"`)
	assert.Contains(t, params, `"username":"testuser"`)
}

func TestRequestLogMiddlewareRedactsAPIKeyFields(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := &log.Logger{Logger: zap.New(core)}

	r := gin.New()
	r.Use(RequestLogMiddleware(logger))
	r.PUT("/ai/service-config", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	requestBody := []byte(`{"apiBaseUrl":"https://api.example.com/v1","model":"gpt-4.1-mini","apiKey":"sk-secret-value"}`)
	request := httptest.NewRequest(http.MethodPut, "/ai/service-config", bytes.NewReader(requestBody))
	response := httptest.NewRecorder()

	r.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	entries := logs.FilterMessage("Request").All()
	require.Len(t, entries, 1)
	params, ok := entries[0].ContextMap()["request_params"].(string)
	require.True(t, ok)
	assert.NotContains(t, params, "sk-secret-value")
	assert.Contains(t, params, `"apiKey":"[REDACTED]"`)
	assert.Contains(t, params, `"model":"gpt-4.1-mini"`)
}

func TestRequestLogMiddlewareOmitsOPMLImportBody(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := &log.Logger{Logger: zap.New(core)}

	r := gin.New()
	r.Use(RequestLogMiddleware(logger))
	r.POST("/api/v1/feeds/import-opml", func(ctx *gin.Context) {
		body, err := io.ReadAll(ctx.Request.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "https://private.example.com/feed.xml")
		ctx.Status(http.StatusNoContent)
	})

	requestBody := []byte(`{"opml":"<opml><body><outline xmlUrl=\"https://private.example.com/feed.xml\"/></body></opml>"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/feeds/import-opml", bytes.NewReader(requestBody))
	response := httptest.NewRecorder()

	r.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	entries := logs.FilterMessage("Request").All()
	require.Len(t, entries, 1)
	params, ok := entries[0].ContextMap()["request_params"].(string)
	require.True(t, ok)
	assert.Equal(t, "[OMITTED: opml import body]", params)
	assert.NotContains(t, params, "private.example.com")
}
