package server

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
	"shiliu/pkg/log"
)

func TestTaskServerSchedulesBackgroundFetchJob(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	feedTask := &recordingFeedTask{}
	server := NewTaskServer(testTaskLogger(), conf, feedTask, &recordingAutoSummaryTask{})

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	job := requireTaskJob(t, scheduler, backgroundFetchJobName)
	require.Equal(t, 30, job.ScheduledInterval())
	require.Equal(t, "minutes", job.ScheduledUnit())
}

func TestTaskServerSchedulesAutoSummaryJob(t *testing.T) {
	server := NewTaskServer(testTaskLogger(), viper.New(), &recordingFeedTask{}, &recordingAutoSummaryTask{})

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	job := requireTaskJob(t, scheduler, autoSummaryJobName)
	require.Equal(t, autoSummaryIntervalMinutes, job.ScheduledInterval())
	require.Equal(t, "minutes", job.ScheduledUnit())
}

func TestTaskServerDoesNotScheduleBackgroundFetchWhenDisabled(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 0)
	server := NewTaskServer(testTaskLogger(), conf, &recordingFeedTask{}, &recordingAutoSummaryTask{})

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)

	require.Nil(t, findTaskJob(scheduler, backgroundFetchJobName))
	require.NotNil(t, findTaskJob(scheduler, autoSummaryJobName))
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

func TestBackgroundFetchIntervalRejectsNonIntegerValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value interface{}
	}{
		{name: "duration", value: "30m"},
		{name: "word", value: "disabled"},
		{name: "empty", value: ""},
		{name: "fractional", value: 30.5},
		{name: "boolean", value: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := viper.New()
			conf.Set("task.fetch_interval_minutes", tc.value)

			interval, err := backgroundFetchIntervalMinutes(conf)

			require.Error(t, err)
			require.Zero(t, interval)
		})
	}
}

func TestTaskServerBackgroundFetchUsesFeedTask(t *testing.T) {
	feedTask := &recordingFeedTask{}
	server := NewTaskServer(testTaskLogger(), viper.New(), feedTask, &recordingAutoSummaryTask{})

	server.runBackgroundFeedFetch(context.Background())

	require.Equal(t, 1, feedTask.Calls())
}

func TestTaskServerAutoSummaryUsesTask(t *testing.T) {
	autoTask := &recordingAutoSummaryTask{}
	server := NewTaskServer(testTaskLogger(), viper.New(), &recordingFeedTask{}, autoTask)

	server.runAutoSummary(context.Background())

	require.Equal(t, 1, autoTask.Calls())
}

