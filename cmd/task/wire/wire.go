//go:build wireinject
// +build wireinject

package wire

import (
	"shiliu/internal/repository"
	"shiliu/internal/server"
	"shiliu/internal/service"
	"shiliu/internal/task"
	"shiliu/pkg/app"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
	"shiliu/pkg/sid"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	repository.NewRepository,
	repository.NewTransaction,
	repository.NewFeedRepository,
	repository.NewContentItemRepository,
	repository.NewAIServiceConfigRepository,
	repository.NewAutoSummaryConfigRepository,
)

var serviceSet = wire.NewSet(
	service.NewService,
	service.NewDefaultFetcher,
	service.NewFeedService,
	service.NewDefaultChatCompletion,
	service.NewContentItemService,
	service.NewAutoSummaryService,
)

var taskSet = wire.NewSet(
	task.NewTask,
	task.NewFeedTask,
	task.NewAutoSummaryTask,
)
var serverSet = wire.NewSet(
	server.NewTaskServer,
)

// build App
func newApp(
	task *server.TaskServer,
) *app.App {
	return app.NewApp(
		app.WithServer(task),
		app.WithName("shiliu-task"),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		repositorySet,
		serviceSet,
		taskSet,
		serverSet,
		newApp,
		sid.NewSid,
		jwt.NewJwt,
	))
}
