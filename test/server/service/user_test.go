package service_test

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
	"shiliu/pkg/config"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
	"shiliu/pkg/sid"
	mock_repository "shiliu/test/mocks/repository"

	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestUserService_IsInitializedFalseWhenNoAccountExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	mockUserRepo.EXPECT().HasAny(ctx).Return(false, nil)

	initialized, err := userService.IsInitialized(ctx)

	assert.NoError(t, err)
	assert.False(t, initialized)
}

func TestUserService_IsInitializedTrueWhenAccountExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	mockUserRepo.EXPECT().HasAny(ctx).Return(true, nil)

	initialized, err := userService.IsInitialized(ctx)

	assert.NoError(t, err)
	assert.True(t, initialized)
}
func TestUserService_InitializeCreatesFirstAccountWithBcryptCost12(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.InitializeRequest{
		Username: "first-account",
		Password: "123456789012",
	}

	mockUserRepo.EXPECT().HasAny(ctx).Return(false, nil)
	mockTm.EXPECT().Transaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			mockUserRepo.EXPECT().HasAny(ctx).Return(false, nil)
			return fn(ctx)
		})
	mockUserRepo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, user *model.User) error {
			assert.Equal(t, req.Username, user.Username)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)))
			cost, err := bcrypt.Cost([]byte(user.PasswordHash))
			assert.NoError(t, err)
			assert.Equal(t, 12, cost)
			return nil
		})

	err := userService.Initialize(ctx, req)

	assert.NoError(t, err)
}

func TestUserService_InitializeRejectsWhenAnyAccountExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.InitializeRequest{
		Username: "second-account",
		Password: "123456789012",
	}
	mockUserRepo.EXPECT().HasAny(ctx).Return(true, nil)
	err := userService.Initialize(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountAlreadyInitialized)
}
func TestUserService_InitializeRejectsIfAccountAppearsInsideTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.InitializeRequest{
		Username: "second-account",
		Password: "123456789012",
	}
	mockUserRepo.EXPECT().HasAny(ctx).Return(false, nil)
	mockTm.EXPECT().Transaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			mockUserRepo.EXPECT().HasAny(ctx).Return(true, nil)
			return fn(ctx)
		})

	err := userService.Initialize(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountAlreadyInitialized)
}

func TestUserService_InitializeMapsCreateRaceToAlreadyInitialized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.InitializeRequest{
		Username: "racer",
		Password: "123456789012",
	}
	mockUserRepo.EXPECT().HasAny(ctx).Return(false, nil)
	mockTm.EXPECT().Transaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			mockUserRepo.EXPECT().HasAny(ctx).Return(false, nil)
			return fn(ctx)
		})
	mockUserRepo.EXPECT().Create(ctx, gomock.Any()).Return(gorm.ErrDuplicatedKey)

	err := userService.Initialize(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountAlreadyInitialized)
}

func TestUserService_InitializeRejectsShortPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	err := userService.Initialize(context.Background(), &v1.InitializeRequest{
		Username: "first-account",
		Password: "12345678901",
	})

	assert.ErrorIs(t, err, v1.ErrBadRequest)
}

func TestUserService_InitializeRejectsPasswordShorterThan12Characters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	err := userService.Initialize(context.Background(), &v1.InitializeRequest{
		Username: "first-account",
		Password: "密码密码密码",
	})

	assert.ErrorIs(t, err, v1.ErrBadRequest)
}

