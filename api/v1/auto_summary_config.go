package v1

import "time"

type SaveAutoSummaryConfigRequest struct {
	Enabled          bool   `json:"enabled"`
	ContentTypeScope string `json:"contentTypeScope" binding:"required" example:"all"`
}

type AutoSummaryConfigResponseData struct {
	Enabled          bool       `json:"enabled"`
	ContentTypeScope string     `json:"contentTypeScope" example:"all"`
	EnabledAt        *time.Time `json:"enabledAt"`
}

type AutoSummaryConfigResponse struct {
	Response
	Data AutoSummaryConfigResponseData `json:"data"`
}
