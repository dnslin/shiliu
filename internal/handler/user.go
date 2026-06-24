package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"shiliu/api/v1"
	"shiliu/internal/service"
)

type UserHandler struct {
	*Handler
	userService service.UserService
}

func NewUserHandler(handler *Handler, userService service.UserService) *UserHandler {
	return &UserHandler{
		Handler:     handler,
		userService: userService,
	}
}

// GetInitializationStatus godoc
// @Summary 查询首次初始化状态
// @Schemes
// @Description 返回拾流单实例是否已经创建唯一用户账户
// @Tags 用户模块
// @Produce json
// @Success 200 {object} v1.InitializationStatusResponse
// @Router /initialization [get]
func (h *UserHandler) GetInitializationStatus(ctx *gin.Context) {
	initialized, err := h.userService.IsInitialized(ctx)
	if err != nil {
		h.logger.WithContext(ctx).Error("userService.IsInitialized error", zap.Error(err))
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}

	v1.HandleSuccess(ctx, v1.InitializationStatusResponseData{Initialized: initialized})
}

// Initialize godoc
// @Summary 首次初始化
// @Schemes
// @Description 尚无用户账户时创建唯一用户账户，成功后入口永久关闭
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body v1.InitializeRequest true "params"
// @Success 200 {object} v1.Response
// @Router /initialization [post]
func (h *UserHandler) Initialize(ctx *gin.Context) {
	req := new(v1.InitializeRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}

	if err := h.userService.Initialize(ctx, req); err != nil {
		switch {
		case errors.Is(err, v1.ErrBadRequest):
			v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		case errors.Is(err, v1.ErrAccountAlreadyInitialized):
			v1.HandleError(ctx, http.StatusConflict, v1.ErrAccountAlreadyInitialized, nil)
		default:
			h.logger.WithContext(ctx).Error("userService.Initialize error", zap.Error(err))
			v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		}
		return
	}

	v1.HandleSuccess(ctx, nil)
}

// Login godoc
// @Summary 账号登录
// @Schemes
// @Description
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body v1.LoginRequest true "params"
// @Success 200 {object} v1.LoginResponse
// @Router /login [post]
func (h *UserHandler) Login(ctx *gin.Context) {
	var req v1.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}

	token, err := h.userService.Login(ctx, &req)
	if err != nil {
		v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
		return
	}
	v1.HandleSuccess(ctx, v1.LoginResponseData{
		AccessToken: token,
	})
}

// GetProfile godoc
// @Summary 获取用户信息
// @Schemes
// @Description
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} v1.GetProfileResponse
// @Router /user [get]
func (h *UserHandler) GetProfile(ctx *gin.Context) {
	userId := GetUserIdFromCtx(ctx)
	if userId == "" {
		v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
		return
	}

	user, err := h.userService.GetProfile(ctx, userId)
	if err != nil {
		switch {
		case errors.Is(err, v1.ErrNotFound):
			v1.HandleError(ctx, http.StatusNotFound, v1.ErrNotFound, nil)
		case errors.Is(err, v1.ErrBadRequest):
			v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		default:
			h.logger.WithContext(ctx).Error("userService.GetProfile error", zap.Error(err))
			v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		}
		return
	}

	v1.HandleSuccess(ctx, user)
}
