package router

import (
	"github.com/gin-gonic/gin"

	"shiliu/internal/middleware"
)

func InitFeedRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.POST("/feeds", deps.FeedHandler.CreateFeed)
	}
}
