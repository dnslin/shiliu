package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultFetcherUsesDirectTimedClient(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	fetcher := NewDefaultFetcher()
	httpFetcher, ok := fetcher.(*HTTPFetcher)
	require.True(t, ok)
	require.NotNil(t, httpFetcher.client)
	assert.Greater(t, httpFetcher.client.Timeout, time.Duration(0))

	transport, ok := httpFetcher.client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, transport.Proxy)
	assert.NotNil(t, transport.DialContext)
}
