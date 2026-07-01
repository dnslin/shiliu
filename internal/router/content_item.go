package router

import (
	"github.com/gin-gonic/gin"

	"shiliu/internal/middleware"
)

func InitContentItemRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.GET("/content-items", deps.ContentItemHandler.ListContentItems)
		strictAuthRouter.GET("/content-views/inbox", deps.ContentItemHandler.ListInboxContentItems)
		strictAuthRouter.GET("/content-views/later", deps.ContentItemHandler.ListLaterContentItems)
		strictAuthRouter.GET("/content-views/favorite", deps.ContentItemHandler.ListFavoriteContentItems)
		strictAuthRouter.GET("/content-views/completed", deps.ContentItemHandler.ListCompletedContentItems)
		strictAuthRouter.GET("/feeds/:id/content-items", deps.ContentItemHandler.ListFeedContentItems)
		strictAuthRouter.GET("/content-items/:id", deps.ContentItemHandler.GetContentItem)
		strictAuthRouter.PUT("/content-items/:id/processing-status", deps.ContentItemHandler.UpdateContentItemProcessingStatus)
		strictAuthRouter.PUT("/content-items/:id/marks/:mark", deps.ContentItemHandler.UpdateContentItemMark)
		strictAuthRouter.PUT("/content-items/:id/audio-progress", deps.ContentItemHandler.UpdateContentItemAudioProgress)
		strictAuthRouter.POST("/content-items/:id/ai-summary", deps.ContentItemHandler.GenerateAISummary)
	}
}
