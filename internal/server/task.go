package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	allowedFetchIntervalMinutesText   = "0, 30, 60, 360, 1440"
)

var allowedFetchIntervalMinutes = map[int]struct{}{
	disabledFetchIntervalMinutes: {},
	30:                           {},
	defaultFetchIntervalMinutes:  {},
	360:                          {},
	1440:                         {},
}

type TaskServer struct {
	log                   *log.Logger
	config                *viper.Viper
	feedTask              task.FeedTask
	mu                    sync.Mutex
	scheduler             *gocron.Scheduler
	cancelBackgroundFetch context.CancelFunc
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
	t.mu.Lock()
	t.scheduler = scheduler
	t.mu.Unlock()
	scheduler.StartBlocking()
	return nil
}

func (t *TaskServer) Stop(ctx context.Context) error {
	t.mu.Lock()
	scheduler := t.scheduler
	cancelBackgroundFetch := t.cancelBackgroundFetch
	t.scheduler = nil
	t.cancelBackgroundFetch = nil
	t.mu.Unlock()

	if cancelBackgroundFetch != nil {
		cancelBackgroundFetch()
	}
	if scheduler != nil {
		scheduler.Stop()
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

	jobCtx, cancelBackgroundFetch := context.WithCancel(ctx)
	job, err := scheduler.Every(interval).Minutes().WaitForSchedule().SingletonMode().Do(func() {
		t.runBackgroundFeedFetch(jobCtx)
	})
	if err != nil {
		cancelBackgroundFetch()
		return nil, err
	}
	job.Name(backgroundFetchJobName)
	t.mu.Lock()
	if t.cancelBackgroundFetch != nil {
		t.cancelBackgroundFetch()
	}
	t.cancelBackgroundFetch = cancelBackgroundFetch
	t.mu.Unlock()
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
		rawInterval := strings.TrimSpace(config.GetString(taskFetchIntervalMinutesConfigKey))
		parsedInterval, err := strconv.Atoi(rawInterval)
		if err != nil {
			return 0, fmt.Errorf("invalid %s %q: allowed integer values are %s", taskFetchIntervalMinutesConfigKey, rawInterval, allowedFetchIntervalMinutesText)
		}
		interval = parsedInterval
	}
	if _, ok := allowedFetchIntervalMinutes[interval]; !ok {
		return 0, fmt.Errorf("invalid %s %d: allowed values are %s", taskFetchIntervalMinutesConfigKey, interval, allowedFetchIntervalMinutesText)
	}
	return interval, nil
}
