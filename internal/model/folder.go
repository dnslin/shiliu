package model

import "time"

type Folder struct {
	Id        uint      `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (f *Folder) TableName() string {
	return "folders"
}
