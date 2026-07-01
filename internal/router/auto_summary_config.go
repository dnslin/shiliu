package router

import (
	"github.com/gin-gonic/gin"

	"shiliu/internal/middleware"
)

func InitAutoSummaryConfigRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.GET("/ai/auto-summary-config", deps.AutoSummaryConfigHandler.GetConfig)
		strictAuthRouter.PUT("/ai/auto-summary-config", deps.AutoSummaryConfigHandler.SaveConfig)
	}
}
