package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	v1 "shiliu/api/v1"
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

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func runMigrations(t *testing.T, dsn string, direction string) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "migration-test.yml")
	content := fmt.Sprintf("data:\n  db:\n    user:\n      driver: sqlite\n      dsn: %q\n      debug: false\n", dsn)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	cmd := exec.Command("go", "run", "./cmd/migration", "-conf", configPath, "-direction", direction, "-path", "migrations")
	cmd.Dir = repoRoot(t)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func openMigratedTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	conf := newDBConfig(t, false)
	runMigrations(t, conf.GetString("data.db.user.dsn"), "up")

	logger, _ := newObservedLogger(zapcore.InfoLevel)
	db := repository.NewDB(conf, logger)
	closeDBOnCleanup(t, db)
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

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewUserRepository(repo)
}

func openSQLDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})
	return db
}

func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return name == tableName
}

func indexExists(t *testing.T, db *sql.DB, indexName string) bool {
	t.Helper()

	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?", indexName).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return name == indexName
}

func contentItemColumnExists(t *testing.T, db *sql.DB, columnName string) bool {
	t.Helper()

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('content_items') WHERE name = ?`, columnName).Scan(&count)
	require.NoError(t, err)
	return count == 1
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

func TestUsersMigration_CreatesUsersTable(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "migration-create.db") + "?_busy_timeout=5000"
	runMigrations(t, dsn, "up")

	db := openSQLDB(t, dsn)
	assert.True(t, tableExists(t, db, "users"))
}

func TestContentItemStateMarksMigration_DownRemovesOnlyLatestBoundary(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "migration-down.db") + "?_busy_timeout=5000"
	runMigrations(t, dsn, "up")
	runMigrations(t, dsn, "down")

	db := openSQLDB(t, dsn)
	assert.True(t, tableExists(t, db, "users"))
	assert.True(t, indexExists(t, db, "idx_users_singleton"))
	assert.True(t, tableExists(t, db, "feeds"))
	assert.True(t, tableExists(t, db, "content_items"))
	assert.True(t, tableExists(t, db, "shiliu_migration_baseline"))
	assert.False(t, indexExists(t, db, "idx_content_items_processing_status"))
	assert.False(t, indexExists(t, db, "idx_content_items_marked_later"))
	assert.False(t, indexExists(t, db, "idx_content_items_favorited"))
	assert.False(t, contentItemColumnExists(t, db, "processing_status"))
	assert.False(t, contentItemColumnExists(t, db, "marked_later"))
	assert.False(t, contentItemColumnExists(t, db, "favorited"))
}

func TestUserRepository_HasAnyReportsAccountPresence(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	initialized, err := userRepo.HasAny(ctx)
	require.NoError(t, err)
	assert.False(t, initialized)

	require.NoError(t, userRepo.Create(ctx, &model.User{Username: "first", PasswordHash: "hash-v1"}))

	initialized, err = userRepo.HasAny(ctx)
	require.NoError(t, err)
	assert.True(t, initialized)
}

func TestUserRepository_CreateSecondAccountFails(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	require.NoError(t, userRepo.Create(ctx, &model.User{Username: "first", PasswordHash: "hash-v1"}))
	err := userRepo.Create(ctx, &model.User{Username: "second", PasswordHash: "hash-v2"})

	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrDuplicatedKey)
}

func TestUserRepository_CreateAndGetByUsername(t *testing.T) {
	userRepo := setupRepository(t)

	ctx := context.Background()
	user := &model.User{
		Username:     "testuser",
		PasswordHash: "hash-v1",
	}

	require.NoError(t, userRepo.Create(ctx, user))
	require.NotZero(t, user.Id)

	got, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, user.Id, got.Id)
	assert.Equal(t, "testuser", got.Username)
	assert.Equal(t, "hash-v1", got.PasswordHash)
	assert.Zero(t, got.FailedLoginCount)
	assert.Nil(t, got.LockedUntil)
}

func TestUserRepository_GetByUsernameMissingReturnsNil(t *testing.T) {
	userRepo := setupRepository(t)

	got, err := userRepo.GetByUsername(context.Background(), "missing")

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserRepository_GetOnlyReturnsSingletonAccount(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	got, err := userRepo.GetOnly(ctx)
	require.NoError(t, err)
	assert.Nil(t, got)

	user := &model.User{Username: "only-account", PasswordHash: "hash-v1"}
	require.NoError(t, userRepo.Create(ctx, user))

	got, err = userRepo.GetOnly(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, user.Id, got.Id)
	assert.Equal(t, user.Username, got.Username)
}

func TestUserRepository_CreateDuplicateUsernameFails(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	require.NoError(t, userRepo.Create(ctx, &model.User{Username: "duplicate", PasswordHash: "hash-v1"}))
	err := userRepo.Create(ctx, &model.User{Username: "duplicate", PasswordHash: "hash-v2"})

	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrDuplicatedKey)
}

func TestUserRepository_UpdatePersistsAuthFields(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	user := &model.User{
		Username:     "updatable",
		PasswordHash: "hash-v1",
	}
	require.NoError(t, userRepo.Create(ctx, user))

	lockedUntil := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)
	user.PasswordHash = "hash-v2"
	user.FailedLoginCount = 5
	user.LockedUntil = &lockedUntil
	require.NoError(t, userRepo.Update(ctx, user))

	got, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "hash-v2", got.PasswordHash)
	assert.Equal(t, 5, got.FailedLoginCount)
	require.NotNil(t, got.LockedUntil)
	assert.WithinDuration(t, lockedUntil, got.LockedUntil.UTC(), time.Second)
}

func TestUserRepository_UpdatePasswordPersistsHashOnly(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	lockedUntil := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)
	user := &model.User{
		Username:         "password-only",
		PasswordHash:     "hash-v1",
		FailedLoginCount: 4,
		LockedUntil:      &lockedUntil,
	}
	require.NoError(t, userRepo.Create(ctx, user))

	require.NoError(t, userRepo.UpdatePassword(ctx, user.Id, "hash-v1", "hash-v2"))

	got, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "hash-v2", got.PasswordHash)
	assert.Equal(t, 4, got.FailedLoginCount)
	require.NotNil(t, got.LockedUntil)
	assert.WithinDuration(t, lockedUntil, got.LockedUntil.UTC(), time.Second)
}

func TestUserRepository_UpdatePasswordRejectsInvalidOrMissingUser(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	assert.ErrorIs(t, userRepo.UpdatePassword(ctx, 0, "hash-v1", "hash-v2"), v1.ErrBadRequest)
	assert.ErrorIs(t, userRepo.UpdatePassword(ctx, 123456, "", "hash-v2"), v1.ErrBadRequest)
	assert.ErrorIs(t, userRepo.UpdatePassword(ctx, 123456, "hash-v1", ""), v1.ErrBadRequest)
	assert.ErrorIs(t, userRepo.UpdatePassword(ctx, 123456, "hash-v1", "hash-v2"), v1.ErrNotFound)
}

func TestUserRepository_UpdatePasswordRejectsStaleCurrentHashWithoutUpdating(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	user := &model.User{
		Username:     "stale-password",
		PasswordHash: "hash-v1",
	}
	require.NoError(t, userRepo.Create(ctx, user))

	err := userRepo.UpdatePassword(ctx, user.Id, "different-current-hash", "hash-v2")

	assert.ErrorIs(t, err, v1.ErrInvalidCredentials)
	got, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "hash-v1", got.PasswordHash)
}

func TestUserRepository_ClearLoginFailuresPreservesActiveLock(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	user := &model.User{Username: "clear-locked", PasswordHash: "hash-v1"}
	require.NoError(t, userRepo.Create(ctx, user))
	lockedUntil := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)
	user.FailedLoginCount = 5
	user.LockedUntil = &lockedUntil
	require.NoError(t, userRepo.Update(ctx, user))

	got, err := userRepo.ClearLoginFailures(ctx, user.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 5, got.FailedLoginCount)
	require.NotNil(t, got.LockedUntil)
	assert.WithinDuration(t, lockedUntil, got.LockedUntil.UTC(), time.Second)
}

func TestUserRepository_ClearLoginFailuresClearsExpiredFailureState(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	user := &model.User{Username: "clear-expired", PasswordHash: "hash-v1"}
	require.NoError(t, userRepo.Create(ctx, user))
	lockedUntil := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	user.FailedLoginCount = 3
	user.LockedUntil = &lockedUntil
	require.NoError(t, userRepo.Update(ctx, user))

	got, err := userRepo.ClearLoginFailures(ctx, user.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Zero(t, got.FailedLoginCount)
	assert.Nil(t, got.LockedUntil)
}

func TestUserRepository_RecordLoginFailureIncrementsAndLocksAtomically(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	user := &model.User{
		Username:     "lockable",
		PasswordHash: "hash-v1",
	}
	require.NoError(t, userRepo.Create(ctx, user))
	lockedUntil := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)

	for i := 1; i <= 5; i++ {
		got, err := userRepo.RecordLoginFailure(ctx, user.Id, 5, lockedUntil)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, i, got.FailedLoginCount)
	}

	got, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 5, got.FailedLoginCount)
	require.NotNil(t, got.LockedUntil)
	assert.WithinDuration(t, lockedUntil, got.LockedUntil.UTC(), time.Second)
}

func TestUserRepository_RecordLoginFailureConcurrentAttemptsLockAccount(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()
	user := &model.User{Username: "concurrent-lockable", PasswordHash: "hash-v1"}
	require.NoError(t, userRepo.Create(ctx, user))
	lockedUntil := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)

	const attempts = 5
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := userRepo.RecordLoginFailure(ctx, user.Id, attempts, lockedUntil)
			errCh <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	got, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, attempts, got.FailedLoginCount)
	require.NotNil(t, got.LockedUntil)
	assert.WithinDuration(t, lockedUntil, got.LockedUntil.UTC(), time.Second)
}

func TestUserRepository_UpdateZeroIDDoesNotInsert(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	err := userRepo.Update(ctx, &model.User{Username: "no-id", PasswordHash: "hash-v1"})

	require.Error(t, err)
	got, err := userRepo.GetByUsername(ctx, "no-id")
	require.NoError(t, err)
	assert.Nil(t, got, "update with zero id must not create a row")
}

func TestUserRepository_UpdateMissingIDDoesNotInsert(t *testing.T) {
	userRepo := setupRepository(t)
	ctx := context.Background()

	err := userRepo.Update(ctx, &model.User{Id: 999, Username: "ghost", PasswordHash: "hash-v1"})

	require.ErrorIs(t, err, v1.ErrNotFound)
	got, err := userRepo.GetByUsername(ctx, "ghost")
	require.NoError(t, err)
	assert.Nil(t, got, "update with a missing id must not create a row")
}
