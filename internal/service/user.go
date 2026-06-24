package service

import (
	"context"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	initializationPasswordMinChars = 12
	bcryptMaxPasswordBytes         = 72
	loginAccessTokenTTL            = 30 * 24 * time.Hour
	loginLockoutThreshold          = 5
	loginLockoutDuration           = 15 * time.Minute
)

type UserService interface {
	IsInitialized(ctx context.Context) (bool, error)
	Initialize(ctx context.Context, req *v1.InitializeRequest) error
	Login(ctx context.Context, req *v1.LoginRequest) (string, error)
	GetProfile(ctx context.Context, userId string) (*v1.GetProfileResponseData, error)
}

func NewUserService(
	service *Service,
	userRepo repository.UserRepository,
) UserService {
	return &userService{
		userRepo: userRepo,
		Service:  service,
	}
}

type userService struct {
	userRepo repository.UserRepository
	*Service
}

func (s *userService) IsInitialized(ctx context.Context) (bool, error) {
	return s.userRepo.HasAny(ctx)
}

func (s *userService) Initialize(ctx context.Context, req *v1.InitializeRequest) error {
	if utf8.RuneCountInString(req.Password) < initializationPasswordMinChars || len(req.Password) > bcryptMaxPasswordBytes {
		return v1.ErrBadRequest
	}
	initialized, err := s.userRepo.HasAny(ctx)
	if err != nil {
		return v1.ErrInternalServerError
	}
	if initialized {
		return v1.ErrAccountAlreadyInitialized
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return err
	}
	user := &model.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
	}

	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		initialized, err := s.userRepo.HasAny(ctx)
		if err != nil {
			return v1.ErrInternalServerError
		}
		if initialized {
			return v1.ErrAccountAlreadyInitialized
		}
		return s.userRepo.Create(ctx, user)
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return v1.ErrAccountAlreadyInitialized
	}
	return err
}

func (s *userService) Login(ctx context.Context, req *v1.LoginRequest) (string, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return "", err
	}
	if user == nil {
		user, err = s.userRepo.GetOnly(ctx)
		if err != nil {
			return "", err
		}
		if user == nil {
			return "", v1.ErrInvalidCredentials
		}
		if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
			return "", v1.ErrAccountLocked
		}
		return s.recordLoginFailure(ctx, user)
	}
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return "", v1.ErrAccountLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return s.recordLoginFailure(ctx, user)
	}
	if user.FailedLoginCount != 0 || user.LockedUntil != nil {
		user, updateErr := s.userRepo.ClearLoginFailures(ctx, user.Id)
		if updateErr != nil {
			return "", updateErr
		}
		if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
			return "", v1.ErrAccountLocked
		}
	}

	token, err := s.jwt.GenToken(strconv.FormatUint(uint64(user.Id), 10), time.Now().Add(loginAccessTokenTTL))
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *userService) recordLoginFailure(ctx context.Context, user *model.User) (string, error) {
	lockedUntil := time.Now().Add(loginLockoutDuration)
	user, updateErr := s.userRepo.RecordLoginFailure(ctx, user.Id, loginLockoutThreshold, lockedUntil)
	if updateErr != nil {
		return "", updateErr
	}
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return "", v1.ErrAccountLocked
	}
	return "", v1.ErrInvalidCredentials
}

func (s *userService) GetProfile(ctx context.Context, userId string) (*v1.GetProfileResponseData, error) {
	id, err := parseUserID(userId)
	if err != nil {
		return nil, v1.ErrBadRequest
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &v1.GetProfileResponseData{
		Id:       user.Id,
		Username: user.Username,
	}, nil
}

func parseUserID(userId string) (uint, error) {
	id, err := strconv.ParseUint(userId, 10, strconv.IntSize)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
