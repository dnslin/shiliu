package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
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
	h.listContentItems(ctx, contentItemListPreset{})
}

// ListInboxContentItems godoc
// @Summary List inbox content items
// @Schemes
// @Description Inbox view presets processing_status=unprocessed and accepts additional single-value filters.
// @Tags Content item module
// @Produce json
// @Security Bearer
// @Param content_type query string false "content type: text/audio"
// @Param mark query string false "content mark: later/favorite"
// @Param feed_id query int false "feed id"
// @Param page query int false "page"
// @Param pageSize query int false "page size"
// @Success 200 {object} v1.ListContentItemsResponse
// @Router /content-views/inbox [get]
func (h *ContentItemHandler) ListInboxContentItems(ctx *gin.Context) {
	h.listContentItems(ctx, contentItemListPreset{ProcessingStatus: string(model.ContentItemProcessingStatusUnprocessed)})
}

// ListLaterContentItems godoc
// @Summary List later content items
// @Schemes
// @Description Later view presets mark=later and accepts additional single-value filters.
// @Tags Content item module
// @Produce json
// @Security Bearer
// @Param content_type query string false "content type: text/audio"
// @Param processing_status query string false "processing status: unprocessed/completed"
// @Param feed_id query int false "feed id"
// @Param page query int false "page"
// @Param pageSize query int false "page size"
// @Success 200 {object} v1.ListContentItemsResponse
// @Router /content-views/later [get]
func (h *ContentItemHandler) ListLaterContentItems(ctx *gin.Context) {
	h.listContentItems(ctx, contentItemListPreset{Mark: string(model.ContentItemMarkLater)})
}

// ListFavoriteContentItems godoc
// @Summary List favorite content items
// @Schemes
// @Description Favorite view presets mark=favorite and accepts additional single-value filters.
// @Tags Content item module
// @Produce json
// @Security Bearer
// @Param content_type query string false "content type: text/audio"
// @Param processing_status query string false "processing status: unprocessed/completed"
// @Param feed_id query int false "feed id"
// @Param page query int false "page"
// @Param pageSize query int false "page size"
// @Success 200 {object} v1.ListContentItemsResponse
// @Router /content-views/favorite [get]
func (h *ContentItemHandler) ListFavoriteContentItems(ctx *gin.Context) {
	h.listContentItems(ctx, contentItemListPreset{Mark: string(model.ContentItemMarkFavorite)})
}

// ListCompletedContentItems godoc
// @Summary List completed content items
// @Schemes
// @Description Completed view presets processing_status=completed and accepts additional single-value filters.
// @Tags Content item module
// @Produce json
// @Security Bearer
// @Param content_type query string false "content type: text/audio"
// @Param mark query string false "content mark: later/favorite"
// @Param feed_id query int false "feed id"
// @Param page query int false "page"
// @Param pageSize query int false "page size"
// @Success 200 {object} v1.ListContentItemsResponse
// @Router /content-views/completed [get]
func (h *ContentItemHandler) ListCompletedContentItems(ctx *gin.Context) {
	h.listContentItems(ctx, contentItemListPreset{ProcessingStatus: string(model.ContentItemProcessingStatusCompleted)})
}

// ListFeedContentItems godoc
// @Summary List feed content items
// @Schemes
// @Description Feed detail view presets feed_id from the path and accepts additional single-value filters.
// @Tags Content item module
// @Produce json
// @Security Bearer
// @Param id path int true "feed id"
// @Param content_type query string false "content type: text/audio"
// @Param processing_status query string false "processing status: unprocessed/completed"
// @Param mark query string false "content mark: later/favorite"
// @Param page query int false "page"
// @Param pageSize query int false "page size"
// @Success 200 {object} v1.ListContentItemsResponse
// @Router /feeds/{id}/content-items [get]
func (h *ContentItemHandler) ListFeedContentItems(ctx *gin.Context) {
	feedID, err := parseContentItemID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	h.listContentItems(ctx, contentItemListPreset{FeedID: strconv.FormatUint(uint64(feedID), 10)})
}

type contentItemListPreset struct {
	ProcessingStatus string
	Mark             string
	FeedID           string
}

func (h *ContentItemHandler) listContentItems(ctx *gin.Context, preset contentItemListPreset) {
	req := &v1.ListContentItemsRequest{
		ContentType:      ctx.Query("content_type"),
		ProcessingStatus: ctx.Query("processing_status"),
		Mark:             ctx.Query("mark"),
		FeedID:           ctx.Query("feed_id"),
		Page:             v1.ParsePageRequest(ctx),
	}
	if preset.ProcessingStatus != "" {
		req.ProcessingStatus = preset.ProcessingStatus
	}
	if preset.Mark != "" {
		req.Mark = preset.Mark
	}
	if preset.FeedID != "" {
		req.FeedID = preset.FeedID
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
