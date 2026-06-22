package router

import (
	"shiliu/internal/handler"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
	"github.com/spf13/viper"
)

type RouterDeps struct {
	Logger      *log.Logger
	Config      *viper.Viper
	JWT         *jwt.JWT
	UserHandler *handler.UserHandler
}
