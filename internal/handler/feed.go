package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"shiliu/api/v1"
	"shiliu/internal/service"
)

type FeedHandler struct {
	*Handler
	feedService service.FeedService
}

func NewFeedHandler(handler *Handler, feedService service.FeedService) *FeedHandler {
	return &FeedHandler{Handler: handler, feedService: feedService}
}

// CreateFeed godoc
// @Summary 添加订阅源
// @Schemes
// @Description 抓取并解析成功后创建订阅源，按规范化 feed URL 去重
// @Tags 订阅源模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body v1.CreateFeedRequest true "params"
// @Success 200 {object} v1.CreateFeedResponse
// @Router /feeds [post]
func (h *FeedHandler) CreateFeed(ctx *gin.Context) {
	req := new(v1.CreateFeedRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}

	feed, err := h.feedService.CreateFeed(ctx.Request.Context(), req)
	if err != nil {
		h.handleFeedError(ctx, "feedService.CreateFeed", err)
		return
	}

	v1.HandleSuccess(ctx, feed)
}

// ListFeeds godoc
// @Summary 查询订阅源列表
// @Schemes
// @Description 返回所有订阅源及其当前 / 最后抓取状态、类型和所属文件夹
// @Tags 订阅源模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.ListFeedsResponse
// @Router /feeds [get]
func (h *FeedHandler) ListFeeds(ctx *gin.Context) {
	result, err := h.feedService.ListFeeds(ctx.Request.Context())
	if err != nil {
		h.handleFeedError(ctx, "feedService.ListFeeds", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// RefreshFeeds godoc
// @Summary 手动刷新全部订阅源
// @Schemes
// @Description 逐个复用抓取 service 刷新全部订阅源；同源抓取进行中时返回 skipped 语义
// @Tags 订阅源模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.RefreshFeedsResponse
// @Router /feeds/refresh [post]
func (h *FeedHandler) RefreshFeeds(ctx *gin.Context) {
	result, err := h.feedService.RefreshFeeds(ctx.Request.Context())
	if err != nil {
		h.handleFeedError(ctx, "feedService.RefreshFeeds", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// RefreshFeed godoc
// @Summary 手动刷新单个订阅源
// @Schemes
// @Description 复用抓取 service 刷新单个订阅源；同源抓取进行中时返回 skipped 语义
// @Tags 订阅源模块
// @Produce json
// @Security Bearer
// @Param id path int true "feed id"
// @Success 200 {object} v1.RefreshFeedResponse
// @Router /feeds/{id}/refresh [post]
func (h *FeedHandler) RefreshFeed(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 63)
	if err != nil || id == 0 {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.feedService.RefreshFeed(ctx.Request.Context(), uint(id))
	if err != nil {
		h.handleFeedError(ctx, "feedService.RefreshFeed", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// DeleteFeed godoc
// @Summary 删除订阅源
// @Schemes
// @Description 删除订阅源并级联删除其内容条目及派生数据
// @Tags 订阅源模块
// @Produce json
// @Security Bearer
// @Param id path int true "feed id"
// @Success 200 {object} v1.Response
// @Router /feeds/{id} [delete]
func (h *FeedHandler) DeleteFeed(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 63)
	if err != nil || id == 0 {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.feedService.DeleteFeed(ctx.Request.Context(), uint(id)); err != nil {
		h.handleFeedError(ctx, "feedService.DeleteFeed", err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func (h *FeedHandler) handleFeedError(ctx *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, v1.ErrFeedInvalidURL):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrFeedInvalidURL, nil)
	case errors.Is(err, v1.ErrFeedAlreadyExists):
		v1.HandleError(ctx, http.StatusConflict, v1.ErrFeedAlreadyExists, nil)
	case errors.Is(err, v1.ErrFeedFetchFailed):
		v1.HandleError(ctx, http.StatusBadGateway, v1.ErrFeedFetchFailed, nil)
	case errors.Is(err, v1.ErrFeedParseFailed):
		v1.HandleError(ctx, http.StatusUnprocessableEntity, v1.ErrFeedParseFailed, nil)
	case errors.Is(err, v1.ErrNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrNotFound, nil)
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
