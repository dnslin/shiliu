package handler

import (
	"errors"
	"net/http"

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
		switch {
		case errors.Is(err, v1.ErrFeedInvalidURL):
			v1.HandleError(ctx, http.StatusBadRequest, v1.ErrFeedInvalidURL, nil)
		case errors.Is(err, v1.ErrFeedAlreadyExists):
			v1.HandleError(ctx, http.StatusConflict, v1.ErrFeedAlreadyExists, nil)
		case errors.Is(err, v1.ErrFeedFetchFailed):
			v1.HandleError(ctx, http.StatusBadGateway, v1.ErrFeedFetchFailed, nil)
		case errors.Is(err, v1.ErrFeedParseFailed):
			v1.HandleError(ctx, http.StatusUnprocessableEntity, v1.ErrFeedParseFailed, nil)
		case errors.Is(err, v1.ErrBadRequest):
			v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		default:
			h.logger.WithContext(ctx).Error("feedService.CreateFeed error", zap.Error(err))
			v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		}
		return
	}

	v1.HandleSuccess(ctx, feed)
}
