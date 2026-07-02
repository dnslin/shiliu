package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestAutoSummaryConfigService_GetConfigReturnsDisabledDefault(t *testing.T) {
	autoService, _, _ := newAutoSummaryConfigIntegrationService(t)

	response, err := autoService.GetConfig(context.Background())

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.False(t, response.Enabled)
	assert.Equal(t, "all", response.ContentTypeScope)
	assert.Nil(t, response.EnabledAt)
}

func TestAutoSummaryConfigService_EnableRequiresAIServiceConfig(t *testing.T) {
	autoService, _, _ := newAutoSummaryConfigIntegrationService(t)

	response, err := autoService.SaveConfig(context.Background(), &v1.SaveAutoSummaryConfigRequest{
		Enabled:          boolPtr(true),
		ContentTypeScope: "all",
	})

	require.ErrorIs(t, err, v1.ErrAIConfigMissing)
	assert.Nil(t, response)
}

func TestAutoSummaryConfigService_EnablePersistsScopeAndEffectiveTime(t *testing.T) {
	ctx := context.Background()
	autoService, configRepo, aiConfigRepo := newAutoSummaryConfigIntegrationService(t)
	require.NoError(t, aiConfigRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	before := time.Now().UTC()

	response, err := autoService.SaveConfig(ctx, &v1.SaveAutoSummaryConfigRequest{
		Enabled:          boolPtr(true),
		ContentTypeScope: "text",
	})
	after := time.Now().UTC()

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Enabled)
	assert.Equal(t, "text", response.ContentTypeScope)
	require.NotNil(t, response.EnabledAt)
	assert.False(t, response.EnabledAt.Before(before.Add(-time.Second)))
	assert.False(t, response.EnabledAt.After(after.Add(time.Second)))

	stored, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.True(t, stored.Enabled)
	assert.Equal(t, model.AutoSummaryContentTypeScopeText, stored.ContentTypeScope)
	require.NotNil(t, stored.EnabledAt)
	assert.WithinDuration(t, *response.EnabledAt, stored.EnabledAt.UTC(), time.Second)
}

func TestAutoSummaryConfigService_DisableDoesNotRequireAIServiceConfig(t *testing.T) {
	autoService, configRepo, _ := newAutoSummaryConfigIntegrationService(t)

	response, err := autoService.SaveConfig(context.Background(), &v1.SaveAutoSummaryConfigRequest{
		Enabled:          boolPtr(false),
		ContentTypeScope: "audio",
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.False(t, response.Enabled)
	assert.Equal(t, "audio", response.ContentTypeScope)
	assert.Nil(t, response.EnabledAt)

	stored, err := configRepo.Get(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.False(t, stored.Enabled)
	assert.Equal(t, model.AutoSummaryContentTypeScopeAudio, stored.ContentTypeScope)
	assert.Nil(t, stored.EnabledAt)
}

func TestAutoSummaryConfigService_EnableKeepsEffectiveTimeForSameScopeAndRefreshesForScopeChange(t *testing.T) {
	ctx := context.Background()
	autoService, configRepo, aiConfigRepo := newAutoSummaryConfigIntegrationService(t)
	require.NoError(t, aiConfigRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))
	first, err := autoService.SaveConfig(ctx, &v1.SaveAutoSummaryConfigRequest{Enabled: boolPtr(true), ContentTypeScope: "text"})
	require.NoError(t, err)
	require.NotNil(t, first.EnabledAt)
	time.Sleep(10 * time.Millisecond)

	second, err := autoService.SaveConfig(ctx, &v1.SaveAutoSummaryConfigRequest{Enabled: boolPtr(true), ContentTypeScope: "text"})
	require.NoError(t, err)
	require.NotNil(t, second.EnabledAt)
	assert.True(t, second.EnabledAt.Equal(*first.EnabledAt))

	time.Sleep(10 * time.Millisecond)
	third, err := autoService.SaveConfig(ctx, &v1.SaveAutoSummaryConfigRequest{Enabled: boolPtr(true), ContentTypeScope: "audio"})
	require.NoError(t, err)
	require.NotNil(t, third.EnabledAt)
	assert.True(t, third.EnabledAt.After(*second.EnabledAt))

	stored, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, model.AutoSummaryContentTypeScopeAudio, stored.ContentTypeScope)
	assert.WithinDuration(t, *third.EnabledAt, stored.EnabledAt.UTC(), time.Second)
}

func TestAutoSummaryConfigService_RejectsInvalidRequest(t *testing.T) {
	cases := []struct {
		name string
		req  *v1.SaveAutoSummaryConfigRequest
	}{
		{name: "nil", req: nil},
		{name: "missing enabled", req: &v1.SaveAutoSummaryConfigRequest{ContentTypeScope: "all"}},
		{name: "blank scope", req: &v1.SaveAutoSummaryConfigRequest{Enabled: boolPtr(true)}},
		{name: "unknown scope", req: &v1.SaveAutoSummaryConfigRequest{Enabled: boolPtr(false), ContentTypeScope: "video"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			autoService, _, _ := newAutoSummaryConfigIntegrationService(t)

			response, err := autoService.SaveConfig(context.Background(), tc.req)

			require.ErrorIs(t, err, v1.ErrBadRequest)
			assert.Nil(t, response)
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func newAutoSummaryConfigIntegrationService(t *testing.T) (service.AutoSummaryConfigService, repository.AutoSummaryConfigRepository, repository.AIServiceConfigRepository) {
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
	configRepo := repository.NewAutoSummaryConfigRepository(repo)
	aiConfigRepo := repository.NewAIServiceConfigRepository(repo)
	srv := service.NewService(repository.NewTransaction(repo), logger, sf, j)
	return service.NewAutoSummaryConfigService(srv, configRepo, aiConfigRepo), configRepo, aiConfigRepo
}
