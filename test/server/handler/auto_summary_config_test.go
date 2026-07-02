package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	v1 "shiliu/api/v1"
	"shiliu/internal/handler"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestAutoSummaryConfigHandler_ReadSaveAndValidate(t *testing.T) {
	r, aiConfigRepo := newAutoSummaryConfigHandlerTestHarness(t)
	ctx := context.Background()

	readDefault := newHttpExcept(t, r).GET("/ai/auto-summary-config").
		Expect().
		Status(http.StatusOK)
	defaultData := readDefault.JSON().Object().Value("data").Object()
	defaultData.Value("enabled").IsEqual(false)
	defaultData.Value("contentTypeScope").IsEqual("all")
	defaultData.Value("enabledAt").IsNull()

	missingAIConfig := newHttpExcept(t, r).PUT("/ai/auto-summary-config").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]interface{}{
			"enabled":          true,
			"contentTypeScope": "text",
		}).
		Expect().
		Status(http.StatusNotFound)
	missingAIConfig.JSON().Object().Value("code").IsEqual(5001)

	requireNoError(t, aiConfigRepo.Save(ctx, &model.AIServiceConfig{APIBaseURL: "https://api.example.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-secret"}))

	saved := newHttpExcept(t, r).PUT("/ai/auto-summary-config").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]interface{}{
			"enabled":          true,
			"contentTypeScope": "text",
		}).
		Expect().
		Status(http.StatusOK)
	savedData := saved.JSON().Object().Value("data").Object()
	savedData.Value("enabled").IsEqual(true)
	savedData.Value("contentTypeScope").IsEqual("text")
	savedData.Value("enabledAt").NotNull()

	readSaved := newHttpExcept(t, r).GET("/ai/auto-summary-config").
		Expect().
		Status(http.StatusOK)
	readSavedData := readSaved.JSON().Object().Value("data").Object()
	readSavedData.Value("enabled").IsEqual(true)
	readSavedData.Value("contentTypeScope").IsEqual("text")
	readSavedData.Value("enabledAt").NotNull()

	newHttpExcept(t, r).PUT("/ai/auto-summary-config").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]interface{}{
			"contentTypeScope": "all",
		}).
		Expect().
		Status(http.StatusBadRequest)

	readAfterMissingEnabled := newHttpExcept(t, r).GET("/ai/auto-summary-config").
		Expect().
		Status(http.StatusOK)
	readAfterMissingEnabledData := readAfterMissingEnabled.JSON().Object().Value("data").Object()
	readAfterMissingEnabledData.Value("enabled").IsEqual(true)
	readAfterMissingEnabledData.Value("contentTypeScope").IsEqual("text")
	readAfterMissingEnabledData.Value("enabledAt").NotNull()

	newHttpExcept(t, r).PUT("/ai/auto-summary-config").
		WithHeader("Content-Type", "application/json").
		WithJSON(map[string]interface{}{
			"enabled":          false,
			"contentTypeScope": "video",
		}).
		Expect().
		Status(http.StatusBadRequest)
}

func newAutoSummaryConfigHandlerTestHarness(t *testing.T) (*gin.Engine, repository.AIServiceConfigRepository) {
	t.Helper()
	conf := viper.New()
	dsn := filepath.Join(t.TempDir(), "auto-summary-config-handler.db") + "?_busy_timeout=5000"
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
	configRepo := repository.NewAutoSummaryConfigRepository(repo)
	aiConfigRepo := repository.NewAIServiceConfigRepository(repo)
	base := service.NewService(repository.NewTransaction(repo), logger, nil, nil)
	autoHandler := handler.NewAutoSummaryConfigHandler(hdl, service.NewAutoSummaryConfigService(base, configRepo, aiConfigRepo))
	r := gin.New()
	r.GET("/ai/auto-summary-config", autoHandler.GetConfig)
	r.PUT("/ai/auto-summary-config", autoHandler.SaveConfig)
	return r, aiConfigRepo
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

var _ = v1.ErrSuccess
