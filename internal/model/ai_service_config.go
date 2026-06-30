package model

import "time"

type AIServiceConfig struct {
	Id          uint      `gorm:"primaryKey;column:id"`
	SingletonID int       `gorm:"column:singleton_id;not null;default:1"`
	APIBaseURL  string    `gorm:"column:api_base_url;not null"`
	Model       string    `gorm:"column:model;not null"`
	APIKey      string    `gorm:"column:api_key;not null"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (c *AIServiceConfig) TableName() string {
	return "ai_service_configs"
}
