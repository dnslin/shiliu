package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "shiliu/api/v1"
)

func TestFeedTaskRefreshFeedsDelegatesToFeedService(t *testing.T) {
	service := &recordingFeedService{
		refreshFeedsResult: &v1.RefreshFeedsResponseData{Total: 2, Refreshed: 1, Skipped: 1},
	}
	feedTask := NewFeedTask(&Task{}, service)

	result, err := feedTask.RefreshFeeds(context.Background())

	require.NoError(t, err)
	require.Equal(t, service.refreshFeedsResult, result)
	require.Equal(t, 1, service.refreshFeedsCalls)
}

type recordingFeedService struct {
	refreshFeedsCalls  int
	refreshFeedsResult *v1.RefreshFeedsResponseData
	refreshFeedsErr    error
}

func (s *recordingFeedService) CreateFeed(context.Context, *v1.CreateFeedRequest) (*v1.CreateFeedResponseData, error) {
	return nil, nil
}

func (s *recordingFeedService) ImportOPML(context.Context, *v1.ImportOPMLRequest) (*v1.ImportOPMLResponseData, error) {
	return nil, nil
}

func (s *recordingFeedService) ListFeeds(context.Context) (*v1.ListFeedsResponseData, error) {
	return nil, nil
}

func (s *recordingFeedService) RefreshFeeds(context.Context) (*v1.RefreshFeedsResponseData, error) {
	s.refreshFeedsCalls++
	return s.refreshFeedsResult, s.refreshFeedsErr
}

func (s *recordingFeedService) RefreshFeed(context.Context, uint) (*v1.RefreshFeedResponseData, error) {
	return nil, nil
}

func (s *recordingFeedService) DeleteFeed(context.Context, uint) error {
	return nil
}
