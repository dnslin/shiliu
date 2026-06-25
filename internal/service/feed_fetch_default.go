package service

import "net/http"

func NewDefaultFetcher() Fetcher {
	return NewHTTPFetcher(http.DefaultClient)
}
