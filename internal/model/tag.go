package model

import "time"

type Tag struct {
	Id        uint      `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (t *Tag) TableName() string {
	return "tags"
}
