package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"shiliu/internal/handler"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

type handlerRecordingAIConfigTester struct {
	calls      int
	lastConfig model.AIServiceConfig
}

func (t *handlerRecordingAIConfigTester) TestAIServiceConfig(ctx context.Context, config model.AIServiceConfig) error {
	t.calls++
	t.lastConfig = config
	return nil
}

func TestAIServiceConfigHandler_SaveReadAndTestDoNotEchoAPIKey(t *testing.T) {
	r, tester := newAIServiceConfigHandlerTestHarness(t)
	secret := "sk-handler-secret"

	saved := newHttpExcept(t, r).PUT("/ai/service-config").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]string{
			"apiBaseUrl": "https://api.example.com/v1",
			"model":      "gpt-4.1-mini",
			"apiKey":     secret,
		}).
		Expect().
		Status(http.StatusOK)
	saved.Body().NotContains(secret)
	saved.JSON().Object().Value("data").Object().Value("apiKeyConfigured").IsEqual(true)

	read := newHttpExcept(t, r).GET("/ai/service-config").
		Expect().
		Status(http.StatusOK)
	read.Body().NotContains(secret)
	readData := read.JSON().Object().Value("data").Object()
	readData.Value("configured").IsEqual(true)
	readData.Value("apiBaseUrl").IsEqual("https://api.example.com/v1")
	readData.Value("model").IsEqual("gpt-4.1-mini")
	readData.Value("apiKeyConfigured").IsEqual(true)

	tested := newHttpExcept(t, r).POST("/ai/service-config/test").
		Expect().
		Status(http.StatusOK)
	tested.Body().NotContains(secret)
	tested.JSON().Object().Value("data").Object().Value("ok").IsEqual(true)
	if tester.calls != 1 {
		t.Fatalf("expected one connectivity test call, got %d", tester.calls)
	}
	if tester.lastConfig.APIKey != secret {
		t.Fatalf("tester did not receive server-only api key")
	}
}

func newAIServiceConfigHandlerTestHarness(t *testing.T) (*gin.Engine, *handlerRecordingAIConfigTester) {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "ai-config-handler.db") + "?_busy_timeout=5000"
	conf.Set("data.db.user.driver", "sqlite")
	conf.Set("data.db.user.dsn", dsn)
	conf.Set("data.db.user.debug", false)
	runContentViewHandlerMigrations(t, dsn)
	db := repository.NewDB(conf, logger)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sql db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sql db: %v", err)
		}
	})
	repo := repository.NewRepository(logger, db)
	configRepo := repository.NewAIServiceConfigRepository(repo)
	tester := &handlerRecordingAIConfigTester{}
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	aiHandler := handler.NewAIServiceConfigHandler(hdl, service.NewAIServiceConfigService(base, configRepo, tester))
	r := gin.New()
	r.GET("/ai/service-config", aiHandler.GetConfig)
	r.PUT("/ai/service-config", aiHandler.SaveConfig)
	r.POST("/ai/service-config/test", aiHandler.TestConfig)
	return r, tester
}
