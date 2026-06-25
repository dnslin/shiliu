package v1

import "time"

type CreateFeedRequest struct {
	FeedURL string `json:"feedUrl" binding:"required" example:"https://example.com/feed.xml"`
}

type CreateFeedResponseData struct {
	Id            uint   `json:"id"`
	FeedURL       string `json:"feedUrl"`
	Type          string `json:"type" example:"rss"`
	FetchedItems  int    `json:"fetchedItems"`
	InsertedItems int    `json:"insertedItems"`
}

type CreateFeedResponse struct {
	Response
	Data CreateFeedResponseData `json:"data"`
}

type FeedResponseData struct {
	Id             uint       `json:"id"`
	FeedURL        string     `json:"feedUrl"`
	Type           string     `json:"type" example:"rss"`
	FetchStatus    string     `json:"fetchStatus" example:"success"`
	LastFetchedAt  *time.Time `json:"lastFetchedAt"`
	LastFetchError *string    `json:"lastFetchError"`
	FolderID       *uint      `json:"folderId"`
}

type ListFeedsResponseData struct {
	Total int                `json:"total"`
	Items []FeedResponseData `json:"items"`
}

type ListFeedsResponse struct {
	Response
	Data ListFeedsResponseData `json:"data"`
}

type RefreshFeedResponseData struct {
	FeedID               uint   `json:"feedId"`
	FeedURL              string `json:"feedUrl"`
	Status               string `json:"status" example:"success"`
	Message              string `json:"message,omitempty"`
	FetchedItems         int    `json:"fetchedItems"`
	InsertedItems        int    `json:"insertedItems"`
	SkippedExistingItems int    `json:"skippedExistingItems"`
}

type RefreshFeedResponse struct {
	Response
	Data RefreshFeedResponseData `json:"data"`
}

type RefreshFeedsResponseData struct {
	Total     int                       `json:"total"`
	Refreshed int                       `json:"refreshed"`
	Skipped   int                       `json:"skipped"`
	Failed    int                       `json:"failed"`
	Items     []RefreshFeedResponseData `json:"items"`
}

type RefreshFeedsResponse struct {
	Response
	Data RefreshFeedsResponseData `json:"data"`
}