func TestUserService_InitializeRejectsPasswordLongerThanBcryptLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	err := userService.Initialize(context.Background(), &v1.InitializeRequest{
		Username: "first-account",
		Password: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	assert.ErrorIs(t, err, v1.ErrBadRequest)
}

func TestUserService_LoginIssuesThirtyDayAccessToken(t *testing.T) {
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

	loginStartedAt := time.Now()
	token, err := userService.Login(ctx, req)

	assert.NoError(t, err)
	claims, err := j.ParseToken(token)
	assert.NoError(t, err)
	if assert.NotNil(t, claims.ExpiresAt) {
		assert.WithinDuration(t, loginStartedAt.Add(30*24*time.Hour), claims.ExpiresAt.Time, 2*time.Second)
	}
}

func TestUserService_LoginClearsFailureStateOnSuccess(t *testing.T) {
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
	lockedUntil := time.Now().Add(-time.Minute)

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 3,
		LockedUntil:      &lockedUntil,
	}, nil)
	mockUserRepo.EXPECT().ClearLoginFailures(ctx, uint(123)).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 0,
		LockedUntil:      nil,
	}, nil)

	token, err := userService.Login(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestUserService_LoginRejectsIfSuccessClearObservesFreshLock(t *testing.T) {
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
		t.Fatal("failed to hash password")
	}
	staleLockedUntil := time.Now().Add(-time.Minute)
	freshLockedUntil := time.Now().Add(15 * time.Minute)

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 4,
		LockedUntil:      &staleLockedUntil,
	}, nil)
	mockUserRepo.EXPECT().ClearLoginFailures(ctx, uint(123)).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 5,
		LockedUntil:      &freshLockedUntil,
	}, nil)

	_, err = userService.Login(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountLocked)
}

func TestUserService_LoginRecordsPasswordFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.LoginRequest{
		Username: "testuser",
		Password: "wrong-password",
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Error("failed to hash password")
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 2,
	}, nil)
	mockUserRepo.EXPECT().RecordLoginFailure(ctx, uint(123), 5, gomock.Any()).DoAndReturn(
		func(ctx context.Context, userID uint, lockThreshold int, lockedUntil time.Time) (*model.User, error) {
			assert.Equal(t, uint(123), userID)
			assert.Equal(t, 5, lockThreshold)
			assert.WithinDuration(t, time.Now().Add(15*time.Minute), lockedUntil, 2*time.Second)
			return &model.User{
				Id:               userID,
				Username:         req.Username,
				PasswordHash:     string(hashedPassword),
				FailedLoginCount: 3,
			}, nil
		})

	_, err = userService.Login(ctx, req)

	assert.ErrorIs(t, err, v1.ErrInvalidCredentials)
}

func TestUserService_LoginLocksAccountAfterFiveFailures(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.LoginRequest{
		Username: "testuser",
		Password: "wrong-password",
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Error("failed to hash password")
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 4,
	}, nil)
	lockStartedAt := time.Now()
	mockUserRepo.EXPECT().RecordLoginFailure(ctx, uint(123), 5, gomock.Any()).DoAndReturn(
		func(ctx context.Context, userID uint, lockThreshold int, lockedUntil time.Time) (*model.User, error) {
			assert.Equal(t, uint(123), userID)
			assert.Equal(t, 5, lockThreshold)
			assert.WithinDuration(t, lockStartedAt.Add(15*time.Minute), lockedUntil, 2*time.Second)
			return &model.User{
				Id:               userID,
				Username:         req.Username,
				PasswordHash:     string(hashedPassword),
				FailedLoginCount: 5,
				LockedUntil:      &lockedUntil,
			}, nil
		})

	_, err = userService.Login(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountLocked)
}

func TestUserService_LoginRejectsLockedAccountBeforePasswordCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.LoginRequest{
		Username: "testuser",
		Password: "correct-password",
	}
	lockedUntil := time.Now().Add(10 * time.Minute)

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(&model.User{
		Id:               123,
		Username:         req.Username,
		PasswordHash:     "not-a-bcrypt-hash",
		FailedLoginCount: 5,
		LockedUntil:      &lockedUntil,
	}, nil)

	_, err := userService.Login(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountLocked)
}

