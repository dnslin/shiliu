package repository

import (
	"context"
	"errors"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"

	"gorm.io/gorm"
)

type FolderRepository interface {
	Create(ctx context.Context, folder *model.Folder) error
	GetByID(ctx context.Context, id uint) (*model.Folder, error)
	List(ctx context.Context) ([]*model.Folder, error)
	Rename(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
}

func NewFolderRepository(r *Repository) FolderRepository {
	return &folderRepository{Repository: r}
}

type folderRepository struct {
	*Repository
}

func (r *folderRepository) Create(ctx context.Context, folder *model.Folder) error {
	return r.DB(ctx).Create(folder).Error
}

func (r *folderRepository) GetByID(ctx context.Context, id uint) (*model.Folder, error) {
	var folder model.Folder
	if err := r.DB(ctx).Where("id = ?", id).First(&folder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	return &folder, nil
}

func (r *folderRepository) List(ctx context.Context) ([]*model.Folder, error) {
	var folders []*model.Folder
	if err := r.DB(ctx).Order("id ASC").Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *folderRepository) Rename(ctx context.Context, id uint, name string) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.Folder{}).
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

func (r *folderRepository) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Feed{}).Where("folder_id = ?", id).Update("folder_id", nil).Error; err != nil {
			return err
		}
		result := tx.Where("id = ?", id).Delete(&model.Folder{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return v1.ErrNotFound
		}
		return nil
	})
}
