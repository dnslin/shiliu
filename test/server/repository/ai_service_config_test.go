package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"shiliu/internal/model"
	"shiliu/internal/repository"
)

func setupAIServiceConfigRepository(t *testing.T) repository.AIServiceConfigRepository {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewAIServiceConfigRepository(repo)
}

func TestAIServiceConfigRepository_SaveAndGet(t *testing.T) {
	configRepo := setupAIServiceConfigRepository(t)
	ctx := context.Background()
	config := &model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	}

	require.NoError(t, configRepo.Save(ctx, config))

	got, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotZero(t, got.Id)
	assert.Equal(t, "https://api.example.com/v1", got.APIBaseURL)
	assert.Equal(t, "gpt-4.1-mini", got.Model)
	assert.Equal(t, "sk-secret-value", got.APIKey)
}

func TestAIServiceConfigRepository_SaveReplacesExistingConfig(t *testing.T) {
	configRepo := setupAIServiceConfigRepository(t)
	ctx := context.Background()
	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{
		APIBaseURL: "https://api.old.example.com/v1",
		Model:      "old-model",
		APIKey:     "old-secret",
	}))

	require.NoError(t, configRepo.Save(ctx, &model.AIServiceConfig{
		APIBaseURL: "https://api.new.example.com/v1",
		Model:      "new-model",
		APIKey:     "new-secret",
	}))

	got, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "https://api.new.example.com/v1", got.APIBaseURL)
	assert.Equal(t, "new-model", got.Model)
	assert.Equal(t, "new-secret", got.APIKey)
}

func TestAIServiceConfigRepository_SaveDoesNotLeakAPIKeyToSQLTraceLogs(t *testing.T) {
	conf := newDBConfig(t, true)
	runMigrations(t, conf.GetString("data.db.user.dsn"), "up")
	logger, logs := newObservedLogger(zapcore.InfoLevel)
	db := repository.NewDB(conf, logger)
	closeDBOnCleanup(t, db)
	repo := repository.NewRepository(logger, db)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	secret := "sk-sql-log-secret"

	require.NoError(t, configRepo.Save(context.Background(), &model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     secret,
	}))

	var loggedSQL strings.Builder
	for _, entry := range logs.All() {
		sql, ok := entry.ContextMap()["sql"].(string)
		if ok {
			loggedSQL.WriteString(sql)
			loggedSQL.WriteByte('\n')
		}
	}
	assert.NotContains(t, loggedSQL.String(), secret)
}
