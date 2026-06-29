package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
)

type TagHandler struct {
	*Handler
	tagService service.TagService
}

func NewTagHandler(handler *Handler, tagService service.TagService) *TagHandler {
	return &TagHandler{Handler: handler, tagService: tagService}
}

// CreateTag godoc
// @Summary 创建标签
// @Schemes
// @Description 创建用于组织内容条目的标签，名称唯一。
// @Tags 标签模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body v1.CreateTagRequest true "tag"
// @Success 200 {object} v1.TagResponse
// @Router /tags [post]
func (h *TagHandler) CreateTag(ctx *gin.Context) {
	var req v1.CreateTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.tagService.CreateTag(ctx.Request.Context(), &req)
	if err != nil {
		h.handleTagError(ctx, "tagService.CreateTag", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// ListTags godoc
// @Summary 查询标签列表
// @Schemes
// @Description 返回全部标签。
// @Tags 标签模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.ListTagsResponse
// @Router /tags [get]
func (h *TagHandler) ListTags(ctx *gin.Context) {
	result, err := h.tagService.ListTags(ctx.Request.Context())
	if err != nil {
		h.handleTagError(ctx, "tagService.ListTags", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// RenameTag godoc
// @Summary 重命名标签
// @Schemes
// @Description 修改已有标签名称。
// @Tags 标签模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "tag id"
// @Param request body v1.RenameTagRequest true "tag"
// @Success 200 {object} v1.TagResponse
// @Router /tags/{id} [put]
func (h *TagHandler) RenameTag(ctx *gin.Context) {
	tagID, err := parseTagID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	var req v1.RenameTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.tagService.RenameTag(ctx.Request.Context(), tagID, &req)
	if err != nil {
		h.handleTagError(ctx, "tagService.RenameTag", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// DeleteTag godoc
// @Summary 删除标签
// @Schemes
// @Description 删除标签本身及其与内容条目的关联，不删除内容条目。
// @Tags 标签模块
// @Produce json
// @Security Bearer
// @Param id path int true "tag id"
// @Success 200 {object} v1.Response
// @Router /tags/{id} [delete]
func (h *TagHandler) DeleteTag(ctx *gin.Context) {
	tagID, err := parseTagID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.tagService.DeleteTag(ctx.Request.Context(), tagID); err != nil {
		h.handleTagError(ctx, "tagService.DeleteTag", err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// AssignContentItemTags godoc
// @Summary 给内容条目添加标签
// @Schemes
// @Description 给单个内容条目添加多个已有标签，不即时新建标签。
// @Tags 标签模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "content item id"
// @Param request body v1.AssignContentItemTagsRequest true "tag ids"
// @Success 200 {object} v1.Response
// @Router /content-items/{id}/tags [put]
func (h *TagHandler) AssignContentItemTags(ctx *gin.Context) {
	h.changeContentItemTags(ctx, "tagService.AssignContentItemTags", h.tagService.AssignContentItemTags)
}

// RemoveContentItemTags godoc
// @Summary 移除内容条目标签
// @Schemes
// @Description 从单个内容条目移除多个标签。
// @Tags 标签模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "content item id"
// @Param request body v1.AssignContentItemTagsRequest true "tag ids"
// @Success 200 {object} v1.Response
// @Router /content-items/{id}/tags [delete]
func (h *TagHandler) RemoveContentItemTags(ctx *gin.Context) {
	h.changeContentItemTags(ctx, "tagService.RemoveContentItemTags", h.tagService.RemoveContentItemTags)
}

func (h *TagHandler) changeContentItemTags(ctx *gin.Context, operation string, change func(context.Context, uint, *v1.AssignContentItemTagsRequest) error) {
	itemID, err := parseContentItemID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	var req v1.AssignContentItemTagsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := change(ctx.Request.Context(), itemID, &req); err != nil {
		h.handleTagError(ctx, operation, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func parseTagID(raw string) (uint, error) {
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

func (h *TagHandler) handleTagError(ctx *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
	case errors.Is(err, v1.ErrTagAlreadyExists):
		v1.HandleError(ctx, http.StatusConflict, v1.ErrTagAlreadyExists, nil)
	case errors.Is(err, v1.ErrTagNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrTagNotFound, nil)
	case errors.Is(err, v1.ErrContentItemNotFound), errors.Is(err, v1.ErrNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrContentItemNotFound, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
