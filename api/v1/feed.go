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
