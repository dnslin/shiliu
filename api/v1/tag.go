package v1

type CreateTagRequest struct {
	Name string `json:"name" binding:"required"`
}

type RenameTagRequest struct {
	Name string `json:"name" binding:"required"`
}

type AssignContentItemTagsRequest struct {
	TagIDs []uint `json:"tagIds" binding:"required"`
}

type TagResponseData struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type TagResponse struct {
	Response
	Data TagResponseData `json:"data"`
}

type ListTagsResponseData struct {
	Total int               `json:"total"`
	Items []TagResponseData `json:"items"`
}

type ListTagsResponse struct {
	Response
	Data ListTagsResponseData `json:"data"`
}
