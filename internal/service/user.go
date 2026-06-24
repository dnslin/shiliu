package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
	if len(req.Password) < 12 {
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
	if err != nil || user == nil {
		return "", v1.ErrUnauthorized
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return "", err
	}
	token, err := s.jwt.GenToken(strconv.FormatUint(uint64(user.Id), 10), time.Now().Add(time.Hour*24*90))
	if err != nil {
		return "", err
	}

	return token, nil
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
