package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"

	"gorm.io/gorm"
)

type FeedService interface {
	CreateFeed(ctx context.Context, req *v1.CreateFeedRequest) (*v1.CreateFeedResponseData, error)
	ImportOPML(ctx context.Context, req *v1.ImportOPMLRequest) (*v1.ImportOPMLResponseData, error)
	ListFeeds(ctx context.Context) (*v1.ListFeedsResponseData, error)
	RefreshFeeds(ctx context.Context) (*v1.RefreshFeedsResponseData, error)
	RefreshFeed(ctx context.Context, feedID uint) (*v1.RefreshFeedResponseData, error)
	DeleteFeed(ctx context.Context, feedID uint) error
}

func NewFeedService(
	service *Service,
	feedRepo repository.FeedRepository,
	contentRepo repository.ContentItemRepository,
	fetcher Fetcher,
) FeedService {
	return &feedService{
		Service:          service,
		feedRepo:         feedRepo,
		contentRepo:      contentRepo,
		fetcher:          fetcher,
		feedFetchService: NewFeedFetchService(service, feedRepo, contentRepo, fetcher),
	}
}

type feedService struct {
	feedRepo         repository.FeedRepository
	contentRepo      repository.ContentItemRepository
	fetcher          Fetcher
	feedFetchService FeedFetchService
	*Service
}

func (s *feedService) CreateFeed(ctx context.Context, req *v1.CreateFeedRequest) (*v1.CreateFeedResponseData, error) {
	if req == nil {
		return nil, v1.ErrBadRequest
	}
	feedURL, err := NormalizeFeedURL(req.FeedURL)
	if err != nil {
		return nil, err
	}
	existing, err := s.feedRepo.GetByURL(ctx, feedURL)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, v1.ErrFeedAlreadyExists
	}
	if s.fetcher == nil {
		return nil, v1.ErrFeedFetchFailed
	}

	body, err := s.fetcher.Fetch(ctx, feedURL)
	if err != nil {
		return nil, mapFeedFetchError(err)
	}
	parsed, err := parseRSSFeed(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", v1.ErrFeedParseFailed, err)
	}

	fetchedAt := time.Now().UTC()
	result := &v1.CreateFeedResponseData{
		FeedURL:      feedURL,
		Type:         string(parsed.Type),
		FetchedItems: len(parsed.Items),
	}

	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		existing, err := s.feedRepo.GetByURL(ctx, feedURL)
		if err != nil {
			return err
		}
		if existing != nil {
			return v1.ErrFeedAlreadyExists
		}

		feed := &model.Feed{
			FeedURL:       feedURL,
			Title:         parsed.Title,
			Type:          parsed.Type,
			FetchStatus:   model.FeedFetchStatusSuccess,
			LastFetchedAt: &fetchedAt,
		}
		if err := s.feedRepo.Create(ctx, feed); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return v1.ErrFeedAlreadyExists
			}
			return err
		}

		inserted, _, err := persistParsedContentItems(ctx, s.contentRepo, feed.Id, parsed.Items, fetchedAt)
		if err != nil {
			return err
		}
		result.Id = feed.Id
		result.InsertedItems = inserted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *feedService) ListFeeds(ctx context.Context) (*v1.ListFeedsResponseData, error) {
	feeds, err := s.feedRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.ListFeedsResponseData{
		Total: len(feeds),
		Items: make([]v1.FeedResponseData, 0, len(feeds)),
	}
	for _, feed := range feeds {
		response.Items = append(response.Items, feedResponseFromModel(feed))
	}
	return response, nil
}

func feedResponseFromModel(feed *model.Feed) v1.FeedResponseData {
	if feed == nil {
		return v1.FeedResponseData{}
	}
	return v1.FeedResponseData{
		Id:             feed.Id,
		FeedURL:        feed.FeedURL,
		Type:           string(feed.Type),
		FetchStatus:    string(feed.FetchStatus),
		LastFetchedAt:  feed.LastFetchedAt,
		LastFetchError: feed.LastFetchError,
		FolderID:       feed.FolderID,
	}
}

func (s *feedService) DeleteFeed(ctx context.Context, feedID uint) error {
	if feedID == 0 {
		return v1.ErrBadRequest
	}
	return s.tm.Transaction(ctx, func(ctx context.Context) error {
		return s.feedRepo.Delete(ctx, feedID)
	})
}

