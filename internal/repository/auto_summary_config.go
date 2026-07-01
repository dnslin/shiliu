package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"shiliu/internal/model"
)

type AutoSummaryConfigRepository interface {
	Save(ctx context.Context, config *model.AutoSummaryConfig) error
	Get(ctx context.Context) (*model.AutoSummaryConfig, error)
}

func NewAutoSummaryConfigRepository(r *Repository) AutoSummaryConfigRepository {
	return &autoSummaryConfigRepository{Repository: r}
}

type autoSummaryConfigRepository struct {
	*Repository
}

func (r *autoSummaryConfigRepository) Save(ctx context.Context, config *model.AutoSummaryConfig) error {
	config.SingletonID = 1
	if config.ContentTypeScope == "" {
		config.ContentTypeScope = model.AutoSummaryContentTypeScopeAll
	}
	return r.DB(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "singleton_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled",
			"content_type_scope",
			"enabled_at",
			"updated_at",
		}),
	}).Create(config).Error
}

func (r *autoSummaryConfigRepository) Get(ctx context.Context) (*model.AutoSummaryConfig, error) {
	var config model.AutoSummaryConfig
	if err := r.DB(ctx).Where("singleton_id = ?", 1).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}
