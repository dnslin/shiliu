package server

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/pkg/log"
)

func TestTaskServerSchedulesBackgroundFetchJob(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	feedTask := &recordingFeedTask{}
	server := NewTaskServer(testTaskLogger(), conf, feedTask)

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	jobs := scheduler.Jobs()
	require.Len(t, jobs, 1)
	require.Equal(t, 30, jobs[0].ScheduledInterval())
	require.Equal(t, "minutes", jobs[0].ScheduledUnit())
	require.Equal(t, backgroundFetchJobName, jobs[0].GetName())
}

func TestTaskServerDoesNotScheduleBackgroundFetchWhenDisabled(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 0)
	server := NewTaskServer(testTaskLogger(), conf, &recordingFeedTask{})

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)

	require.Empty(t, scheduler.Jobs())
}

func TestBackgroundFetchIntervalUsesDefaultWhenConfigIsMissing(t *testing.T) {
	interval, err := backgroundFetchIntervalMinutes(viper.New())

	require.NoError(t, err)
	require.Equal(t, 60, interval)
}

func TestBackgroundFetchIntervalAcceptsConfiguredAllowedValues(t *testing.T) {
	for _, value := range []int{0, 30, 60, 360, 1440} {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			conf := viper.New()
			conf.Set("task.fetch_interval_minutes", value)

			interval, err := backgroundFetchIntervalMinutes(conf)

			require.NoError(t, err)
			require.Equal(t, value, interval)
		})
	}
}

func TestBackgroundFetchIntervalRejectsUnexpectedValues(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 15)

	interval, err := backgroundFetchIntervalMinutes(conf)

	require.Error(t, err)
	require.Zero(t, interval)
}

func TestTaskServerBackgroundFetchUsesFeedTask(t *testing.T) {
	feedTask := &recordingFeedTask{}
	server := NewTaskServer(testTaskLogger(), viper.New(), feedTask)

	server.runBackgroundFeedFetch(context.Background())

	require.Equal(t, 1, feedTask.Calls())
}

type recordingFeedTask struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (t *recordingFeedTask) RefreshFeeds(context.Context) (*v1.RefreshFeedsResponseData, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	return &v1.RefreshFeedsResponseData{Total: 1, Refreshed: 1}, t.err
}

func (t *recordingFeedTask) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func testTaskLogger() *log.Logger {
	return &log.Logger{Logger: zap.NewNop()}
}
