package router

import (
	"github.com/gin-gonic/gin"

	"shiliu/internal/middleware"
)

func InitFolderRouter(
	deps RouterDeps,
	r *gin.RouterGroup,
) {
	strictAuthRouter := r.Group("/").Use(middleware.StrictAuth(deps.JWT, deps.Logger))
	{
		strictAuthRouter.GET("/folders", deps.FolderHandler.ListFolders)
		strictAuthRouter.POST("/folders", deps.FolderHandler.CreateFolder)
		strictAuthRouter.PUT("/folders/:id", deps.FolderHandler.RenameFolder)
		strictAuthRouter.DELETE("/folders/:id", deps.FolderHandler.DeleteFolder)
		strictAuthRouter.PUT("/feeds/:id/folder", deps.FolderHandler.AssignFeedFolder)
	}
}
