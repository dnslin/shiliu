package handler

import (
	"net/http"
	"testing"

	v1 "shiliu/api/v1"
	"shiliu/internal/handler"
	"shiliu/internal/middleware"
	mock_service "shiliu/test/mocks/service"

	"github.com/golang/mock/gomock"
)

func TestUserHandler_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := v1.RegisterRequest{
		Username: "testuser",
		Password: "123456",
	}

	mockUserService := mock_service.NewMockUserService(ctrl)
	mockUserService.EXPECT().Register(gomock.Any(), &params).Return(nil)

	userHandler := handler.NewUserHandler(hdl, mockUserService)
	router.POST("/register", userHandler.Register)

	e := newHttpExcept(t, router)
	obj := e.POST("/register").
		WithHeader("Content-Type", "application/json").
		WithJSON(params).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()
	obj.Value("code").IsEqual(0)
	obj.Value("message").IsEqual("ok")
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
	router.Use(middleware.NoStrictAuth(jwt, logger))
	router.GET("/user", userHandler.GetProfile)

	obj := newHttpExcept(t, router).GET("/user").
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
