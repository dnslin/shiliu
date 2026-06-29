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
	Delete(ctx context.Context, id uint) error
	ClaimFetch(ctx context.Context, feedID uint, startedAt time.Time, staleBefore time.Time) (bool, error)
	UpdateFetchStateIfOwned(ctx context.Context, feedID uint, claimedFetchStartedAt time.Time, status model.FeedFetchStatus, fetchStartedAt *time.Time, lastFetchedAt *time.Time, lastFetchError *string) (bool, error)
	ReleaseFetchClaimIfOwned(ctx context.Context, feedID uint, claimedFetchStartedAt time.Time) (bool, error)
	UpdateFetchState(ctx context.Context, feedID uint, status model.FeedFetchStatus, fetchStartedAt *time.Time, lastFetchedAt *time.Time, lastFetchError *string) error
	AssignFolder(ctx context.Context, feedID uint, folderID *uint) error
	UpdateTitle(ctx context.Context, feedID uint, title string) error
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

func (r *feedRepository) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Where("id = ?", id).Delete(&model.Feed{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func (r *feedRepository) ClaimFetch(ctx context.Context, feedID uint, startedAt time.Time, staleBefore time.Time) (bool, error) {
	if feedID == 0 || startedAt.IsZero() || staleBefore.IsZero() {
		return false, v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Feed{}).
		Where("id = ?", feedID).
		Where("fetch_status <> ? OR fetch_started_at IS NULL OR fetch_started_at <= ?", model.FeedFetchStatusFetching, staleBefore).
		Updates(map[string]interface{}{
			"fetch_status":     model.FeedFetchStatusFetching,
			"fetch_started_at": &startedAt,
			"last_fetch_error": nil,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	if _, err := r.GetByID(ctx, feedID); err != nil {
		return false, err
	}
	return false, nil
}
func (r *feedRepository) UpdateFetchStateIfOwned(ctx context.Context, feedID uint, claimedFetchStartedAt time.Time, status model.FeedFetchStatus, fetchStartedAt *time.Time, lastFetchedAt *time.Time, lastFetchError *string) (bool, error) {
	if feedID == 0 || status == "" || claimedFetchStartedAt.IsZero() {
		return false, v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Feed{}).
		Where("id = ? AND fetch_status = ? AND fetch_started_at = ?", feedID, model.FeedFetchStatusFetching, claimedFetchStartedAt).
		Updates(map[string]interface{}{
			"fetch_status":     status,
			"fetch_started_at": fetchStartedAt,
			"last_fetched_at":  lastFetchedAt,
			"last_fetch_error": lastFetchError,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	return true, nil
}

func (r *feedRepository) ReleaseFetchClaimIfOwned(ctx context.Context, feedID uint, claimedFetchStartedAt time.Time) (bool, error) {
	if feedID == 0 || claimedFetchStartedAt.IsZero() {
		return false, v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Feed{}).
		Where("id = ? AND fetch_status = ? AND fetch_started_at = ?", feedID, model.FeedFetchStatusFetching, claimedFetchStartedAt).
		Updates(map[string]interface{}{
			"fetch_status":     model.FeedFetchStatusIdle,
			"fetch_started_at": nil,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	return true, nil
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

func (r *feedRepository) UpdateTitle(ctx context.Context, feedID uint, title string) error {
	if feedID == 0 {
		return v1.ErrBadRequest
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var feed model.Feed
		if err := tx.Select("title").Where("id = ?", feedID).Take(&feed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return v1.ErrNotFound
			}
			return err
		}
		if feed.Title == title {
			return nil
		}

		result := tx.Model(&model.Feed{}).
			Where("id = ?", feedID).
			Update("title", title)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return v1.ErrNotFound
		}

		return tx.Exec(
			`UPDATE content_item_search_index SET feed_title = ? WHERE rowid IN (SELECT id FROM content_items WHERE feed_id = ?)`,
			title,
			feedID,
		).Error
	})
}
