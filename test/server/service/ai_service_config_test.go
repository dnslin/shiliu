package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

type recordingAIConfigTester struct {
	calls      int
	lastConfig model.AIServiceConfig
}

func (t *recordingAIConfigTester) TestAIServiceConfig(ctx context.Context, config model.AIServiceConfig) error {
	t.calls++
	t.lastConfig = config
	return nil
}

func TestAIServiceConfigService_SaveConfigTrimsPersistsAndDoesNotTestConnection(t *testing.T) {
	ctx := context.Background()
	aiService, configRepo, tester := newAIServiceConfigIntegrationService(t)

	response, err := aiService.SaveConfig(ctx, &v1.SaveAIServiceConfigRequest{
		APIBaseURL: "  https://api.example.com/v1/  ",
		Model:      "  gpt-4.1-mini  ",
		APIKey:     "  sk-secret-value  ",
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "https://api.example.com/v1", response.APIBaseURL)
	assert.Equal(t, "gpt-4.1-mini", response.Model)
	assert.True(t, response.APIKeyConfigured)
	assert.Zero(t, tester.calls, "saving config must not force a connectivity test")

	stored, err := configRepo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "https://api.example.com/v1", stored.APIBaseURL)
	assert.Equal(t, "gpt-4.1-mini", stored.Model)
	assert.Equal(t, "sk-secret-value", stored.APIKey)
}

func TestAIServiceConfigService_SaveConfigRejectsInvalidBaseURL(t *testing.T) {
	invalidBaseURLs := []string{
		"api.example.com/v1",
		"ftp://api.example.com/v1",
		"https://user:pass@api.example.com/v1",
		"https://api.example.com/v1?apiKey=leak",
		"https://api.example.com/v1?",
		"https://api.example.com/v1#fragment",
		"http://127.0.0.1:8080/v1",
		"http://localhost:8080/v1",
		"http://169.254.169.254/v1",
		"http://10.0.0.1/v1",
		"http://[::1]:8080/v1",
	}
	for _, baseURL := range invalidBaseURLs {
		t.Run(baseURL, func(t *testing.T) {
			aiService, _, _ := newAIServiceConfigIntegrationService(t)

			response, err := aiService.SaveConfig(context.Background(), &v1.SaveAIServiceConfigRequest{
				APIBaseURL: baseURL,
				Model:      "gpt-4.1-mini",
				APIKey:     "sk-secret-value",
			})

			require.ErrorIs(t, err, v1.ErrBadRequest)
			assert.Nil(t, response)
		})
	}
}

func TestAIServiceConfigService_SaveConfigRejectsBlankModelOrAPIKey(t *testing.T) {
	cases := []struct {
		name   string
		model  string
		apiKey string
	}{
		{name: "blank model", model: "   ", apiKey: "sk-secret-value"},
		{name: "blank api key", model: "gpt-4.1-mini", apiKey: "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aiService, _, _ := newAIServiceConfigIntegrationService(t)

			response, err := aiService.SaveConfig(context.Background(), &v1.SaveAIServiceConfigRequest{
				APIBaseURL: "https://api.example.com/v1",
				Model:      tc.model,
				APIKey:     tc.apiKey,
			})

			require.ErrorIs(t, err, v1.ErrBadRequest)
			assert.Nil(t, response)
		})
	}
}

func TestAIServiceConfigService_GetConfigReturnsConfiguredStateWithoutSecret(t *testing.T) {
	ctx := context.Background()
	aiService, _, _ := newAIServiceConfigIntegrationService(t)
	_, err := aiService.SaveConfig(ctx, &v1.SaveAIServiceConfigRequest{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	})
	require.NoError(t, err)

	response, err := aiService.GetConfig(ctx)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Configured)
	assert.True(t, response.APIKeyConfigured)
	assert.Equal(t, "https://api.example.com/v1", response.APIBaseURL)
	assert.Equal(t, "gpt-4.1-mini", response.Model)
	body, err := json.Marshal(response)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "sk-secret-value")
	assert.NotContains(t, strings.ToLower(string(body)), "apikey\":\"")
}

func TestAIServiceConfigService_TestConfigUsesStoredSecretOnlyOnDemand(t *testing.T) {
	ctx := context.Background()
	aiService, _, tester := newAIServiceConfigIntegrationService(t)
	_, err := aiService.SaveConfig(ctx, &v1.SaveAIServiceConfigRequest{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	})
	require.NoError(t, err)

	response, err := aiService.TestConfig(ctx)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.OK)
	assert.Equal(t, 1, tester.calls)
	assert.Equal(t, "https://api.example.com/v1", tester.lastConfig.APIBaseURL)
	assert.Equal(t, "gpt-4.1-mini", tester.lastConfig.Model)
	assert.Equal(t, "sk-secret-value", tester.lastConfig.APIKey)
}

func TestAIServiceConfigService_TestConfigRequiresSavedConfig(t *testing.T) {
	ctx := context.Background()
	aiService, _, tester := newAIServiceConfigIntegrationService(t)

	response, err := aiService.TestConfig(ctx)

	require.ErrorIs(t, err, v1.ErrAIConfigMissing)
	assert.Nil(t, response)
	assert.Zero(t, tester.calls)
}

func newAIServiceConfigIntegrationService(t *testing.T) (service.AIServiceConfigService, repository.AIServiceConfigRepository, *recordingAIConfigTester) {
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
	configRepo := repository.NewAIServiceConfigRepository(repo)
	tester := &recordingAIConfigTester{}
	srv := service.NewService(repository.NewTransaction(repo), logger, sf, j)
	return service.NewAIServiceConfigService(srv, configRepo, tester), configRepo, tester
}
