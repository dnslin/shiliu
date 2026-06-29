package v1

import "time"

type ListContentItemsRequest struct {
	ContentType      string      `form:"content_type" json:"contentType"`
	ProcessingStatus string      `form:"processing_status" json:"processingStatus"`
	Mark             string      `form:"mark" json:"mark"`
	FeedID           string      `form:"feed_id" json:"feedId"`
	TagID            string      `form:"tag_id" json:"tagId"`
	FolderID         string      `form:"folder_id" json:"folderId"`
	Keyword          string      `form:"keyword" json:"keyword"`
	Page             PageRequest `json:"page"`
}

type UpdateContentItemProcessingStatusRequest struct {
	ProcessingStatus string `json:"processingStatus" binding:"required"`
}

type UpdateContentItemMarkRequest struct {
	Marked *bool `json:"marked" binding:"required"`
}

type UpdateContentItemAudioProgressRequest struct {
	AudioProgressSeconds *int `json:"audioProgressSeconds" binding:"required"`
}

type ContentItemListItemData struct {
	Id                   uint       `json:"id"`
	FeedID               uint       `json:"feedId"`
	ContentType          string     `json:"contentType" example:"text"`
	Title                string     `json:"title"`
	AvailableText        string     `json:"availableText"`
	PublishedAt          *time.Time `json:"publishedAt"`
	FetchedAt            time.Time  `json:"fetchedAt"`
	ProcessingStatus     string     `json:"processingStatus" example:"unprocessed"`
	MarkedLater          bool       `json:"markedLater"`
	Favorited            bool       `json:"favorited"`
	AudioProgressSeconds int        `json:"audioProgressSeconds"`
}

type ContentItemDetailResponseData struct {
	Id                   uint       `json:"id"`
	FeedID               uint       `json:"feedId"`
	ContentType          string     `json:"contentType" example:"text"`
	Title                string     `json:"title"`
	DescriptionSafe      string     `json:"descriptionSafe"`
	ContentSafe          string     `json:"contentSafe"`
	ShowNotesSafe        string     `json:"showNotesSafe"`
	AvailableText        string     `json:"availableText"`
	PublishedAt          *time.Time `json:"publishedAt"`
	FetchedAt            time.Time  `json:"fetchedAt"`
	ProcessingStatus     string     `json:"processingStatus" example:"unprocessed"`
	MarkedLater          bool       `json:"markedLater"`
	Favorited            bool       `json:"favorited"`
	AudioProgressSeconds int        `json:"audioProgressSeconds"`
}

type GetContentItemResponse struct {
	Response
	Data ContentItemDetailResponseData `json:"data"`
}

type ListContentItemsResponseData struct {
	Items []ContentItemListItemData `json:"items"`
	Page  PageMeta                  `json:"page"`
}

type ListContentItemsResponse struct {
	Response
	Data ListContentItemsResponseData `json:"data"`
}
