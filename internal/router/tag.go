package router

import (
	"github.com/gin-gonic/gin"

	"shiliu/internal/middleware"
)

func InitTagRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.GET("/tags", deps.TagHandler.ListTags)
		strictAuthRouter.POST("/tags", deps.TagHandler.CreateTag)
		strictAuthRouter.PUT("/tags/:id", deps.TagHandler.RenameTag)
		strictAuthRouter.DELETE("/tags/:id", deps.TagHandler.DeleteTag)
		strictAuthRouter.PUT("/content-items/:id/tags", deps.TagHandler.AssignContentItemTags)
		strictAuthRouter.DELETE("/content-items/:id/tags", deps.TagHandler.RemoveContentItemTags)
	}
}
