package service

import (
	"context"
	"strconv"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
)

type ContentItemService interface {
	ListContentItems(ctx context.Context, req *v1.ListContentItemsRequest) (*v1.ListContentItemsResponseData, error)
	GetContentItem(ctx context.Context, id uint) (*v1.ContentItemDetailResponseData, error)
}

func NewContentItemService(service *Service, contentRepo repository.ContentItemRepository) ContentItemService {
	return &contentItemService{Service: service, contentRepo: contentRepo}
}

type contentItemService struct {
	contentRepo repository.ContentItemRepository
	*Service
}

func (s *contentItemService) ListContentItems(ctx context.Context, req *v1.ListContentItemsRequest) (*v1.ListContentItemsResponseData, error) {
	if req == nil {
		return nil, v1.ErrBadRequest
	}
	filter, err := contentItemListFilterFromRequest(req)
	if err != nil {
		return nil, err
	}
	page, limit, offset := req.Page.LimitOffsetPage()
	items, total, err := s.contentRepo.List(ctx, filter, limit, offset)
	if err != nil {
		return nil, err
	}
	responseItems := make([]v1.ContentItemListItemData, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, contentItemListItemFromModel(item))
	}
	return &v1.ListContentItemsResponseData{
		Items: responseItems,
		Page:  v1.NewPageData(responseItems, page, total).Page,
	}, nil
}

func (s *contentItemService) GetContentItem(ctx context.Context, id uint) (*v1.ContentItemDetailResponseData, error) {
	if id == 0 {
		return nil, v1.ErrBadRequest
	}
	item, err := s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := contentItemDetailFromModel(item)
	return &result, nil
}

func contentItemListFilterFromRequest(req *v1.ListContentItemsRequest) (repository.ContentItemListFilter, error) {
	var filter repository.ContentItemListFilter
	if req.ContentType != "" {
		contentType := model.ContentItemType(req.ContentType)
		switch contentType {
		case model.ContentItemTypeText, model.ContentItemTypeAudio:
			filter.ContentType = &contentType
		default:
			return filter, v1.ErrInvalidContentFilter
		}
	}
	if req.ProcessingStatus != "" {
		status := model.ContentItemProcessingStatus(req.ProcessingStatus)
		switch status {
		case model.ContentItemProcessingStatusUnprocessed, model.ContentItemProcessingStatusCompleted:
			filter.ProcessingStatus = &status
		default:
			return filter, v1.ErrInvalidContentFilter
		}
	}
	if req.Mark != "" {
		mark := model.ContentItemMark(req.Mark)
		switch mark {
		case model.ContentItemMarkLater, model.ContentItemMarkFavorite:
			filter.Mark = &mark
		default:
			return filter, v1.ErrInvalidContentFilter
		}
	}
	if req.FeedID != "" {
		feedID, err := parseContentItemFilterID(req.FeedID)
		if err != nil {
			return filter, err
		}
		filter.FeedID = &feedID
	}
	return filter, nil
}

func parseContentItemFilterID(raw string) (uint, error) {
	bitSize := strconv.IntSize
	if bitSize > 63 {
		bitSize = 63
	}
	id, err := strconv.ParseUint(raw, 10, bitSize)
	if err != nil || id == 0 {
		return 0, v1.ErrInvalidContentFilter
	}
	return uint(id), nil
}

func contentItemListItemFromModel(item *model.ContentItem) v1.ContentItemListItemData {
	if item == nil {
		return v1.ContentItemListItemData{}
	}
	return v1.ContentItemListItemData{
		Id:                   item.Id,
		FeedID:               item.FeedID,
		ContentType:          string(item.Type),
		Title:                item.Title,
		AvailableText:        item.AvailableText,
		PublishedAt:          item.PublishedAt,
		FetchedAt:            item.FetchedAt,
		ProcessingStatus:     string(item.ProcessingStatus),
		MarkedLater:          item.MarkedLater,
		Favorited:            item.Favorited,
		AudioProgressSeconds: item.AudioProgressSeconds,
	}
}

func contentItemDetailFromModel(item *model.ContentItem) v1.ContentItemDetailResponseData {
	if item == nil {
		return v1.ContentItemDetailResponseData{}
	}
	return v1.ContentItemDetailResponseData{
		Id:                   item.Id,
		FeedID:               item.FeedID,
		ContentType:          string(item.Type),
		Title:                item.Title,
		DescriptionSafe:      item.DescriptionSafe,
		ContentSafe:          item.ContentSafe,
		ShowNotesSafe:        item.ShowNotesSafe,
		AvailableText:        item.AvailableText,
		PublishedAt:          item.PublishedAt,
		FetchedAt:            item.FetchedAt,
		ProcessingStatus:     string(item.ProcessingStatus),
		MarkedLater:          item.MarkedLater,
		Favorited:            item.Favorited,
		AudioProgressSeconds: item.AudioProgressSeconds,
	}
}
