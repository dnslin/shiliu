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
		strictAuthRouter.GET("/content-items/:id", deps.ContentItemHandler.GetContentItem)
	}
}
