package server

import (
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
		Logger:      logger,
		Config:      conf,
		JWT:         jwt.NewJwt(conf),
		UserHandler: handler.NewUserHandler(baseHandler, userService),
	}
}
