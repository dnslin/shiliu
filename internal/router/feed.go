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
		strictAuthRouter.POST("/feeds/refresh", deps.FeedHandler.RefreshFeeds)
		strictAuthRouter.POST("/feeds/:id/refresh", deps.FeedHandler.RefreshFeed)
	}
}
