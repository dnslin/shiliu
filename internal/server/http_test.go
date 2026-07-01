package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "shiliu/api/v1"

	"shiliu/docs"
	"shiliu/internal/handler"
	"shiliu/internal/router"
	"shiliu/internal/service"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
	mock_service "shiliu/test/mocks/service"
)

func TestNewHTTPServerUsesAPIV1RoutePrefix(t *testing.T) {
	restoreGinMode(t)
	restoreSwaggerBasePath(t)
	server := NewHTTPServer(newTestRouterDeps())

	require.Equal(t, "/api/v1", docs.SwaggerInfo.BasePath)
	newRequest(server, http.MethodGet, "/api/v1/initialization").CodeIsNot(t, http.StatusNotFound)
	newRequest(server, http.MethodPost, "/api/v1/initialization").CodeIsNot(t, http.StatusNotFound)
	newRequest(server, http.MethodPost, "/api/v1/register").CodeEquals(t, http.StatusNotFound)
	newRequest(server, http.MethodPost, "/v1/initialization").CodeEquals(t, http.StatusNotFound)
}

func TestSwaggerDocumentsChangePasswordBounds(t *testing.T) {
	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(docs.SwaggerInfo.ReadDoc()), &spec))
	definitions := spec["definitions"].(map[string]interface{})
	changePassword := definitions["v1.ChangePasswordRequest"].(map[string]interface{})
	properties := changePassword["properties"].(map[string]interface{})
	newPassword := properties["newPassword"].(map[string]interface{})

	require.Equal(t, float64(12), newPassword["minLength"])
	require.Equal(t, float64(72), newPassword["maxLength"])
}

func TestSwaggerDocumentsAssignFeedFolderRequestContract(t *testing.T) {
	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(docs.SwaggerInfo.ReadDoc()), &spec))
	definitions := spec["definitions"].(map[string]interface{})
	assignFolder := definitions["v1.AssignFeedFolderRequest"].(map[string]interface{})
	properties := assignFolder["properties"].(map[string]interface{})
	folderID := properties["folderId"].(map[string]interface{})

	require.Contains(t, assignFolder["required"], "folderId")
	require.Equal(t, true, folderID["x-nullable"])
}

func TestSwaggerDocumentsContentPresetViewRoutes(t *testing.T) {
	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(docs.SwaggerInfo.ReadDoc()), &spec))
	paths := spec["paths"].(map[string]interface{})

	requireSwaggerOperation(t, paths, "/content-views/inbox", "List inbox content items", "Inbox view presets processing_status=unprocessed and accepts additional single-value filters.")
	requireSwaggerOperation(t, paths, "/content-views/later", "List later content items", "Later view presets mark=later and accepts additional single-value filters.")
	requireSwaggerOperation(t, paths, "/content-views/favorite", "List favorite content items", "Favorite view presets mark=favorite and accepts additional single-value filters.")
	requireSwaggerOperation(t, paths, "/content-views/completed", "List completed content items", "Completed view presets processing_status=completed and accepts additional single-value filters.")
	requireSwaggerOperation(t, paths, "/feeds/{id}/content-items", "List feed content items", "Feed detail view presets feed_id from the path and accepts additional single-value filters.")
	requireSwaggerMethodOperation(t, paths, "/content-items/{id}/processing-status", "put", "更新内容条目处理状态", "手动在未处理和已完成之间切换内容条目的处理状态，不改变内容标记或消费进度。", []interface{}{"内容条目模块"})
	requireSwaggerMethodOperation(t, paths, "/content-items/{id}/marks/{mark}", "put", "更新内容条目标记", "独立设置或取消稍后处理、收藏标记，不改变处理状态或消费进度。", []interface{}{"内容条目模块"})
	requireSwaggerMethodOperation(t, paths, "/content-items/{id}/audio-progress", "put", "更新音频播放进度", "仅持久化音频型内容条目的播放位置，不持久化文本阅读位置，也不改变处理状态。", []interface{}{"内容条目模块"})
	pathItem := paths["/content-items/{id}/ai-summary"].(map[string]interface{})
	operation := pathItem["post"].(map[string]interface{})
	responses := operation["responses"].(map[string]interface{})
	okResponse := responses["200"].(map[string]interface{})
	schema := okResponse["schema"].(map[string]interface{})
	require.Equal(t, "#/definitions/v1.AISummaryResponse", schema["$ref"])
}

