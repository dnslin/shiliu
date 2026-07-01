package server

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"shiliu/internal/task"
	"shiliu/pkg/log"
)

const (
	backgroundFetchJobName            = "background-feed-fetch"
	taskFetchIntervalMinutesConfigKey = "task.fetch_interval_minutes"
	defaultFetchIntervalMinutes       = 60
	disabledFetchIntervalMinutes      = 0
)

var allowedFetchIntervalMinutes = map[int]struct{}{
	disabledFetchIntervalMinutes: {},
	30:                           {},
	defaultFetchIntervalMinutes:  {},
	360:                          {},
	1440:                         {},
}

type TaskServer struct {
	log       *log.Logger
	config    *viper.Viper
	scheduler *gocron.Scheduler
	feedTask  task.FeedTask
}

func NewTaskServer(
	log *log.Logger,
	config *viper.Viper,
	feedTask task.FeedTask,
) *TaskServer {
	return &TaskServer{
		log:      log,
		config:   config,
		feedTask: feedTask,
	}
}

func (t *TaskServer) Start(ctx context.Context) error {
	gocron.SetPanicHandler(func(jobName string, recoverData interface{}) {
		t.log.Error("TaskServer Panic", zap.String("job", jobName), zap.Any("recover", recoverData))
	})

	scheduler, err := t.newScheduler(ctx)
	if err != nil {
		t.log.Error("TaskServer schedule error", zap.Error(err))
		return err
	}
	t.scheduler = scheduler
	t.scheduler.StartBlocking()
	return nil
}

func (t *TaskServer) Stop(ctx context.Context) error {
	if t.scheduler != nil {
		t.scheduler.Stop()
	}
	t.log.Info("TaskServer stop...")
	return nil
}

func (t *TaskServer) newScheduler(ctx context.Context) (*gocron.Scheduler, error) {
	interval, err := backgroundFetchIntervalMinutes(t.config)
	if err != nil {
		return nil, err
	}

	scheduler := gocron.NewScheduler(time.UTC)
	if interval == disabledFetchIntervalMinutes {
		t.log.Info("background feed fetch disabled")
		return scheduler, nil
	}

	job, err := scheduler.Every(interval).Minutes().Do(func() {
		t.runBackgroundFeedFetch(ctx)
	})
	if err != nil {
		return nil, err
	}
	job.Name(backgroundFetchJobName)
	return scheduler, nil
}

func (t *TaskServer) runBackgroundFeedFetch(ctx context.Context) {
	if t.feedTask == nil {
		t.log.Error("background feed fetch task missing")
		return
	}
	result, err := t.feedTask.RefreshFeeds(ctx)
	if err != nil {
		t.log.Error("background feed fetch error", zap.Error(err))
		return
	}
	if result == nil {
		t.log.Info("background feed fetch completed")
		return
	}
	t.log.Info(
		"background feed fetch completed",
		zap.Int("total", result.Total),
		zap.Int("refreshed", result.Refreshed),
		zap.Int("skipped", result.Skipped),
		zap.Int("failed", result.Failed),
	)
}

func backgroundFetchIntervalMinutes(config *viper.Viper) (int, error) {
	interval := defaultFetchIntervalMinutes
	if config != nil && config.IsSet(taskFetchIntervalMinutesConfigKey) {
		interval = config.GetInt(taskFetchIntervalMinutesConfigKey)
	}
	if _, ok := allowedFetchIntervalMinutes[interval]; !ok {
		return 0, fmt.Errorf("invalid %s %d: allowed values are 0, 30, 60, 360, 1440", taskFetchIntervalMinutesConfigKey, interval)
	}
	return interval, nil
}
