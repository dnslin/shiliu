package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"shiliu/internal/model"
)

type AIServiceConfigRepository interface {
	Save(ctx context.Context, config *model.AIServiceConfig) error
	Get(ctx context.Context) (*model.AIServiceConfig, error)
}

func NewAIServiceConfigRepository(r *Repository) AIServiceConfigRepository {
	return &aiServiceConfigRepository{Repository: r}
}

type aiServiceConfigRepository struct {
	*Repository
}

func (r *aiServiceConfigRepository) Save(ctx context.Context, config *model.AIServiceConfig) error {
	config.SingletonID = 1
	return r.DB(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "singleton_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"api_base_url",
			"model",
			"api_key",
			"updated_at",
		}),
	}).Create(config).Error
}

func (r *aiServiceConfigRepository) Get(ctx context.Context) (*model.AIServiceConfig, error) {
	var config model.AIServiceConfig
	if err := r.DB(ctx).Where("singleton_id = ?", 1).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}
