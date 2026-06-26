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

type ContentItemHandler struct {
	*Handler
	contentItemService service.ContentItemService
}

func NewContentItemHandler(handler *Handler, contentItemService service.ContentItemService) *ContentItemHandler {
	return &ContentItemHandler{Handler: handler, contentItemService: contentItemService}
}

// ListContentItems godoc
// @Summary 查询统一内容条目列表
// @Schemes
// @Description 统一内容列表查询入口，支持内容类型、处理状态、内容标记和订阅源单值 AND 过滤
// @Tags 内容条目模块
// @Produce json
// @Security Bearer
// @Param content_type query string false "content type: text/audio"
// @Param processing_status query string false "processing status: unprocessed/completed"
// @Param mark query string false "content mark: later/favorite"
// @Param feed_id query int false "feed id"
// @Param page query int false "page"
// @Param pageSize query int false "page size"
// @Success 200 {object} v1.ListContentItemsResponse
// @Router /content-items [get]
func (h *ContentItemHandler) ListContentItems(ctx *gin.Context) {
	req := &v1.ListContentItemsRequest{
		ContentType:      ctx.Query("content_type"),
		ProcessingStatus: ctx.Query("processing_status"),
		Mark:             ctx.Query("mark"),
		FeedID:           ctx.Query("feed_id"),
		Page:             v1.ParsePageRequest(ctx),
	}
	result, err := h.contentItemService.ListContentItems(ctx.Request.Context(), req)
	if err != nil {
		h.handleContentItemError(ctx, "contentItemService.ListContentItems", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// GetContentItem godoc
// @Summary 查询内容条目详情
// @Schemes
// @Description 返回单条内容条目的详情与安全渲染字段
// @Tags 内容条目模块
// @Produce json
// @Security Bearer
// @Param id path int true "content item id"
// @Success 200 {object} v1.GetContentItemResponse
// @Router /content-items/{id} [get]
func (h *ContentItemHandler) GetContentItem(ctx *gin.Context) {
	id, err := parseContentItemID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.contentItemService.GetContentItem(ctx.Request.Context(), id)
	if err != nil {
		h.handleContentItemError(ctx, "contentItemService.GetContentItem", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

func parseContentItemID(raw string) (uint, error) {
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

func (h *ContentItemHandler) handleContentItemError(ctx *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, v1.ErrInvalidContentFilter), errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
	case errors.Is(err, v1.ErrContentItemNotFound), errors.Is(err, v1.ErrNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrContentItemNotFound, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
