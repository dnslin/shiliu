package repository

import (
	"context"
	"errors"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"

	"gorm.io/gorm"
)

type FeedRepository interface {
	Create(ctx context.Context, feed *model.Feed) error
	GetByID(ctx context.Context, id uint) (*model.Feed, error)
	GetByURL(ctx context.Context, feedURL string) (*model.Feed, error)
	List(ctx context.Context) ([]*model.Feed, error)
	UpdateFetchState(ctx context.Context, feedID uint, status model.FeedFetchStatus, fetchStartedAt *time.Time, lastFetchedAt *time.Time, lastFetchError *string) error
	AssignFolder(ctx context.Context, feedID uint, folderID *uint) error
}

func NewFeedRepository(r *Repository) FeedRepository {
	return &feedRepository{Repository: r}
}

type feedRepository struct {
	*Repository
}

func (r *feedRepository) Create(ctx context.Context, feed *model.Feed) error {
	if feed.FetchStatus == "" {
		feed.FetchStatus = model.FeedFetchStatusIdle
	}
	return r.DB(ctx).Create(feed).Error
}

func (r *feedRepository) GetByID(ctx context.Context, id uint) (*model.Feed, error) {
	var feed model.Feed
	if err := r.DB(ctx).Where("id = ?", id).First(&feed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	return &feed, nil
}

func (r *feedRepository) GetByURL(ctx context.Context, feedURL string) (*model.Feed, error) {
	var feed model.Feed
	if err := r.DB(ctx).Where("feed_url = ?", feedURL).First(&feed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &feed, nil
}

func (r *feedRepository) List(ctx context.Context) ([]*model.Feed, error) {
	var feeds []*model.Feed
	if err := r.DB(ctx).Order("id ASC").Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *feedRepository) UpdateFetchState(ctx context.Context, feedID uint, status model.FeedFetchStatus, fetchStartedAt *time.Time, lastFetchedAt *time.Time, lastFetchError *string) error {
	if feedID == 0 || status == "" {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Feed{}).
		Where("id = ?", feedID).
		Updates(map[string]interface{}{
			"fetch_status":     status,
			"fetch_started_at": fetchStartedAt,
			"last_fetched_at":  lastFetchedAt,
			"last_fetch_error": lastFetchError,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func (r *feedRepository) AssignFolder(ctx context.Context, feedID uint, folderID *uint) error {
	if feedID == 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Feed{}).
		Where("id = ?", feedID).
		Update("folder_id", folderID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}
