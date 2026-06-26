package repository

import (
	"context"
	"errors"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"

	"gorm.io/gorm"
)

type ContentItemListFilter struct {
	ContentType      *model.ContentItemType
	ProcessingStatus *model.ContentItemProcessingStatus
	Mark             *model.ContentItemMark
	FeedID           *uint
}

type ContentItemRepository interface {
	Create(ctx context.Context, item *model.ContentItem) error
	GetByID(ctx context.Context, id uint) (*model.ContentItem, error)
	GetByFeedAndDedupeKey(ctx context.Context, feedID uint, dedupeKey string) (*model.ContentItem, error)
	List(ctx context.Context, filter ContentItemListFilter, limit int, offset int) ([]*model.ContentItem, int64, error)
	ListByFeedID(ctx context.Context, feedID uint, limit int) ([]*model.ContentItem, error)
	UpdateProcessingStatus(ctx context.Context, itemID uint, status model.ContentItemProcessingStatus) error
	UpdateMark(ctx context.Context, itemID uint, mark model.ContentItemMark, marked bool) error
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
	if item.ProcessingStatus == "" {
		item.ProcessingStatus = model.ContentItemProcessingStatusUnprocessed
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

const contentItemListOrder = "COALESCE(published_at, fetched_at) DESC, id DESC"

func (r *contentItemRepository) List(ctx context.Context, filter ContentItemListFilter, limit int, offset int) ([]*model.ContentItem, int64, error) {
	if limit < 0 || offset < 0 {
		return nil, 0, v1.ErrBadRequest
	}
	query, err := applyContentItemListFilter(r.DB(ctx).Model(&model.ContentItem{}), filter)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = query.Select("id", "feed_id", "type", "title", "available_text", "published_at", "fetched_at", "processing_status", "marked_later", "favorited", "audio_progress_seconds").Order(contentItemListOrder)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var items []*model.ContentItem
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *contentItemRepository) ListByFeedID(ctx context.Context, feedID uint, limit int) ([]*model.ContentItem, error) {
	if feedID == 0 {
		return nil, v1.ErrBadRequest
	}
	query := r.DB(ctx).Where("feed_id = ?", feedID).Order(contentItemListOrder)
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []*model.ContentItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func applyContentItemListFilter(query *gorm.DB, filter ContentItemListFilter) (*gorm.DB, error) {
	if filter.ContentType != nil {
		if !validContentItemType(*filter.ContentType) {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("type = ?", *filter.ContentType)
	}
	if filter.ProcessingStatus != nil {
		if !validContentItemProcessingStatus(*filter.ProcessingStatus) {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("processing_status = ?", *filter.ProcessingStatus)
	}
	if filter.Mark != nil {
		column, ok := contentItemMarkColumn(*filter.Mark)
		if !ok {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where(column+" = ?", true)
	}
	if filter.FeedID != nil {
		if *filter.FeedID == 0 {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("feed_id = ?", *filter.FeedID)
	}
	return query, nil
}

func validContentItemType(contentType model.ContentItemType) bool {
	switch contentType {
	case model.ContentItemTypeText, model.ContentItemTypeAudio:
		return true
	default:
		return false
	}
}

func validContentItemProcessingStatus(status model.ContentItemProcessingStatus) bool {
	switch status {
	case model.ContentItemProcessingStatusUnprocessed, model.ContentItemProcessingStatusCompleted:
		return true
	default:
		return false
	}
}

func (r *contentItemRepository) UpdateProcessingStatus(ctx context.Context, itemID uint, status model.ContentItemProcessingStatus) error {
	if itemID == 0 || !validContentItemProcessingStatus(status) {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.ContentItem{}).
		Where("id = ?", itemID).
		Update("processing_status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func (r *contentItemRepository) UpdateMark(ctx context.Context, itemID uint, mark model.ContentItemMark, marked bool) error {
	if itemID == 0 {
		return v1.ErrBadRequest
	}
	column, ok := contentItemMarkColumn(mark)
	if !ok {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.ContentItem{}).
		Where("id = ?", itemID).
		Update(column, marked)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func contentItemMarkColumn(mark model.ContentItemMark) (string, bool) {
	switch mark {
	case model.ContentItemMarkLater:
		return "marked_later", true
	case model.ContentItemMarkFavorite:
		return "favorited", true
	default:
		return "", false
	}
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
