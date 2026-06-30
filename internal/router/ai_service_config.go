package router

import (
	"github.com/gin-gonic/gin"

	"shiliu/internal/middleware"
)

func InitAIServiceConfigRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.GET("/ai/service-config", deps.AIServiceConfigHandler.GetConfig)
		strictAuthRouter.PUT("/ai/service-config", deps.AIServiceConfigHandler.SaveConfig)
		strictAuthRouter.POST("/ai/service-config/test", deps.AIServiceConfigHandler.TestConfig)
	}
}