func requireSwaggerMethodOperation(t *testing.T, paths map[string]interface{}, path string, method string, summary string, description string, tags []interface{}) {
	t.Helper()
	pathItem := paths[path].(map[string]interface{})
	operation := pathItem[method].(map[string]interface{})
	require.Equal(t, summary, operation["summary"])
	require.Equal(t, description, operation["description"])
	require.Equal(t, tags, operation["tags"])
}

func requireSwaggerOperation(t *testing.T, paths map[string]interface{}, path string, summary string, description string) {
	t.Helper()
	requireSwaggerMethodOperation(t, paths, path, "get", summary, description, []interface{}{"Content item module"})
}

func TestNewHTTPServerProtectsBusinessRoutesWithAuthorizationHeader(t *testing.T) {
	restoreGinMode(t)
	restoreSwaggerBasePath(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserService := mock_service.NewMockUserService(ctrl)
	deps := newTestRouterDepsWithUserService(mockUserService)
	token, err := deps.JWT.GenToken("123", time.Now().Add(time.Hour))
	require.NoError(t, err)
	server := NewHTTPServer(deps)

	newRequest(server, http.MethodGet, "/api/v1/user?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/user/password?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPost, "/api/v1/feeds?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/feeds?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodDelete, "/api/v1/feeds/42?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-items?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-items/42?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/content-items/42/processing-status?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/content-items/42/marks/later?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/content-items/42/audio-progress?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPost, "/api/v1/content-items/42/ai-summary?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/ai/service-config?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/ai/service-config?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPost, "/api/v1/ai/service-config/test?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/ai/auto-summary-config?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/ai/auto-summary-config?accessToken="+token).CodeEquals(t, http.StatusUnauthorized)
}

func TestNewHTTPServerRejectsMissingInvalidAndExpiredBearerTokens(t *testing.T) {
	restoreGinMode(t)
	restoreSwaggerBasePath(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserService := mock_service.NewMockUserService(ctrl)
	deps := newTestRouterDepsWithUserService(mockUserService)
	expiredToken, err := deps.JWT.GenToken("123", time.Now().Add(-time.Hour))
	require.NoError(t, err)
	server := NewHTTPServer(deps)

	newRequest(server, http.MethodGet, "/api/v1/user").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/user", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/user", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/user/password").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/user/password", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/user/password", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPost, "/api/v1/feeds").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPost, "/api/v1/feeds", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPost, "/api/v1/feeds", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/feeds").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/feeds", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/feeds", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodDelete, "/api/v1/feeds/42").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodDelete, "/api/v1/feeds/42", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodDelete, "/api/v1/feeds/42", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-items").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/content-items", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/content-items", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-items/42").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/content-items/42", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/content-items/42", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/content-items/42/processing-status").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/content-items/42/processing-status", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/content-items/42/processing-status", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/content-items/42/marks/later").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/content-items/42/marks/later", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/content-items/42/marks/later", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/content-items/42/audio-progress").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/content-items/42/audio-progress", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/content-items/42/audio-progress", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPost, "/api/v1/content-items/42/ai-summary").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPost, "/api/v1/content-items/42/ai-summary", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPost, "/api/v1/content-items/42/ai-summary", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/ai/service-config").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/ai/service-config", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/ai/service-config", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/ai/service-config").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/ai/service-config", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/ai/service-config", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPost, "/api/v1/ai/service-config/test").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPost, "/api/v1/ai/service-config/test", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPost, "/api/v1/ai/service-config/test", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/ai/auto-summary-config").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/ai/auto-summary-config", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodGet, "/api/v1/ai/auto-summary-config", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodPut, "/api/v1/ai/auto-summary-config").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/ai/auto-summary-config", "Authorization", "Bearer not-a-token").CodeEquals(t, http.StatusUnauthorized)
	newRequestWithHeader(server, http.MethodPut, "/api/v1/ai/auto-summary-config", "Authorization", "Bearer "+expiredToken).CodeEquals(t, http.StatusUnauthorized)
}

