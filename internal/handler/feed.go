package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"shiliu/api/v1"
	"shiliu/internal/service"
)

const (
	maxOPMLImportBytes        = int64(10 << 20)
	maxOPMLImportRequestBytes = maxOPMLImportBytes + int64(1<<20)
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

// ImportOPML godoc
// @Summary OPML 批量导入订阅源
// @Schemes
// @Description 上传或粘贴 OPML，一次性批量创建订阅源；只读取 feed URL，忽略 OPML 文件夹 / 分组层级
// @Tags 订阅源模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body v1.ImportOPMLRequest false "pasted OPML; runtime also accepts multipart file field file or text field opml"
// @Success 200 {object} v1.ImportOPMLResponse
// @Router /feeds/import-opml [post]
func (h *FeedHandler) ImportOPML(ctx *gin.Context) {
	req, err := bindImportOPMLRequest(ctx)
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrOPMLInvalid, nil)
		return
	}

	result, err := h.feedService.ImportOPML(ctx.Request.Context(), req)
	if err != nil {
		h.handleFeedError(ctx, "feedService.ImportOPML", err)
		return
	}

	v1.HandleSuccess(ctx, result)
}

func bindImportOPMLRequest(ctx *gin.Context) (*v1.ImportOPMLRequest, error) {
	limitOPMLImportRequest(ctx)
	if strings.HasPrefix(ctx.GetHeader("Content-Type"), "multipart/form-data") {
		return bindMultipartOPMLRequest(ctx)
	}
	req := new(v1.ImportOPMLRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		return nil, err
	}
	content, err := readOPMLWithLimit(strings.NewReader(req.OPML))
	if err != nil {
		return nil, err
	}
	req.OPML = content
	return req, nil
}

func limitOPMLImportRequest(ctx *gin.Context) {
	if ctx.Request != nil && ctx.Request.Body != nil {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxOPMLImportRequestBytes)
	}
}

func bindMultipartOPMLRequest(ctx *gin.Context) (*v1.ImportOPMLRequest, error) {
	fileHeader, err := ctx.FormFile("file")
	if err == nil {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()
		content, err := readOPMLWithLimit(file)
		if err != nil {
			return nil, err
		}
		return &v1.ImportOPMLRequest{OPML: content}, nil
	}
	if !errors.Is(err, http.ErrMissingFile) {
		return nil, err
	}
	opml := ctx.PostForm("opml")
	if strings.TrimSpace(opml) == "" {
		return nil, v1.ErrOPMLInvalid
	}
	content, err := readOPMLWithLimit(strings.NewReader(opml))
	if err != nil {
		return nil, err
	}
	return &v1.ImportOPMLRequest{OPML: content}, nil
}

func readOPMLWithLimit(reader io.Reader) (string, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maxOPMLImportBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(content)) > maxOPMLImportBytes {
		return "", v1.ErrOPMLInvalid
	}
	return string(content), nil
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
	id, err := parseFeedID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.feedService.RefreshFeed(ctx.Request.Context(), id)
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
	id, err := parseFeedID(ctx.Param("id"))
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.feedService.DeleteFeed(ctx.Request.Context(), id); err != nil {
		h.handleFeedError(ctx, "feedService.DeleteFeed", err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func parseFeedID(raw string) (uint, error) {
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
	case errors.Is(err, v1.ErrOPMLInvalid):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrOPMLInvalid, nil)
	case errors.Is(err, v1.ErrOPMLImportFailed):
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrOPMLImportFailed, nil)
	case errors.Is(err, v1.ErrNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrNotFound, nil)
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
