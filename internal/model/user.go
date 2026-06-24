package model

import "time"

type User struct {
	Id               uint       `gorm:"primaryKey;column:id"`
	Username         string     `gorm:"column:username;not null;uniqueIndex"`
	PasswordHash     string     `gorm:"column:password_hash;not null"`
	FailedLoginCount int        `gorm:"column:failed_login_count;not null;default:0"`
	LockedUntil      *time.Time `gorm:"column:locked_until"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (u *User) TableName() string {
	return "users"
}
