package v1

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
