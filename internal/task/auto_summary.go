package task

import (
	"context"

	v1 "shiliu/api/v1"
	"shiliu/internal/service"
)

type AutoSummaryTask interface {
	RunAutoSummary(ctx context.Context) (*service.AutoSummaryRunResult, error)
}

func NewAutoSummaryTask(
	task *Task,
	autoSummaryService service.AutoSummaryService,
) AutoSummaryTask {
	return &autoSummaryTask{
		autoSummaryService: autoSummaryService,
		Task:               task,
	}
}

type autoSummaryTask struct {
	autoSummaryService service.AutoSummaryService
	*Task
}

func (t *autoSummaryTask) RunAutoSummary(ctx context.Context) (*service.AutoSummaryRunResult, error) {
	if t.autoSummaryService == nil {
		return nil, v1.ErrInternalServerError
	}
	return t.autoSummaryService.RunAutoSummary(ctx)
}