func TestUserService_LoginRecordsUnknownUsernameFailureOnOnlyAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.LoginRequest{
		Username: "wrong-account",
		Password: "wrong-password",
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal("failed to hash password")
	}

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(nil, nil)
	mockUserRepo.EXPECT().GetOnly(ctx).Return(&model.User{
		Id:               123,
		Username:         "real-account",
		PasswordHash:     string(hashedPassword),
		FailedLoginCount: 4,
	}, nil)
	lockStartedAt := time.Now()
	mockUserRepo.EXPECT().RecordLoginFailure(ctx, uint(123), 5, gomock.Any()).DoAndReturn(
		func(ctx context.Context, userID uint, lockThreshold int, lockedUntil time.Time) (*model.User, error) {
			assert.Equal(t, uint(123), userID)
			assert.Equal(t, 5, lockThreshold)
			assert.WithinDuration(t, lockStartedAt.Add(15*time.Minute), lockedUntil, 2*time.Second)
			return &model.User{
				Id:               userID,
				Username:         "real-account",
				PasswordHash:     string(hashedPassword),
				FailedLoginCount: 5,
				LockedUntil:      &lockedUntil,
			}, nil
		})

	_, err = userService.Login(ctx, req)

	assert.ErrorIs(t, err, v1.ErrAccountLocked)
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

	mockUserRepo.EXPECT().GetByUsername(ctx, req.Username).Return(nil, nil)
	mockUserRepo.EXPECT().GetOnly(ctx).Return(nil, nil)

	_, err := userService.Login(ctx, req)

	assert.ErrorIs(t, err, v1.ErrInvalidCredentials)
}

func TestUserService_ChangePasswordUpdatesPasswordHashWithCost12(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.ChangePasswordRequest{
		OldPassword: "old-password-12",
		NewPassword: "new-password-12",
	}
	oldHash, err := bcrypt.GenerateFromPassword([]byte(req.OldPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal("failed to hash old password")
	}

	mockUserRepo.EXPECT().GetByID(ctx, uint(123)).Return(&model.User{
		Id:           123,
		Username:     "testuser",
		PasswordHash: string(oldHash),
	}, nil)
	mockUserRepo.EXPECT().UpdatePassword(ctx, uint(123), string(oldHash), gomock.Any()).DoAndReturn(
		func(ctx context.Context, userID uint, currentPasswordHash string, newPasswordHash string) error {
			assert.Equal(t, uint(123), userID)
			assert.Equal(t, string(oldHash), currentPasswordHash)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(newPasswordHash), []byte(req.NewPassword)))
			assert.Error(t, bcrypt.CompareHashAndPassword([]byte(newPasswordHash), []byte(req.OldPassword)))
			cost, err := bcrypt.Cost([]byte(newPasswordHash))
			assert.NoError(t, err)
			assert.Equal(t, 12, cost)
			return nil
		})

	err = userService.ChangePassword(ctx, "123", req)

	assert.NoError(t, err)
}

func TestUserService_ChangePasswordMakesNewPasswordLoginAndOldPasswordFail(t *testing.T) {
	userService, userRepo := newPasswordIntegrationService(t)
	ctx := context.Background()
	oldPassword := "old-password-12"
	newPassword := "new-password-12"
	oldHash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
	require.NoError(t, err)
	user := &model.User{
		Username:     "testuser",
		PasswordHash: string(oldHash),
	}
	require.NoError(t, userRepo.Create(ctx, user))

	require.NoError(t, userService.ChangePassword(ctx, strconv.FormatUint(uint64(user.Id), 10), &v1.ChangePasswordRequest{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}))

	oldToken, err := userService.Login(ctx, &v1.LoginRequest{Username: user.Username, Password: oldPassword})
	assert.ErrorIs(t, err, v1.ErrInvalidCredentials)
	assert.Empty(t, oldToken)

	newToken, err := userService.Login(ctx, &v1.LoginRequest{Username: user.Username, Password: newPassword})
	assert.NoError(t, err)
	assert.NotEmpty(t, newToken)

	got, err := userRepo.GetByID(ctx, user.Id)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte(newPassword)))
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte(oldPassword)))
	cost, err := bcrypt.Cost([]byte(got.PasswordHash))
	require.NoError(t, err)
	assert.Equal(t, 12, cost)
}

