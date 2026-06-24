package repository

import (
	"context"
	"errors"
	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepository interface {
	HasAny(ctx context.Context) (bool, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	UpdatePassword(ctx context.Context, userID uint, currentPasswordHash string, newPasswordHash string) error
	GetByID(ctx context.Context, id uint) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetOnly(ctx context.Context) (*model.User, error)
	ClearLoginFailures(ctx context.Context, userID uint) (*model.User, error)
	RecordLoginFailure(ctx context.Context, userID uint, lockThreshold int, lockedUntil time.Time) (*model.User, error)
}

func NewUserRepository(
	r *Repository,
) UserRepository {
	return &userRepository{
		Repository: r,
	}
}

type userRepository struct {
	*Repository
}

func (r *userRepository) HasAny(ctx context.Context) (bool, error) {
	var user model.User
	result := r.DB(ctx).Select("id").Limit(1).Find(&user)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	if err := r.DB(ctx).Create(user).Error; err != nil {
		return err
	}
	return nil
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	if user.Id == 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.User{}).
		Where("id = ?", user.Id).
		Updates(map[string]interface{}{
			"password_hash":      user.PasswordHash,
			"failed_login_count": user.FailedLoginCount,
			"locked_until":       user.LockedUntil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}
func (r *userRepository) UpdatePassword(ctx context.Context, userID uint, currentPasswordHash string, newPasswordHash string) error {
	if userID == 0 || currentPasswordHash == "" || newPasswordHash == "" {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Where("password_hash = ?", currentPasswordHash).
		Update("password_hash", newPasswordHash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var user model.User
		if err := r.DB(ctx).Select("id").Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return v1.ErrNotFound
			}
			return err
		}
		return v1.ErrInvalidCredentials
	}
	return nil
}

func (r *userRepository) RecordLoginFailure(ctx context.Context, userID uint, lockThreshold int, lockedUntil time.Time) (*model.User, error) {
	if userID == 0 || lockThreshold <= 0 {
		return nil, v1.ErrBadRequest
	}
	var user model.User
	result := r.DB(ctx).Model(&user).
		Clauses(clause.Returning{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_login_count": gorm.Expr("failed_login_count + 1"),
			"locked_until": gorm.Expr(
				"CASE WHEN failed_login_count + 1 >= ? THEN ? ELSE locked_until END",
				lockThreshold,
				lockedUntil,
			),
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, v1.ErrNotFound
	}
	return &user, nil
}

func (r *userRepository) ClearLoginFailures(ctx context.Context, userID uint) (*model.User, error) {
	if userID == 0 {
		return nil, v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Where("locked_until IS NULL OR datetime(locked_until) <= datetime('now')").
		Updates(map[string]interface{}{
			"failed_login_count": 0,
			"locked_until":       nil,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	return r.GetByID(ctx, userID)
}

func (r *userRepository) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.DB(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetOnly(ctx context.Context) (*model.User, error) {
	var user model.User
	if err := r.DB(ctx).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.DB(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
