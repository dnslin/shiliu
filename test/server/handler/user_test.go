package handler

import (
	"errors"
	"net/http"
	"testing"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/handler"
	"shiliu/internal/middleware"
	mock_service "shiliu/test/mocks/service"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
)

func TestUserHandler_GetInitializationStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().IsInitialized(gomock.Any()).Return(false, nil)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.GET("/initialization", userHandler.GetInitializationStatus)

	obj := newHttpExcept(t, r).GET("/initialization").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
	obj.Value("data").Object().Value("initialized").IsEqual(false)
}
func TestUserHandler_Initialize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.InitializeRequest{
		Username: "first-account",
		Password: "123456789012",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Initialize(gomock.Any(), &params).Return(nil)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.POST("/initialization", userHandler.Initialize)

	obj := newHttpExcept(t, r).POST("/initialization").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
	obj.Value("data").Object().IsEmpty()
}

func TestUserHandler_InitializeAlreadyInitialized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.InitializeRequest{
		Username: "second-account",
		Password: "123456789012",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Initialize(gomock.Any(), &params).Return(v1.ErrAccountAlreadyInitialized)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.POST("/initialization", userHandler.Initialize)

	obj := newHttpExcept(t, r).POST("/initialization").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusConflict).
		JSON().
		Object()
	obj.Value("code").IsEqual(1003)
	obj.Value("message").IsEqual("account already initialized")
}
func TestUserHandler_InitializeShortPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.InitializeRequest{
		Username: "first-account",
		Password: "12345678901",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Initialize(gomock.Any(), &params).Return(v1.ErrBadRequest)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.POST("/initialization", userHandler.Initialize)

	obj := newHttpExcept(t, r).POST("/initialization").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusBadRequest).
		JSON().
		Object()
	obj.Value("code").IsEqual(400)
	obj.Value("message").IsEqual("Bad Request")
}

func TestUserHandler_Login(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.LoginRequest{
		Username: "testuser",
		Password: "123456",
	}

	tk := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJVc2VySWQiOiJ4eHgiLCJleHAiOjE3MzgyMjA1MTQsIm5iZiI6MTczMDQ0NDUxNCwiaWF0IjoxNzMwNDQ0NTE0fQ.3D4YupmPBCkv16ESnYyWSV5Mxcdu0twzEUqx0K-UiWo"
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Login(gomock.Any(), &params).Return(tk, nil)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	router.POST("/login", userHandler.Login)

	obj := newHttpExcept(t, router).POST("/login").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
	obj.Value("data").Object().Value("accessToken").IsEqual(tk)
}

func TestUserHandler_LoginInvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.LoginRequest{
		Username: "testuser",
		Password: "wrong-password",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Login(gomock.Any(), &params).Return("", v1.ErrInvalidCredentials)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.POST("/login", userHandler.Login)

	obj := newHttpExcept(t, r).POST("/login").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusUnauthorized).
		JSON().
		Object()
	obj.Value("code").IsEqual(1004)
	obj.Value("message").IsEqual("invalid credentials")
}

func TestUserHandler_LoginAccountLocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.LoginRequest{
		Username: "testuser",
		Password: "correct-password",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Login(gomock.Any(), &params).Return("", v1.ErrAccountLocked)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.POST("/login", userHandler.Login)

	obj := newHttpExcept(t, r).POST("/login").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusUnauthorized).
		JSON().
		Object()
	obj.Value("code").IsEqual(1005)
	obj.Value("message").IsEqual("account locked")
}

func TestUserHandler_LoginUnexpectedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.LoginRequest{
		Username: "testuser",
		Password: "correct-password",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Login(gomock.Any(), &params).Return("", errors.New("database unavailable"))

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.POST("/login", userHandler.Login)

	obj := newHttpExcept(t, r).POST("/login").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusInternalServerError).
		JSON().
		Object()
	obj.Value("code").IsEqual(500)
	obj.Value("message").IsEqual("Internal Server Error")
}

