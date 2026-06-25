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
}

func NewFeedService(
	service *Service,
	feedRepo repository.FeedRepository,
	contentRepo repository.ContentItemRepository,
	fetcher Fetcher,
) FeedService {
	return &feedService{
		Service:     service,
		feedRepo:    feedRepo,
		contentRepo: contentRepo,
		fetcher:     fetcher,
	}
}

type feedService struct {
	feedRepo    repository.FeedRepository
	contentRepo repository.ContentItemRepository
	fetcher     Fetcher
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