func newPasswordIntegrationService(t *testing.T) (service.UserService, repository.UserRepository) {
	t.Helper()
	conf := newServiceDBConfig(t)
	runServiceMigrations(t, conf.GetString("data.db.user.dsn"))
	db := repository.NewDB(conf, logger)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
	})
	repo := repository.NewRepository(logger, db)
	userRepo := repository.NewUserRepository(repo)
	srv := service.NewService(repository.NewTransaction(repo), logger, sf, j)
	return service.NewUserService(srv, userRepo), userRepo
}

func newServiceDBConfig(t *testing.T) *viper.Viper {
	t.Helper()
	conf := viper.New()
	conf.Set("data.db.user.driver", "sqlite")
	conf.Set("data.db.user.dsn", filepath.Join(t.TempDir(), "shiliu-service-test.db")+"?_busy_timeout=5000")
	conf.Set("data.db.user.debug", false)
	return conf
}

func runServiceMigrations(t *testing.T, dsn string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "migration-test.yml")
	content := fmt.Sprintf("data:\n  db:\n    user:\n      driver: sqlite\n      dsn: %q\n      debug: false\n", dsn)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))
	cmd := exec.Command("go", "run", "./cmd/migration", "-conf", configPath, "-direction", "up", "-path", "migrations")
	cmd.Dir = serviceRepoRoot(t)
	cmd.Env = append(os.Environ(), "APP_CONF="+configPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func serviceRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func TestUserService_ChangePasswordRecordsWrongOldPasswordFailureWithoutUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	ctx := context.Background()
	req := &v1.ChangePasswordRequest{
		OldPassword: "wrong-password-12",
		NewPassword: "new-password-12",
	}
	oldHash, err := bcrypt.GenerateFromPassword([]byte("current-password-12"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal("failed to hash current password")
	}
	mockUserRepo.EXPECT().GetByID(ctx, uint(123)).Return(&model.User{
		Id:           123,
		Username:     "testuser",
		PasswordHash: string(oldHash),
	}, nil)
	mockUserRepo.EXPECT().RecordLoginFailure(ctx, uint(123), 5, gomock.Any()).DoAndReturn(
		func(ctx context.Context, userID uint, lockThreshold int, lockedUntil time.Time) (*model.User, error) {
			assert.Equal(t, uint(123), userID)
			assert.Equal(t, 5, lockThreshold)
			assert.WithinDuration(t, time.Now().Add(15*time.Minute), lockedUntil, 2*time.Second)
			return &model.User{
				Id:               userID,
				Username:         "testuser",
				PasswordHash:     string(oldHash),
				FailedLoginCount: 3,
			}, nil
		})

	err = userService.ChangePassword(ctx, "123", req)

	assert.ErrorIs(t, err, v1.ErrInvalidCredentials)
}

func TestUserService_ChangePasswordRejectsShortNewPasswordWithoutLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	err := userService.ChangePassword(context.Background(), "123", &v1.ChangePasswordRequest{
		OldPassword: "old-password-12",
		NewPassword: "short",
	})

	assert.ErrorIs(t, err, v1.ErrBadRequest)
}

func TestUserService_ChangePasswordRejectsUnchangedPasswordWithoutLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_repository.NewMockUserRepository(ctrl)
	mockTm := mock_repository.NewMockTransaction(ctrl)
	srv := service.NewService(mockTm, logger, sf, j)
	userService := service.NewUserService(srv, mockUserRepo)

	err := userService.ChangePassword(context.Background(), "123", &v1.ChangePasswordRequest{
		OldPassword: "same-password-12",
		NewPassword: "same-password-12",
	})

	assert.ErrorIs(t, err, v1.ErrBadRequest)
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
