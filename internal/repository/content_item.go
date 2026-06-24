package repository

import (
	"context"
	"errors"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"

	"gorm.io/gorm"
)

type ContentItemRepository interface {
	Create(ctx context.Context, item *model.ContentItem) error
	GetByID(ctx context.Context, id uint) (*model.ContentItem, error)
	GetByFeedAndDedupeKey(ctx context.Context, feedID uint, dedupeKey string) (*model.ContentItem, error)
	ListByFeedID(ctx context.Context, feedID uint, limit int) ([]*model.ContentItem, error)
	UpdateAudioProgress(ctx context.Context, itemID uint, progressSeconds int) error
}

func NewContentItemRepository(r *Repository) ContentItemRepository {
	return &contentItemRepository{Repository: r}
}

type contentItemRepository struct {
	*Repository
}

func (r *contentItemRepository) Create(ctx context.Context, item *model.ContentItem) error {
	if item.FetchedAt.IsZero() {
		item.FetchedAt = time.Now().UTC()
	}
	return r.DB(ctx).Create(item).Error
}

func (r *contentItemRepository) GetByID(ctx context.Context, id uint) (*model.ContentItem, error) {
	var item model.ContentItem
	if err := r.DB(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *contentItemRepository) GetByFeedAndDedupeKey(ctx context.Context, feedID uint, dedupeKey string) (*model.ContentItem, error) {
	var item model.ContentItem
	if err := r.DB(ctx).Where("feed_id = ? AND dedupe_key = ?", feedID, dedupeKey).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *contentItemRepository) ListByFeedID(ctx context.Context, feedID uint, limit int) ([]*model.ContentItem, error) {
	if feedID == 0 {
		return nil, v1.ErrBadRequest
	}
	query := r.DB(ctx).Where("feed_id = ?", feedID).Order("published_at DESC, id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []*model.ContentItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *contentItemRepository) UpdateAudioProgress(ctx context.Context, itemID uint, progressSeconds int) error {
	if itemID == 0 || progressSeconds < 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.ContentItem{}).
		Where("id = ?", itemID).
		Update("audio_progress_seconds", progressSeconds)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}
