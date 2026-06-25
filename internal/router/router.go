package router

import (
	"github.com/spf13/viper"
	"shiliu/internal/handler"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
)

type RouterDeps struct {
	Logger      *log.Logger
	Config      *viper.Viper
	JWT         *jwt.JWT
	UserHandler *handler.UserHandler
	FeedHandler *handler.FeedHandler
}
