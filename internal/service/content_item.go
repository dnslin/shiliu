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
	ExportObsidianMarkdown(ctx context.Context, id uint) (*v1.ExportContentItemObsidianResponseData, error)
	GenerateAISummary(ctx context.Context, id uint) (*v1.AISummaryResponseData, error)
	GenerateAutoAISummary(ctx context.Context, id uint) (*AutoAISummaryGenerationResult, error)
}

type AutoAISummaryGenerationResult struct {
	Response *v1.AISummaryResponseData
	Skipped  bool
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

func (s *contentItemService) ExportObsidianMarkdown(ctx context.Context, id uint) (*v1.ExportContentItemObsidianResponseData, error) {
	data, err := s.contentRepo.GetExportDataByID(ctx, id)
	if err != nil {
		return nil, err
	}
	markdown, err := formatObsidianExportMarkdown(data)
	if err != nil {
		return nil, err
	}
	return &v1.ExportContentItemObsidianResponseData{
		ContentItemID: data.ContentItemID,
		Filename:      obsidianExportFilename(data),
		Markdown:      markdown,
	}, nil
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
	result, err := s.generateAISummary(ctx, id, []model.AISummaryStatus{
		model.AISummaryStatusNone,
		model.AISummaryStatusFailed,
		model.AISummaryStatusSuccess,
	}, false)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (s *contentItemService) GenerateAutoAISummary(ctx context.Context, id uint) (*AutoAISummaryGenerationResult, error) {
	return s.generateAISummary(ctx, id, []model.AISummaryStatus{model.AISummaryStatusNone}, true)
}

func (s *contentItemService) generateAISummary(ctx context.Context, id uint, claimStatuses []model.AISummaryStatus, automatic bool) (*AutoAISummaryGenerationResult, error) {
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
		return &AutoAISummaryGenerationResult{Response: &result, Skipped: automatic}, nil
	case model.AISummaryStatusInsufficientText:
		result := aiSummaryResponseFromItem(item)
		if result.Message == "" {
			result.Message = "可用文本不足，不可重试"
		}
		return &AutoAISummaryGenerationResult{Response: &result, Skipped: automatic}, nil
	}
	if automatic && item.AISummaryStatus != model.AISummaryStatusNone {
		result := aiSummaryResponseFromItem(item)
		return &AutoAISummaryGenerationResult{Response: &result, Skipped: true}, nil
	}
	availableText := strings.TrimSpace(item.AvailableText)
	if utf8.RuneCountInString(availableText) < aiSummaryMinimumAvailableTextRunes {
		summaryError := "可用文本不足，无法生成可靠摘要"
		if err := s.contentRepo.UpdateAISummary(ctx, id, model.AISummaryStatusInsufficientText, "", nil, summaryError); err != nil {
			return nil, err
		}
		result := v1.AISummaryResponseData{ContentItemID: id, State: string(model.AISummaryStatusInsufficientText), Error: summaryError, Message: summaryError}
		return &AutoAISummaryGenerationResult{Response: &result}, nil
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
	if err := s.contentRepo.ClaimAISummary(ctx, id, claimStatuses); err != nil {
		if errors.Is(err, v1.ErrAISummaryInProgress) || errors.Is(err, v1.ErrAIInsufficientText) {
			result, currentErr := s.currentAISummaryResponse(ctx, id)
			if currentErr != nil {
				return nil, currentErr
			}
			return &AutoAISummaryGenerationResult{Response: result, Skipped: automatic}, nil
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
		return &AutoAISummaryGenerationResult{Response: &result}, nil
	}
	generatedAt := time.Now().UTC()
	updateCtx, cancel := aiSummaryStateWriteContext(ctx)
	defer cancel()
	if err := s.contentRepo.UpdateAISummary(updateCtx, id, model.AISummaryStatusSuccess, markdown, &generatedAt, ""); err != nil {
		return nil, err
	}
	result := v1.AISummaryResponseData{ContentItemID: id, State: string(model.AISummaryStatusSuccess), Markdown: markdown, GeneratedAt: &generatedAt}
	return &AutoAISummaryGenerationResult{Response: &result}, nil
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

const obsidianExportExcerptMaxRunes = 2000

func formatObsidianExportMarkdown(data *repository.ContentItemExportData) (string, error) {
	if data == nil || data.ContentItemID == 0 {
		return "", v1.ErrExportFailed
	}
	summary, err := obsidianExportSummaryText(data)
	if err != nil {
		return "", err
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = "Untitled"
	}
	publishedAt := "无"
	if data.PublishedAt != nil {
		publishedAt = data.PublishedAt.UTC().Format(time.RFC3339)
	}
	tags := "无"
	if len(data.TagNames) > 0 {
		tags = strings.Join(data.TagNames, ", ")
	}
	folder := "无"
	if data.FolderName != nil && strings.TrimSpace(*data.FolderName) != "" {
		folder = strings.TrimSpace(*data.FolderName)
	}
	excerpt, truncated := obsidianExportExcerpt(data.AvailableText)

	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(title)
	builder.WriteString("\n\n## 元信息\n\n")
	builder.WriteString("- 标题：")
	builder.WriteString(title)
	builder.WriteString("\n- 链接：")
	builder.WriteString(strings.TrimSpace(data.FeedURL))
	builder.WriteString("\n- 订阅源：")
	builder.WriteString(strings.TrimSpace(data.FeedTitle))
	builder.WriteString("\n- 发布时间：")
	builder.WriteString(publishedAt)
	builder.WriteString("\n- 内容类型：")
	builder.WriteString(string(data.ContentType))
	builder.WriteString("\n- 标签：")
	builder.WriteString(tags)
	builder.WriteString("\n- 订阅源文件夹：")
	builder.WriteString(folder)
	builder.WriteString("\n\n## AI 摘要\n\n")
	builder.WriteString(summary)
	builder.WriteString("\n\n## 可用文本摘录\n\n")
	builder.WriteString(excerpt)
	if truncated {
		if excerpt != "" {
			builder.WriteString("\n\n")
		}
		builder.WriteString("已截断，请打开原文链接查看完整内容")
	}
	builder.WriteString("\n")
	return builder.String(), nil
}

func obsidianExportSummaryText(data *repository.ContentItemExportData) (string, error) {
	switch data.AISummaryStatus {
	case model.AISummaryStatusSuccess:
		markdown := strings.TrimSpace(data.AISummaryMarkdown)
		if markdown == "" {
			return "未生成", nil
		}
		return markdown, nil
	case model.AISummaryStatusNone:
		return "未生成", nil
	case model.AISummaryStatusPending:
		return "正在生成", nil
	case model.AISummaryStatusFailed:
		summaryError := strings.TrimSpace(data.AISummaryError)
		if summaryError == "" {
			return "生成失败", nil
		}
		return "生成失败：" + summaryError, nil
	case model.AISummaryStatusInsufficientText:
		return "可用文本不足", nil
	default:
		return "", v1.ErrExportFailed
	}
}

func obsidianExportExcerpt(availableText string) (string, bool) {
	text := strings.TrimSpace(availableText)
	if utf8.RuneCountInString(text) <= obsidianExportExcerptMaxRunes {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:obsidianExportExcerptMaxRunes]), true
}

func obsidianExportFilename(data *repository.ContentItemExportData) string {
	if data == nil {
		return "content-item.md"
	}
	name := strings.TrimSpace(data.Title)
	replacer := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", `"`, "-", "<", "-", ">", "-", "|", "-")
	name = strings.TrimSpace(replacer.Replace(name))
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "content-item-" + strconv.FormatUint(uint64(data.ContentItemID), 10)
	}
	return name + ".md"
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
