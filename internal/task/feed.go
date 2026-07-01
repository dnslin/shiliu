package task

import (
	"context"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
)

type FeedTask interface {
	RefreshFeeds(ctx context.Context) (*v1.RefreshFeedsResponseData, error)
}

func NewFeedTask(
	task *Task,
	feedService service.FeedService,
) FeedTask {
	return &feedTask{
		feedService: feedService,
		Task:        task,
	}
}

type feedTask struct {
	feedService service.FeedService
	*Task
}

func (t *feedTask) RefreshFeeds(ctx context.Context) (*v1.RefreshFeedsResponseData, error) {
	if t.feedService == nil {
		return nil, v1.ErrInternalServerError
	}
	return t.feedService.RefreshFeeds(ctx)
}
