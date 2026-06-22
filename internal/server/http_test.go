package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"shiliu/docs"
	"shiliu/internal/handler"
	"shiliu/internal/router"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
)

func TestNewHTTPServerUsesAPIV1RoutePrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewHTTPServer(newTestRouterDeps())

	require.Equal(t, "/api/v1", docs.SwaggerInfo.BasePath)
	newRequest(server, http.MethodPost, "/api/v1/register").CodeEquals(t, http.StatusBadRequest)
	newRequest(server, http.MethodPost, "/v1/register").CodeEquals(t, http.StatusNotFound)
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

func (r routeResponse) CodeEquals(t *testing.T, want int) {
	t.Helper()
	require.Equal(t, want, r.code)
}

func newTestRouterDeps() router.RouterDeps {
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
		UserHandler: handler.NewUserHandler(baseHandler, nil),
	}
}
