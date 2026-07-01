package service

import (
	"context"
	"errors"
	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const aiSummaryMinimumAvailableTextRunes = 80

type ChatCompletionMessage struct {
	Role    string
	Content string
}

type ChatCompletion interface {
	ChatCompletion(ctx context.Context, config model.AIServiceConfig, messages []ChatCompletionMessage) (string, error)
}

type ContentItemService interface {
	ListContentItems(ctx context.Context, req *v1.ListContentItemsRequest) (*v1.ListContentItemsResponseData, error)
	UpdateProcessingStatus(ctx context.Context, id uint, req *v1.UpdateContentItemProcessingStatusRequest) (*v1.ContentItemDetailResponseData, error)
	UpdateMark(ctx context.Context, id uint, mark model.ContentItemMark, req *v1.UpdateContentItemMarkRequest) (*v1.ContentItemDetailResponseData, error)
	UpdateAudioProgress(ctx context.Context, id uint, req *v1.UpdateContentItemAudioProgressRequest) (*v1.ContentItemDetailResponseData, error)
	GetContentItem(ctx context.Context, id uint) (*v1.ContentItemDetailResponseData, error)
	GenerateAISummary(ctx context.Context, id uint) (*v1.AISummaryResponseData, error)
}

func NewContentItemService(service *Service, contentRepo repository.ContentItemRepository, configRepo repository.AIServiceConfigRepository, chatCompletion ChatCompletion) ContentItemService {
	return &contentItemService{Service: service, contentRepo: contentRepo, configRepo: configRepo, chatCompletion: chatCompletion}
}

type contentItemService struct {
	contentRepo    repository.ContentItemRepository
	configRepo     repository.AIServiceConfigRepository
	chatCompletion ChatCompletion
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

func (s *contentItemService) UpdateProcessingStatus(ctx context.Context, id uint, req *v1.UpdateContentItemProcessingStatusRequest) (*v1.ContentItemDetailResponseData, error) {
	if id == 0 || req == nil {
		return nil, v1.ErrBadRequest
	}
	status := model.ContentItemProcessingStatus(req.ProcessingStatus)
	if !validContentItemProcessingStatus(status) {
		return nil, v1.ErrBadRequest
	}
	if err := s.contentRepo.UpdateProcessingStatus(ctx, id, status); err != nil {
		return nil, err
	}
	item, err := s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := contentItemDetailFromModel(item)
	return &result, nil
}

func (s *contentItemService) UpdateMark(ctx context.Context, id uint, mark model.ContentItemMark, req *v1.UpdateContentItemMarkRequest) (*v1.ContentItemDetailResponseData, error) {
	if id == 0 || req == nil || req.Marked == nil {
		return nil, v1.ErrBadRequest
	}
	if !validContentItemMark(mark) {
		return nil, v1.ErrBadRequest
	}
	if err := s.contentRepo.UpdateMark(ctx, id, mark, *req.Marked); err != nil {
		return nil, err
	}
	item, err := s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := contentItemDetailFromModel(item)
	return &result, nil
}

func (s *contentItemService) UpdateAudioProgress(ctx context.Context, id uint, req *v1.UpdateContentItemAudioProgressRequest) (*v1.ContentItemDetailResponseData, error) {
	if id == 0 || req == nil || req.AudioProgressSeconds == nil || *req.AudioProgressSeconds < 0 {
		return nil, v1.ErrBadRequest
	}
	item, err := s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Type != model.ContentItemTypeAudio {
		return nil, v1.ErrBadRequest
	}
	if err := s.contentRepo.UpdateAudioProgress(ctx, id, *req.AudioProgressSeconds); err != nil {
		return nil, err
	}
	item, err = s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := contentItemDetailFromModel(item)
	return &result, nil
}

func (s *contentItemService) GenerateAISummary(ctx context.Context, id uint) (*v1.AISummaryResponseData, error) {
	if id == 0 {
		return nil, v1.ErrBadRequest
	}
	item, err := s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	switch item.AISummaryStatus {
	case model.AISummaryStatusPending:
		result := aiSummaryResponseFromItem(item)
		result.Message = "正在生成"
		return &result, nil
	case model.AISummaryStatusInsufficientText:
		result := aiSummaryResponseFromItem(item)
		if result.Message == "" {
			result.Message = "可用文本不足，不可重试"
		}
		return &result, nil
	}
	availableText := strings.TrimSpace(item.AvailableText)
	if utf8.RuneCountInString(availableText) < aiSummaryMinimumAvailableTextRunes {
		summaryError := "可用文本不足，无法生成可靠摘要"
		if err := s.contentRepo.UpdateAISummary(ctx, id, model.AISummaryStatusInsufficientText, "", nil, summaryError); err != nil {
			return nil, err
		}
		result := v1.AISummaryResponseData{ContentItemID: id, State: string(model.AISummaryStatusInsufficientText), Error: summaryError, Message: summaryError}
		return &result, nil
	}
	if s.configRepo == nil || s.chatCompletion == nil {
		return nil, v1.ErrAIConfigMissing
	}
	config, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, v1.ErrAIConfigMissing
	}
	if err := s.contentRepo.UpdateAISummary(ctx, id, model.AISummaryStatusPending, "", nil, ""); err != nil {
		if errors.Is(err, v1.ErrAISummaryInProgress) || errors.Is(err, v1.ErrAIInsufficientText) {
			return s.currentAISummaryResponse(ctx, id)
		}
		return nil, err
	}
	markdown, err := s.chatCompletion.ChatCompletion(ctx, *config, buildAISummaryMessages(item, availableText))
	markdown = strings.TrimSpace(markdown)
	if err != nil || markdown == "" {
		summaryError := aiSummaryFailureReason(err)
		if markdown == "" && err == nil {
			summaryError = "AI 摘要响应为空"
		}
		updateCtx, cancel := aiSummaryStateWriteContext(ctx)
		defer cancel()
		if updateErr := s.contentRepo.UpdateAISummary(updateCtx, id, model.AISummaryStatusFailed, "", nil, summaryError); updateErr != nil {
			return nil, updateErr
		}
		result := v1.AISummaryResponseData{ContentItemID: id, State: string(model.AISummaryStatusFailed), Error: summaryError, Message: summaryError}
		return &result, nil
	}
	generatedAt := time.Now().UTC()
	updateCtx, cancel := aiSummaryStateWriteContext(ctx)
	defer cancel()
	if err := s.contentRepo.UpdateAISummary(updateCtx, id, model.AISummaryStatusSuccess, markdown, &generatedAt, ""); err != nil {
		return nil, err
	}
	result := v1.AISummaryResponseData{ContentItemID: id, State: string(model.AISummaryStatusSuccess), Markdown: markdown, GeneratedAt: &generatedAt}
	return &result, nil
}

func buildAISummaryMessages(item *model.ContentItem, availableText string) []ChatCompletionMessage {
	return []ChatCompletionMessage{
		{Role: "system", Content: "你是拾流的 AI 摘要器。只能基于可用文本生成固定结构化 Markdown，正文使用简体中文，必要英文术语、代码名、产品名和链接保留原文。结构必须包含：TL;DR、要点、对开发者 / 信息重度用户的价值、原文信息。"},
		{Role: "user", Content: "内容条目标题：" + item.Title + "\n内容类型：" + string(item.Type) + "\n可用文本：\n" + availableText},
	}
}

func aiSummaryFailureReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "AI 摘要生成超时"
	}
	return "AI 摘要生成失败"
}

func aiSummaryStateWriteContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (s *contentItemService) currentAISummaryResponse(ctx context.Context, id uint) (*v1.AISummaryResponseData, error) {
	item, err := s.contentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := aiSummaryResponseFromItem(item)
	switch item.AISummaryStatus {
	case model.AISummaryStatusPending:
		result.Message = "正在生成"
	case model.AISummaryStatusInsufficientText:
		if result.Message == "" {
			result.Message = "可用文本不足，不可重试"
		}
	}
	return &result, nil
}

func aiSummaryResponseFromItem(item *model.ContentItem) v1.AISummaryResponseData {
	if item == nil {
		return v1.AISummaryResponseData{}
	}
	return v1.AISummaryResponseData{
		ContentItemID: item.Id,
		State:         string(item.AISummaryStatus),
		Markdown:      item.AISummaryMarkdown,
		GeneratedAt:   item.AISummaryGeneratedAt,
		Error:         item.AISummaryError,
	}
}

func contentItemListFilterFromRequest(req *v1.ListContentItemsRequest) (repository.ContentItemListFilter, error) {
	var filter repository.ContentItemListFilter
	filter.Keyword = strings.TrimSpace(req.Keyword)
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
		if !validContentItemProcessingStatus(status) {
			return filter, v1.ErrInvalidContentFilter
		}
		filter.ProcessingStatus = &status
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
	if req.TagID != "" {
		tagID, err := parseContentItemFilterID(req.TagID)
		if err != nil {
			return filter, err
		}
		filter.TagID = &tagID
	}
	if req.FolderID != "" {
		folderID, err := parseContentItemFilterID(req.FolderID)
		if err != nil {
			return filter, err
		}
		filter.FolderID = &folderID
	}
	return filter, nil
}

func validContentItemProcessingStatus(status model.ContentItemProcessingStatus) bool {
	switch status {
	case model.ContentItemProcessingStatusUnprocessed, model.ContentItemProcessingStatusCompleted:
		return true
	default:
		return false
	}
}

func validContentItemMark(mark model.ContentItemMark) bool {
	switch mark {
	case model.ContentItemMarkLater, model.ContentItemMarkFavorite:
		return true
	default:
		return false
	}
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
