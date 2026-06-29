package v1

import "encoding/json"

type CreateFolderRequest struct {
	Name string `json:"name" binding:"required"`
}

type RenameFolderRequest struct {
	Name string `json:"name" binding:"required"`
}

type AssignFeedFolderRequest struct {
	FolderID    *uint `json:"folderId"`
	folderIDSet bool
}

func (r *AssignFeedFolderRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		FolderID json.RawMessage `json:"folderId"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.folderIDSet = raw.FolderID != nil
	if !r.folderIDSet || string(raw.FolderID) == "null" {
		r.FolderID = nil
		return nil
	}
	var folderID uint
	if err := json.Unmarshal(raw.FolderID, &folderID); err != nil {
		return err
	}
	r.FolderID = &folderID
	return nil
}

func (r *AssignFeedFolderRequest) HasFolderID() bool {
	return r != nil && r.folderIDSet
}

type FolderResponseData struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type FolderResponse struct {
	Response
	Data FolderResponseData `json:"data"`
}

type ListFoldersResponseData struct {
	Total int                  `json:"total"`
	Items []FolderResponseData `json:"items"`
}

type ListFoldersResponse struct {
	Response
	Data ListFoldersResponseData `json:"data"`
}
