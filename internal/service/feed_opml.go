package service

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	v1 "shiliu/api/v1"
)

func (s *feedService) ImportOPML(ctx context.Context, req *v1.ImportOPMLRequest) (*v1.ImportOPMLResponseData, error) {
	if req == nil || strings.TrimSpace(req.OPML) == "" {
		return nil, v1.ErrOPMLInvalid
	}
	candidates, err := parseOPMLFeedURLs(req.OPML)
	if err != nil {
		return nil, err
	}
	result := &v1.ImportOPMLResponseData{Total: len(candidates)}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		feedURL, err := NormalizeFeedURL(candidate)
		if err != nil {
			result.Failed++
			continue
		}
		if _, ok := seen[feedURL]; ok {
			result.Duplicate++
			continue
		}
		seen[feedURL] = struct{}{}
		_, err = s.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: feedURL})
		switch {
		case err == nil:
			result.Success++
		case errors.Is(err, v1.ErrFeedAlreadyExists):
			result.Duplicate++
		case errors.Is(err, v1.ErrFeedInvalidURL), errors.Is(err, v1.ErrFeedFetchFailed), errors.Is(err, v1.ErrFeedParseFailed):
			result.Failed++
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, err
		default:
			return nil, fmt.Errorf("%w: %v", v1.ErrOPMLImportFailed, err)
		}
	}
	return result, nil
}

func parseOPMLFeedURLs(raw string) ([]string, error) {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	var urls []string
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("%w: %v", v1.ErrOPMLInvalid, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "outline" {
			continue
		}
		if feedURL, ok := outlineFeedURL(start.Attr); ok {
			urls = append(urls, feedURL)
		}
	}
	if len(urls) == 0 {
		return nil, v1.ErrOPMLInvalid
	}
	return urls, nil
}

func outlineFeedURL(attrs []xml.Attr) (string, bool) {
	for _, name := range [...]string{"xmlUrl", "xmlURL", "url"} {
		for _, attr := range attrs {
			if attr.Name.Local == name {
				return attr.Value, true
			}
		}
	}
	return "", false
}
