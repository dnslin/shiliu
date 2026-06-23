package service_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strconv"
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
	"gorm.io/gorm"
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

	assert.ErrorIs(t, err, v1.ErrUsernameAlreadyUse)
}

func TestUserService_Register_DuplicateOnCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.RegisterRequest{
		Username: "racer",
		Password: "password",
	}

	// The pre-check sees no user (lost the race), but Create hits the unique index.
	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(nil, nil)
	mockTm.EXPECT().Transaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
	mockUserRepo.EXPECT().Create(ctx, gomock.Any()).Return(gorm.ErrDuplicatedKey)

	err := userService.Register(ctx, req)

	assert.ErrorIs(t, err, v1.ErrUsernameAlreadyUse)
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

func TestUserService_GetProfile_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	// "Just over the platform uint width" must overflow on both 32-bit and 64-bit
	// targets. gomock fails the test if GetByID is reached, proving no lookup happens
	// for a malformed/out-of-range subject.
	overflow := new(big.Int).Lsh(big.NewInt(1), uint(strconv.IntSize)).String()
	for _, badID := range []string{"not-a-number", "-1", overflow} {
		_, err := userService.GetProfile(context.Background(), badID)
		assert.ErrorIs(t, err, v1.ErrBadRequest, "expected bad-request rejection for id %q", badID)
	}
}

func TestUserService_GetProfile_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	mockUserRepo.EXPECT().GetByID(ctx, uint(123)).Return(nil, v1.ErrNotFound)

	_, err := userService.GetProfile(ctx, "123")

	assert.ErrorIs(t, err, v1.ErrNotFound)
}
