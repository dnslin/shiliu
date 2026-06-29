package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
)

type FolderHandler struct {
	*Handler
	folderService service.FolderService
}

func NewFolderHandler(handler *Handler, folderService service.FolderService) *FolderHandler {
	return &FolderHandler{Handler: handler, folderService: folderService}
}

// CreateFolder godoc
// @Summary 创建文件夹
// @Schemes
// @Description 创建用于组织订阅源的文件夹，名称唯一。
// @Tags 文件夹模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body v1.CreateFolderRequest true "folder"
// @Success 200 {object} v1.FolderResponse
// @Router /folders [post]
func (h *FolderHandler) CreateFolder(ctx *gin.Context) {
	var req v1.CreateFolderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.folderService.CreateFolder(ctx.Request.Context(), &req)
	if err != nil {
		h.handleFolderError(ctx, "folderService.CreateFolder", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// ListFolders godoc
// @Summary 查询文件夹列表
// @Schemes
// @Description 返回全部文件夹。
// @Tags 文件夹模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.ListFoldersResponse
// @Router /folders [get]
func (h *FolderHandler) ListFolders(ctx *gin.Context) {
	result, err := h.folderService.ListFolders(ctx.Request.Context())
	if err != nil {
		h.handleFolderError(ctx, "folderService.ListFolders", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// RenameFolder godoc
// @Summary 重命名文件夹
// @Schemes
// @Description 修改已有文件夹名称。
// @Tags 文件夹模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "folder id"
// @Param request body v1.RenameFolderRequest true "folder"
// @Success 200 {object} v1.FolderResponse
// @Router /folders/{id} [put]
func (h *FolderHandler) RenameFolder(ctx *gin.Context) {
	folderID, err := parseFolderID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	var req v1.RenameFolderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.folderService.RenameFolder(ctx.Request.Context(), folderID, &req)
	if err != nil {
		h.handleFolderError(ctx, "folderService.RenameFolder", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// DeleteFolder godoc
// @Summary 删除文件夹
// @Schemes
// @Description 删除文件夹本身，并把原属于该文件夹的订阅源置为未归类；不删除订阅源或内容条目。
// @Tags 文件夹模块
// @Produce json
// @Security Bearer
// @Param id path int true "folder id"
// @Success 200 {object} v1.Response
// @Router /folders/{id} [delete]
func (h *FolderHandler) DeleteFolder(ctx *gin.Context) {
	folderID, err := parseFolderID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.folderService.DeleteFolder(ctx.Request.Context(), folderID); err != nil {
		h.handleFolderError(ctx, "folderService.DeleteFolder", err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// AssignFeedFolder godoc
// @Summary 设置订阅源文件夹
// @Schemes
// @Description 给单个订阅源分配一个已存在文件夹，或传 folderId=null 置为未归类。
// @Tags 文件夹模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "feed id"
// @Param request body v1.AssignFeedFolderRequest true "folder id or null"
// @Success 200 {object} v1.Response
// @Router /feeds/{id}/folder [put]
func (h *FolderHandler) AssignFeedFolder(ctx *gin.Context) {
	feedID, err := parseFolderID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	var req v1.AssignFeedFolderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.folderService.AssignFeedFolder(ctx.Request.Context(), feedID, &req); err != nil {
		h.handleFolderError(ctx, "folderService.AssignFeedFolder", err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func parseFolderID(raw string) (uint, error) {
	bitSize := strconv.IntSize
	if bitSize > 63 {
		bitSize = 63
	}
	id, err := strconv.ParseUint(raw, 10, bitSize)
	if err != nil || id == 0 {
		return 0, v1.ErrBadRequest
	}
	return uint(id), nil
}

func (h *FolderHandler) handleFolderError(ctx *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
	case errors.Is(err, v1.ErrFolderAlreadyExists):
		v1.HandleError(ctx, http.StatusConflict, v1.ErrFolderAlreadyExists, nil)
	case errors.Is(err, v1.ErrFolderNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrFolderNotFound, nil)
	case errors.Is(err, v1.ErrNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrNotFound, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
