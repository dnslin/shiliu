package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
)

type AIServiceConfigHandler struct {
	*Handler
	aiServiceConfigService service.AIServiceConfigService
}

func NewAIServiceConfigHandler(handler *Handler, aiServiceConfigService service.AIServiceConfigService) *AIServiceConfigHandler {
	return &AIServiceConfigHandler{Handler: handler, aiServiceConfigService: aiServiceConfigService}
}

// SaveConfig godoc
// @Summary 保存 AI 服务配置
// @Schemes
// @Description 保存 OpenAI-compatible AI 服务配置；只做格式校验，不强制连通性测试。
// @Tags AI 服务配置模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body v1.SaveAIServiceConfigRequest true "ai service config"
// @Success 200 {object} v1.AIServiceConfigResponse
// @Router /ai/service-config [put]
func (h *AIServiceConfigHandler) SaveConfig(ctx *gin.Context) {
	var req v1.SaveAIServiceConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.aiServiceConfigService.SaveConfig(ctx.Request.Context(), &req)
	if err != nil {
		h.handleAIServiceConfigError(ctx, "aiServiceConfigService.SaveConfig", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// GetConfig godoc
// @Summary 读取 AI 服务配置状态
// @Schemes
// @Description 返回 AI 服务配置状态、base URL 和模型名；不回显完整 API Key。
// @Tags AI 服务配置模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.AIServiceConfigResponse
// @Router /ai/service-config [get]
func (h *AIServiceConfigHandler) GetConfig(ctx *gin.Context) {
	result, err := h.aiServiceConfigService.GetConfig(ctx.Request.Context())
	if err != nil {
		h.handleAIServiceConfigError(ctx, "aiServiceConfigService.GetConfig", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

// TestConfig godoc
// @Summary 测试 AI 服务配置
// @Schemes
// @Description 主动对已保存 AI 服务配置发起一次 OpenAI-compatible Chat Completions 连通性测试。
// @Tags AI 服务配置模块
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.TestAIServiceConfigResponse
// @Router /ai/service-config/test [post]
func (h *AIServiceConfigHandler) TestConfig(ctx *gin.Context) {
	result, err := h.aiServiceConfigService.TestConfig(ctx.Request.Context())
	if err != nil {
		h.handleAIServiceConfigError(ctx, "aiServiceConfigService.TestConfig", err)
		return
	}
	v1.HandleSuccess(ctx, result)
}

func (h *AIServiceConfigHandler) handleAIServiceConfigError(ctx *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
	case errors.Is(err, v1.ErrAIConfigMissing):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrAIConfigMissing, nil)
	case errors.Is(err, v1.ErrAIConfigTestFailed):
		v1.HandleError(ctx, http.StatusBadGateway, v1.ErrAIConfigTestFailed, nil)
	default:
		h.logger.WithContext(ctx).Error(operation+" error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}