func (s *feedService) RefreshFeeds(ctx context.Context) (*v1.RefreshFeedsResponseData, error) {
	feeds, err := s.feedRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.RefreshFeedsResponseData{
		Total: len(feeds),
		Items: make([]v1.RefreshFeedResponseData, 0, len(feeds)),
	}
	for _, feed := range feeds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item, err := s.refreshFeed(ctx, feed)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			item = failedRefreshFeedResponse(feed, err)
		}
		countRefreshResult(response, item.Status)
		response.Items = append(response.Items, item)
	}
	return response, nil
}

func (s *feedService) RefreshFeed(ctx context.Context, feedID uint) (*v1.RefreshFeedResponseData, error) {
	if feedID == 0 {
		return nil, v1.ErrBadRequest
	}
	feed, err := s.feedRepo.GetByID(ctx, feedID)
	if err != nil {
		return nil, err
	}
	item, err := s.refreshFeed(ctx, feed)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *feedService) refreshFeed(ctx context.Context, feed *model.Feed) (v1.RefreshFeedResponseData, error) {
	result, err := s.feedFetchService.FetchFeed(ctx, feed)
	if err != nil {
		return v1.RefreshFeedResponseData{}, err
	}
	return refreshFeedResponseFromResult(result), nil
}

func refreshFeedResponseFromResult(result *FetchFeedResult) v1.RefreshFeedResponseData {
	if result == nil {
		return v1.RefreshFeedResponseData{}
	}
	return v1.RefreshFeedResponseData{
		FeedID:               result.FeedID,
		FeedURL:              result.FeedURL,
		Status:               string(result.Status),
		Message:              result.Message,
		FetchedItems:         result.FetchedItems,
		InsertedItems:        result.InsertedItems,
		SkippedExistingItems: result.SkippedExistingItems,
	}
}

func failedRefreshFeedResponse(feed *model.Feed, err error) v1.RefreshFeedResponseData {
	item := v1.RefreshFeedResponseData{Status: string(FetchResultStatusFailed), Message: publicRefreshFeedErrorMessage(err)}
	if feed != nil {
		item.FeedID = feed.Id
		item.FeedURL = feed.FeedURL
	}
	return item
}

func publicRefreshFeedErrorMessage(err error) string {
	switch {
	case errors.Is(err, v1.ErrFeedInvalidURL):
		return v1.ErrFeedInvalidURL.Error()
	case errors.Is(err, v1.ErrFeedFetchFailed):
		return v1.ErrFeedFetchFailed.Error()
	case errors.Is(err, v1.ErrFeedParseFailed):
		return v1.ErrFeedParseFailed.Error()
	case errors.Is(err, v1.ErrFeedFetchInProgress):
		return v1.ErrFeedFetchInProgress.Error()
	default:
		return v1.ErrInternalServerError.Error()
	}
}

func countRefreshResult(response *v1.RefreshFeedsResponseData, status string) {
	switch status {
	case string(FetchResultStatusSuccess):
		response.Refreshed++
	case string(FetchResultStatusSkipped):
		response.Skipped++
	case string(FetchResultStatusFailed):
		response.Failed++
	}
}

func persistParsedContentItems(
	ctx context.Context,
	contentRepo repository.ContentItemRepository,
	feedID uint,
	items []parsedFeedItem,
	fetchedAt time.Time,
) (inserted int, skippedExisting int, err error) {
	for _, item := range itemsToPersist(items) {
		contentItem, ok := buildContentItem(feedID, item, fetchedAt)
		if !ok {
			continue
		}
		existing, err := contentRepo.GetByFeedAndDedupeKey(ctx, feedID, contentItem.DedupeKey)
		if err != nil {
			return 0, 0, err
		}
		if existing != nil {
			skippedExisting++
			continue
		}
		if err := contentRepo.Create(ctx, contentItem); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				skippedExisting++
				continue
			}
			return 0, 0, err
		}
		inserted++
	}
	return inserted, skippedExisting, nil
}

func mapFeedFetchError(err error) error {
	if errors.Is(err, v1.ErrFeedInvalidURL) {
		return err
	}
	return fmt.Errorf("%w: %v", v1.ErrFeedFetchFailed, err)
}
