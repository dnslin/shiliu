package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
)

type AutoSummaryConfigHandler struct {
	*Handler
	autoSummaryConfigService service.AutoSummaryConfigService
}

func NewAutoSummaryConfigHandler(handler *Handler, autoSummaryConfigService service.AutoSummaryConfigService) *AutoSummaryConfigHandler {
	return &AutoSummaryConfigHandler{Handler: handler, autoSummaryConfigService: autoSummaryConfigService}
}

// SaveConfig godoc
// @Summary 保存自动摘要配置
// @Schemes
// @Description 保存自动摘要全局开关和内容类型范围；开启时只对开启后新入库且摘要状态为 none 的内容生效。
// @Tags AI 服务配置模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body v1.SaveAutoSummaryConfigRequest true "auto summary config"
// @Success 200 {object} v1.AutoSummaryConfigResponse
// @Router /ai/auto-summary-config [put]
func (h *AutoSummaryConfigHandler) SaveConfig(ctx *gin.Context) {
	var req v1.SaveAutoSummaryConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.autoSummaryConfigService.SaveConfig(ctx.Request.Context(), &req)
	if err != nil {
		h.handleAutoSummaryConfigError(ctx, "autoSummaryConfigService.SaveConfig", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// GetConfig godoc
// @Summary 读取自动摘要配置
// @Schemes
// @Description 返回自动摘要全局开关、内容类型范围和当前生效时间。
// @Tags AI 服务配置模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.AutoSummaryConfigResponse
// @Router /ai/auto-summary-config [get]
func (h *AutoSummaryConfigHandler) GetConfig(ctx *gin.Context) {
	result, err := h.autoSummaryConfigService.GetConfig(ctx.Request.Context())
	if err != nil {
		h.handleAutoSummaryConfigError(ctx, "autoSummaryConfigService.GetConfig", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

func (h *AutoSummaryConfigHandler) handleAutoSummaryConfigError(ctx *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
	case errors.Is(err, v1.ErrAIConfigMissing):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrAIConfigMissing, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
