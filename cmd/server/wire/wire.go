//go:build wireinject
// +build wireinject

package wire

import (
	"shiliu/internal/handler"
	"shiliu/internal/job"
	"shiliu/internal/repository"
	"shiliu/internal/router"
	"shiliu/internal/server"
	"shiliu/internal/service"
	"shiliu/pkg/app"
	"shiliu/pkg/jwt"
	"shiliu/pkg/log"
	"shiliu/pkg/server/http"
	"shiliu/pkg/sid"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	repository.NewRepository,
	repository.NewTransaction,
	repository.NewUserRepository,
	repository.NewFeedRepository,
	repository.NewContentItemRepository,
	repository.NewTagRepository,
)

var serviceSet = wire.NewSet(
	service.NewService,
	service.NewUserService,
	service.NewDefaultFetcher,
	service.NewFeedService,
	service.NewFeedFetchService,
	service.NewContentItemService,
	service.NewTagService,
)

var handlerSet = wire.NewSet(
	handler.NewHandler,
	handler.NewUserHandler,
	handler.NewFeedHandler,
	handler.NewContentItemHandler,
	handler.NewTagHandler,
)

var jobSet = wire.NewSet(
	job.NewJob,
	job.NewUserJob,
)
var serverSet = wire.NewSet(
	server.NewHTTPServer,
	server.NewJobServer,
)

// build App
func newApp(
	httpServer *http.Server,
	jobServer *server.JobServer,
	// task *server.Task,
) *app.App {
	return app.NewApp(
		app.WithServer(httpServer, jobServer),
		app.WithName("shiliu-server"),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		repositorySet,
		serviceSet,
		handlerSet,
		jobSet,
		serverSet,
		wire.Struct(new(router.RouterDeps), "*"),
		sid.NewSid,
		jwt.NewJwt,
		newApp,
	))
}