func TestNewHTTPServerProtectsContentPresetViewRoutes(t *testing.T) {
	restoreGinMode(t)
	restoreSwaggerBasePath(t)
	server := NewHTTPServer(newTestRouterDeps())

	newRequest(server, http.MethodGet, "/api/v1/content-views/inbox").CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-views/later").CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-views/favorite").CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/content-views/completed").CodeEquals(t, http.StatusUnauthorized)
	newRequest(server, http.MethodGet, "/api/v1/feeds/42/content-items").CodeEquals(t, http.StatusUnauthorized)
}

func TestNewHTTPServerAllowsValidBearerTokenOnBusinessRoutes(t *testing.T) {
	restoreGinMode(t)
	restoreSwaggerBasePath(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserService := mock_service.NewMockUserService(ctrl)
	deps := newTestRouterDepsWithUserService(mockUserService)
	token, err := deps.JWT.GenToken("123", time.Now().Add(time.Hour))
	require.NoError(t, err)
	mockUserService.EXPECT().GetProfile(gomock.Any(), "123").Return(&v1.GetProfileResponseData{Id: 123, Username: "testuser"}, nil)
	server := NewHTTPServer(deps)

	newRequestWithHeader(server, http.MethodGet, "/api/v1/user", "Authorization", "Bearer "+token).CodeEquals(t, http.StatusOK)
}

type routeResponse struct {
	code int
}

func newRequest(router http.Handler, method string, path string) routeResponse {
	request := httptest.NewRequest(method, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return routeResponse{code: response.Code}
}

func newRequestWithHeader(router http.Handler, method string, path string, key string, value string) routeResponse {
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set(key, value)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return routeResponse{code: response.Code}
}

func (r routeResponse) CodeEquals(t *testing.T, want int) {
	t.Helper()
	require.Equal(t, want, r.code)
}

func (r routeResponse) CodeIsNot(t *testing.T, unwanted int) {
	t.Helper()
	require.NotEqual(t, unwanted, r.code)
}

func restoreGinMode(t *testing.T) {
	t.Helper()
	mode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() {
		gin.SetMode(mode)
	})
}

func restoreSwaggerBasePath(t *testing.T) {
	t.Helper()
	basePath := docs.SwaggerInfo.BasePath
	t.Cleanup(func() {
		docs.SwaggerInfo.BasePath = basePath
	})
}

func newTestRouterDeps() router.RouterDeps {
	return newTestRouterDepsWithUserService(nil)
}

func newTestRouterDepsWithUserService(userService service.UserService) router.RouterDeps {
	conf := viper.New()
	conf.Set("env", "test")
	conf.Set("http.host", "127.0.0.1")
	conf.Set("http.port", 0)
	conf.Set("security.jwt.key", "test-secret")

	logger := &log.Logger{Logger: zap.NewNop()}
	baseHandler := handler.NewHandler(logger)

	return router.RouterDeps{
		Logger:                   logger,
		Config:                   conf,
		JWT:                      jwt.NewJwt(conf),
		UserHandler:              handler.NewUserHandler(baseHandler, userService),
		FeedHandler:              handler.NewFeedHandler(baseHandler, nil),
		ContentItemHandler:       handler.NewContentItemHandler(baseHandler, nil),
		AIServiceConfigHandler:   handler.NewAIServiceConfigHandler(baseHandler, nil),
		AutoSummaryConfigHandler: handler.NewAutoSummaryConfigHandler(baseHandler, nil),
	}
}