func TestTaskServerWaitsForFirstSchedule(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	feedTask := &recordingFeedTask{}
	autoTask := &recordingAutoSummaryTask{}
	server := NewTaskServer(testTaskLogger(), conf, feedTask, autoTask)

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	scheduler.StartAsync()
	defer scheduler.Stop()

	require.Never(t, func() bool {
		return feedTask.Calls() > 0 || autoTask.Calls() > 0
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestTaskServerDoesNotOverlapBackgroundFetchRuns(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	feedTask := newBlockingFeedTask()
	server := NewTaskServer(testTaskLogger(), conf, feedTask, &recordingAutoSummaryTask{})

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	scheduler.StartAsync()
	defer scheduler.Stop()

	scheduler.RunAll()
	require.Eventually(t, func() bool {
		return feedTask.Calls() == 1
	}, time.Second, 10*time.Millisecond)

	scheduler.RunAll()
	require.Never(t, func() bool {
		return feedTask.Calls() > 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	feedTask.Release()
}

func TestTaskServerDoesNotOverlapAutoSummaryRuns(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	autoTask := newBlockingAutoSummaryTask()
	server := NewTaskServer(testTaskLogger(), conf, &recordingFeedTask{}, autoTask)

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	scheduler.StartAsync()
	defer scheduler.Stop()

	scheduler.RunAll()
	require.Eventually(t, func() bool {
		return autoTask.Calls() == 1
	}, time.Second, 10*time.Millisecond)

	scheduler.RunAll()
	require.Never(t, func() bool {
		return autoTask.Calls() > 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	autoTask.Release()
}

func TestTaskServerStopCancelsInFlightBackgroundFetch(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	feedTask := newBlockingFeedTask()
	server := NewTaskServer(testTaskLogger(), conf, feedTask, &recordingAutoSummaryTask{})

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	server.scheduler = scheduler
	scheduler.StartAsync()

	scheduler.RunAll()
	require.Eventually(t, func() bool {
		return feedTask.Calls() == 1
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, server.Stop(context.Background()))
	require.True(t, feedTask.Canceled())
}

func TestTaskServerStopCancelsInFlightAutoSummary(t *testing.T) {
	conf := viper.New()
	conf.Set("task.fetch_interval_minutes", 30)
	autoTask := newBlockingAutoSummaryTask()
	server := NewTaskServer(testTaskLogger(), conf, &recordingFeedTask{}, autoTask)

	scheduler, err := server.newScheduler(context.Background())
	require.NoError(t, err)
	server.scheduler = scheduler
	scheduler.StartAsync()

	scheduler.RunAll()
	require.Eventually(t, func() bool {
		return autoTask.Calls() == 1
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, server.Stop(context.Background()))
	require.True(t, autoTask.Canceled())
}

func requireTaskJob(t *testing.T, scheduler *gocron.Scheduler, name string) *gocron.Job {
	t.Helper()
	job := findTaskJob(scheduler, name)
	require.NotNil(t, job)
	return job
}

func findTaskJob(scheduler *gocron.Scheduler, name string) *gocron.Job {
	for _, job := range scheduler.Jobs() {
		if job.GetName() == name {
			return job
		}
	}
	return nil
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

type recordingAutoSummaryTask struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (t *recordingAutoSummaryTask) RunAutoSummary(context.Context) (*service.AutoSummaryRunResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	return &service.AutoSummaryRunResult{Enabled: true, TotalCandidates: 1, Succeeded: 1}, t.err
}

func (t *recordingAutoSummaryTask) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

type blockingFeedTask struct {
	mu       sync.Mutex
	calls    int
	release  chan struct{}
	canceled bool
}

func newBlockingFeedTask() *blockingFeedTask {
	return &blockingFeedTask{
		release: make(chan struct{}),
	}
}

func (t *blockingFeedTask) RefreshFeeds(ctx context.Context) (*v1.RefreshFeedsResponseData, error) {
	t.mu.Lock()
	t.calls++
	t.mu.Unlock()

	select {
	case <-t.release:
		return &v1.RefreshFeedsResponseData{Total: 1, Refreshed: 1}, nil
	case <-ctx.Done():
		t.mu.Lock()
		t.canceled = true
		t.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (t *blockingFeedTask) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func (t *blockingFeedTask) Release() {
	close(t.release)
}

func (t *blockingFeedTask) Canceled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.canceled
}

type blockingAutoSummaryTask struct {
	mu       sync.Mutex
	calls    int
	release  chan struct{}
	canceled bool
}

func newBlockingAutoSummaryTask() *blockingAutoSummaryTask {
	return &blockingAutoSummaryTask{
		release: make(chan struct{}),
	}
}

func (t *blockingAutoSummaryTask) RunAutoSummary(ctx context.Context) (*service.AutoSummaryRunResult, error) {
	t.mu.Lock()
	t.calls++
	t.mu.Unlock()

	select {
	case <-t.release:
		return &service.AutoSummaryRunResult{Enabled: true, TotalCandidates: 1, Succeeded: 1}, nil
	case <-ctx.Done():
		t.mu.Lock()
		t.canceled = true
		t.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (t *blockingAutoSummaryTask) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func (t *blockingAutoSummaryTask) Release() {
	close(t.release)
}

func (t *blockingAutoSummaryTask) Canceled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.canceled
}

func testTaskLogger() *log.Logger {
	return &log.Logger{Logger: zap.NewNop()}
}
