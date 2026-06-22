package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/pkg/log"
)

func newObservedLogger(level zapcore.LevelEnabler) (*log.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return &log.Logger{Logger: zap.New(core)}, logs
}

func newDBConfig(t *testing.T, debug bool) *viper.Viper {
	t.Helper()

	conf := viper.New()
	conf.Set("data.db.user.driver", "sqlite")
	conf.Set("data.db.user.dsn", filepath.Join(t.TempDir(), "shiliu-test.db")+"?_busy_timeout=5000")
	conf.Set("data.db.user.debug", debug)
	return conf
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := repository.NewDB(newDBConfig(t, false), logger)
	closeDBOnCleanup(t, db)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	return db
}

func closeDBOnCleanup(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
	})
}

func setupRepository(t *testing.T) repository.UserRepository {
	t.Helper()

	db := openTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewUserRepository(repo)
}

func TestNewDB_OpenSQLite(t *testing.T) {
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := repository.NewDB(newDBConfig(t, false), logger)
	closeDBOnCleanup(t, db)

	var result int
	require.NoError(t, db.Raw("SELECT 1").Scan(&result).Error)
	assert.Equal(t, 1, result)
}

func TestNewDB_RejectsUnsupportedDriver(t *testing.T) {
	for _, driver := range []string{"mysql", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			conf := newDBConfig(t, false)
			conf.Set("data.db.user.driver", driver)
			logger, _ := newObservedLogger(zapcore.InfoLevel)

			assert.PanicsWithValue(t, "unsupported db driver \""+driver+"\": only sqlite is supported", func() {
				repository.NewDB(conf, logger)
			})
		})
	}
}

func TestNewDB_DebugOffByDefault(t *testing.T) {
	conf := newDBConfig(t, false)
	conf.Set("data.db.user.debug", nil)
	logger, logs := newObservedLogger(zapcore.InfoLevel)
	db := repository.NewDB(conf, logger)
	closeDBOnCleanup(t, db)

	require.NoError(t, db.Exec("SELECT 1").Error)

	assert.False(t, hasTraceLog(logs), "expected no SQL trace log when db debug is unset")
}

func TestNewDB_DebugEnabled(t *testing.T) {
	logger, logs := newObservedLogger(zapcore.InfoLevel)
	db := repository.NewDB(newDBConfig(t, true), logger)
	closeDBOnCleanup(t, db)

	require.NoError(t, db.Exec("SELECT 1").Error)

	assert.True(t, hasTraceLog(logs), "expected SQL trace log when db debug is true")
}

func hasTraceLog(logs *observer.ObservedLogs) bool {
	for _, entry := range logs.All() {
		if entry.Message == "trace" {
			return true
		}
	}
	return false
}

func TestUserRepository_Create(t *testing.T) {
	userRepo := setupRepository(t)

	ctx := context.Background()
	user := &model.User{
		UserId:    "123",
		Nickname:  "Test",
		Password:  "password",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := userRepo.Create(ctx, user)
	assert.NoError(t, err)
	assert.NotZero(t, user.Id)
}

func TestUserRepository_Update(t *testing.T) {
	userRepo := setupRepository(t)

	ctx := context.Background()
	user := &model.User{
		UserId:    "123",
		Nickname:  "Test",
		Password:  "password",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, userRepo.Create(ctx, user))

	user.Nickname = "Updated"
	err := userRepo.Update(ctx, user)
	assert.NoError(t, err)

	got, err := userRepo.GetByID(ctx, user.UserId)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Updated", got.Nickname)
}

func TestUserRepository_GetById(t *testing.T) {
	userRepo := setupRepository(t)

	ctx := context.Background()
	user := &model.User{
		UserId:   "123",
		Nickname: "Test",
		Password: "password",
		Email:    "test@example.com",
	}
	require.NoError(t, userRepo.Create(ctx, user))

	got, err := userRepo.GetByID(ctx, user.UserId)
	assert.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "123", got.UserId)
}

func TestUserRepository_GetByEmail(t *testing.T) {
	userRepo := setupRepository(t)

	ctx := context.Background()
	email := "test@example.com"
	user := &model.User{
		UserId:   "123",
		Nickname: "Test",
		Password: "password",
		Email:    email,
	}
	require.NoError(t, userRepo.Create(ctx, user))

	got, err := userRepo.GetByEmail(ctx, email)
	assert.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, email, got.Email)
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	userRepo := setupRepository(t)

	got, err := userRepo.GetByEmail(context.Background(), "missing@example.com")
	assert.NoError(t, err)
	assert.Nil(t, got)
}
