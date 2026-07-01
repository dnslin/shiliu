package model

import "time"

type AutoSummaryContentTypeScope string

const (
	AutoSummaryContentTypeScopeText  AutoSummaryContentTypeScope = "text"
	AutoSummaryContentTypeScopeAudio AutoSummaryContentTypeScope = "audio"
	AutoSummaryContentTypeScopeAll   AutoSummaryContentTypeScope = "all"
)

type AutoSummaryConfig struct {
	Id               uint                        `gorm:"primaryKey;column:id"`
	SingletonID      int                         `gorm:"column:singleton_id;not null;default:1"`
	Enabled          bool                        `gorm:"column:enabled;not null"`
	ContentTypeScope AutoSummaryContentTypeScope `gorm:"column:content_type_scope;not null"`
	EnabledAt        *time.Time                  `gorm:"column:enabled_at"`
	CreatedAt        time.Time                   `gorm:"column:created_at"`
	UpdatedAt        time.Time                   `gorm:"column:updated_at"`
}

func (c *AutoSummaryConfig) TableName() string {
	return "auto_summary_configs"
}
