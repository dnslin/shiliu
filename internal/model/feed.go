package model

import "time"

type FeedType string

const (
	FeedTypeRSS     FeedType = "rss"
	FeedTypePodcast FeedType = "podcast"
)

type FeedFetchStatus string

const (
	FeedFetchStatusIdle     FeedFetchStatus = "idle"
	FeedFetchStatusFetching FeedFetchStatus = "fetching"
	FeedFetchStatusSuccess  FeedFetchStatus = "success"
	FeedFetchStatusFailed   FeedFetchStatus = "failed"
)

type Feed struct {
	Id             uint            `gorm:"primaryKey;column:id"`
	FeedURL        string          `gorm:"column:feed_url;not null;uniqueIndex"`
	Type           FeedType        `gorm:"column:type;not null"`
	FetchStatus    FeedFetchStatus `gorm:"column:fetch_status;not null"`
	FetchStartedAt *time.Time      `gorm:"column:fetch_started_at"`
	LastFetchedAt  *time.Time      `gorm:"column:last_fetched_at"`
	LastFetchError *string         `gorm:"column:last_fetch_error"`
	FolderID       *uint           `gorm:"column:folder_id"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
	UpdatedAt      time.Time       `gorm:"column:updated_at"`
}

func (f *Feed) TableName() string {
	return "feeds"
}
