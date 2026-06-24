package router

import (
	"github.com/gin-gonic/gin"
	"shiliu/internal/middleware"
)

func InitUserRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	// No route group has permission
	noAuthRouter := r.Group("/")
	{
		noAuthRouter.GET("/initialization", deps.UserHandler.GetInitializationStatus)
		noAuthRouter.POST("/initialization", deps.UserHandler.Initialize)
		noAuthRouter.POST("/login", deps.UserHandler.Login)
	}
	// Strict permission routing group
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.GET("/user", deps.UserHandler.GetProfile)
	}

}
