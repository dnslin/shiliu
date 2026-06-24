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
	Register(ctx context.Context, req *v1.RegisterRequest) error
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

func (s *userService) Register(ctx context.Context, req *v1.RegisterRequest) error {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return v1.ErrInternalServerError
	}
	if user != nil {
		return v1.ErrUsernameAlreadyUse
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user = &model.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
	}
	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		if err = s.userRepo.Create(ctx, user); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return v1.ErrUsernameAlreadyUse
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
