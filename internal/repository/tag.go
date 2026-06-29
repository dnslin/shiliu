package repository

import (
	"context"
	"errors"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"

	"gorm.io/gorm"
)

type TagRepository interface {
	Create(ctx context.Context, tag *model.Tag) error
	GetByID(ctx context.Context, id uint) (*model.Tag, error)
	List(ctx context.Context) ([]*model.Tag, error)
	Rename(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
}

func NewTagRepository(r *Repository) TagRepository {
	return &tagRepository{Repository: r}
}

type tagRepository struct {
	*Repository
}

func (r *tagRepository) Create(ctx context.Context, tag *model.Tag) error {
	return r.DB(ctx).Create(tag).Error
}

func (r *tagRepository) GetByID(ctx context.Context, id uint) (*model.Tag, error) {
	var tag model.Tag
	if err := r.DB(ctx).Where("id = ?", id).First(&tag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	return &tag, nil
}

func (r *tagRepository) List(ctx context.Context) ([]*model.Tag, error) {
	var tags []*model.Tag
	if err := r.DB(ctx).Order("id ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *tagRepository) Rename(ctx context.Context, id uint, name string) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Tag{}).
		Where("id = ?", id).
		Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func (r *tagRepository) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Where("id = ?", id).Delete(&model.Tag{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}
