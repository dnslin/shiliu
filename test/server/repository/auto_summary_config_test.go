package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"shiliu/internal/model"
	"shiliu/internal/repository"
)

func setupAutoSummaryConfigRepository(t *testing.T) repository.AutoSummaryConfigRepository {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewAutoSummaryConfigRepository(repo)
}

func TestAutoSummaryConfigRepository_GetReturnsNilWhenMissing(t *testing.T) {
	configRepo := setupAutoSummaryConfigRepository(t)

	got, err := configRepo.Get(context.Background())

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestAutoSummaryConfigRepository_SaveAndGet(t *testing.T) {
	configRepo := setupAutoSummaryConfigRepository(t)
	ctx := context.Background()
	enabledAt := time.Now().UTC().Truncate(time.Second)
	config := &model.AutoSummaryConfig{
		Enabled:          true,
		ContentTypeScope: model.AutoSummaryContentTypeScopeAll,
		EnabledAt:        &enabledAt,
	}

	require.NoError(t, configRepo.Save(ctx, config))

	got, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotZero(t, got.Id)
	assert.True(t, got.Enabled)
	assert.Equal(t, model.AutoSummaryContentTypeScopeAll, got.ContentTypeScope)
	require.NotNil(t, got.EnabledAt)
	assert.WithinDuration(t, enabledAt, got.EnabledAt.UTC(), time.Second)
}

func TestAutoSummaryConfigRepository_SaveReplacesExistingConfig(t *testing.T) {
	configRepo := setupAutoSummaryConfigRepository(t)
	ctx := context.Background()
	firstEnabledAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, configRepo.Save(ctx, &model.AutoSummaryConfig{
		Enabled:          true,
		ContentTypeScope: model.AutoSummaryContentTypeScopeText,
		EnabledAt:        &firstEnabledAt,
	}))

	require.NoError(t, configRepo.Save(ctx, &model.AutoSummaryConfig{
		Enabled:          false,
		ContentTypeScope: model.AutoSummaryContentTypeScopeAudio,
		EnabledAt:        nil,
	}))

	got, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.Enabled)
	assert.Equal(t, model.AutoSummaryContentTypeScopeAudio, got.ContentTypeScope)
	assert.Nil(t, got.EnabledAt)
}
