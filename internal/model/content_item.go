package model

import "time"

type ContentItemType string

const (
	ContentItemTypeText  ContentItemType = "text"
	ContentItemTypeAudio ContentItemType = "audio"
)

type ContentItem struct {
	Id                   uint            `gorm:"primaryKey;column:id"`
	FeedID               uint            `gorm:"column:feed_id;not null;index"`
	DedupeKey            string          `gorm:"column:dedupe_key;not null"`
	Type                 ContentItemType `gorm:"column:type;not null"`
	Title                string          `gorm:"column:title;not null"`
	Description          string          `gorm:"column:description;not null"`
	Content              string          `gorm:"column:content;not null"`
	ShowNotes            string          `gorm:"column:show_notes;not null"`
	DescriptionSafe      string          `gorm:"column:description_safe;not null"`
	ContentSafe          string          `gorm:"column:content_safe;not null"`
	ShowNotesSafe        string          `gorm:"column:show_notes_safe;not null"`
	AvailableText        string          `gorm:"column:available_text;not null"`
	PublishedAt          *time.Time      `gorm:"column:published_at"`
	FetchedAt            time.Time       `gorm:"column:fetched_at;not null"`
	AudioProgressSeconds int             `gorm:"column:audio_progress_seconds;not null"`
	CreatedAt            time.Time       `gorm:"column:created_at"`
	UpdatedAt            time.Time       `gorm:"column:updated_at"`
}

func (c *ContentItem) TableName() string {
	return "content_items"
}