func TestUserHandler_GetProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	username := "testuser"
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().GetProfile(gomock.Any(), userId).Return(&v1.GetProfileResponseData{
		Id:       123,
		Username: username,
	}, nil)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.Use(middleware.StrictAuth(jwt, logger))
	r.GET("/user", userHandler.GetProfile)

	obj := newHttpExcept(t, r).GET("/user").
		WithHeader("Authorization", "Bearer "+genToken(t)).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
	objData := obj.Value("data").Object()
	objData.Value("id").IsEqual(123)
	objData.Value("username").IsEqual(username)
}

func TestUserHandler_ChangePassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.ChangePasswordRequest{
		OldPassword: "old-password-12",
		NewPassword: "new-password-12",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().ChangePassword(gomock.Any(), userId, &params).Return(nil)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.Use(middleware.StrictAuth(jwt, logger))
	r.PUT("/user/password", userHandler.ChangePassword)

	obj := newHttpExcept(t, r).PUT("/user/password").
		WithHeader("Authorization", "Bearer "+genToken(t)).
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
	obj.Value("data").Object().IsEmpty()
}

func TestUserHandler_ChangePasswordInvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.ChangePasswordRequest{
		OldPassword: "wrong-password-12",
		NewPassword: "new-password-12",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().ChangePassword(gomock.Any(), userId, &params).Return(v1.ErrInvalidCredentials)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.Use(middleware.StrictAuth(jwt, logger))
	r.PUT("/user/password", userHandler.ChangePassword)

	obj := newHttpExcept(t, r).PUT("/user/password").
		WithHeader("Authorization", "Bearer "+genToken(t)).
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusUnauthorized).
		JSON().
		Object()
	obj.Value("code").IsEqual(1004)
	obj.Value("message").IsEqual("invalid credentials")
}

func TestUserHandler_ChangePasswordBadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.ChangePasswordRequest{
		OldPassword: "old-password-12",
		NewPassword: "short",
	}
	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().ChangePassword(gomock.Any(), userId, &params).Return(v1.ErrBadRequest)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.Use(middleware.StrictAuth(jwt, logger))
	r.PUT("/user/password", userHandler.ChangePassword)

	obj := newHttpExcept(t, r).PUT("/user/password").
		WithHeader("Authorization", "Bearer "+genToken(t)).
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusBadRequest).
		JSON().
		Object()
	obj.Value("code").IsEqual(400)
	obj.Value("message").IsEqual("Bad Request")
}

func TestUserHandler_GetProfileRejectsMissingInvalidAndExpiredTokens(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserService := mock_service.NewMockUserService(ctrl)
	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.Use(middleware.StrictAuth(jwt, logger))
	r.GET("/user", userHandler.GetProfile)
	expiredToken, err := jwt.GenToken(userId, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	newHttpExcept(t, r).GET("/user").
		Expect().
		Status(http.StatusUnauthorized).
		JSON().
		Object().
		Value("code").IsEqual(401)
	newHttpExcept(t, r).GET("/user").
		WithHeader("Authorization", "Bearer not-a-token").
		Expect().
		Status(http.StatusUnauthorized).
		JSON().
		Object().
		Value("code").IsEqual(401)
	newHttpExcept(t, r).GET("/user").
		WithHeader("Authorization", "Bearer "+expiredToken).
		Expect().
		Status(http.StatusUnauthorized).
		JSON().
		Object().
		Value("code").IsEqual(401)
}

func TestUserHandler_GetProfile_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().GetProfile(gomock.Any(), userId).Return(nil, v1.ErrNotFound)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	r := gin.New()
	r.Use(middleware.StrictAuth(jwt, logger))
	r.GET("/user", userHandler.GetProfile)

	obj := newHttpExcept(t, r).GET("/user").
		WithHeader("Authorization", "Bearer "+genToken(t)).
		Expect().
		Status(http.StatusNotFound).
		JSON().
		Object()
	obj.Value("code").IsEqual(404)
	obj.Value("message").IsEqual("Not Found")
}
