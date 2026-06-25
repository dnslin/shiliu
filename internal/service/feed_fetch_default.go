package service

import (
	"net/http"
	"time"
)

const defaultFeedFetchTimeout = 15 * time.Second

func NewDefaultFetcher() Fetcher {
	return NewHTTPFetcher(&http.Client{
		Timeout:   defaultFeedFetchTimeout,
		Transport: newDefaultFeedTransport(),
	})
}

func newDefaultFeedTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return transport
}
