package service_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/service"
	"shiliu/pkg/config"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
	"shiliu/pkg/sid"
	mock_repository "shiliu/test/mocks/repository"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

var (
	logger *log.Logger
	j      *jwt.JWT
	sf     *sid.Sid
)

func TestMain(m *testing.M) {
	fmt.Println("begin")

	err := os.Setenv("APP_CONF", "../../../config/local.yml")
	if err != nil {
		panic(err)
	}

	var envConf = flag.String("conf", "config/local.yml", "config path, eg: -conf ./config/local.yml")
	flag.Parse()
	conf := config.NewConfig(*envConf)

	logger = log.NewLog(conf)
	j = jwt.NewJwt(conf)
	sf = sid.NewSid()

	code := m.Run()
	fmt.Println("test end")

	os.Exit(code)
}

func TestUserService_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)

	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.RegisterRequest{
		Username: "testuser",
		Password: "password",
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(nil, nil)
	mockTm.EXPECT().Transaction(ctx, gomock.Any()).Return(nil)

	err := userService.Register(ctx, req)

	assert.NoError(t, err)
}

func TestUserService_Register_UsernameExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.RegisterRequest{
		Username: "testuser",
		Password: "password",
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{}, nil)

	err := userService.Register(ctx, req)

	assert.Error(t, err)
}

func TestUserService_Login(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.LoginRequest{
		Username: "testuser",
		Password: "password",
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		t.Error("failed to hash password")
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{
		Id:           123,
		PasswordHash: string(hashedPassword),
	}, nil)

	token, err := userService.Login(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestUserService_Login_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.LoginRequest{
		Username: "missing",
		Password: "password",
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(nil, errors.New("user not found"))

	_, err := userService.Login(ctx, req)

	assert.Error(t, err)
}

func TestUserService_GetProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	userId := "123"

	mockUserRepo.EXPECT().GetByID(ctx, uint(123)).Return(&model.User{
		Id:       123,
		Username: "testuser",
	}, nil)

	user, err := userService.GetProfile(ctx, userId)

	assert.NoError(t, err)
	assert.Equal(t, uint(123), user.Id)
	assert.Equal(t, "testuser", user.Username)
}
