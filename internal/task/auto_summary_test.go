package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"shiliu/internal/service"
)

func TestAutoSummaryTaskRunAutoSummaryDelegatesToService(t *testing.T) {
	summaryService := &recordingAutoSummaryService{
		result: &service.AutoSummaryRunResult{Enabled: true, TotalCandidates: 2, Succeeded: 1, Skipped: 1},
	}
	autoSummaryTask := NewAutoSummaryTask(&Task{}, summaryService)

	result, err := autoSummaryTask.RunAutoSummary(context.Background())

	require.NoError(t, err)
	require.Equal(t, summaryService.result, result)
	require.Equal(t, 1, summaryService.calls)
}

type recordingAutoSummaryService struct {
	calls  int
	result *service.AutoSummaryRunResult
	err    error
}

func (s *recordingAutoSummaryService) RunAutoSummary(context.Context) (*service.AutoSummaryRunResult, error) {
	s.calls++
	return s.result, s.err
}
