package service

import (
	"context"

	"shiliu/internal/model"
	"shiliu/internal/repository"
)

const autoSummaryBatchLimit = 20

type AutoSummaryService interface {
	RunAutoSummary(ctx context.Context) (*AutoSummaryRunResult, error)
}

type AutoSummaryRunResult struct {
	Enabled          bool
	TotalCandidates  int
	Succeeded        int
	Failed           int
	InsufficientText int
	Skipped          int
}

func NewAutoSummaryService(
	service *Service,
	configRepo repository.AutoSummaryConfigRepository,
	contentRepo repository.ContentItemRepository,
	contentService ContentItemService,
) AutoSummaryService {
	return &autoSummaryService{
		Service:        service,
		configRepo:     configRepo,
		contentRepo:    contentRepo,
		contentService: contentService,
	}
}

type autoSummaryService struct {
	configRepo     repository.AutoSummaryConfigRepository
	contentRepo    repository.ContentItemRepository
	contentService ContentItemService
	*Service
}

func (s *autoSummaryService) RunAutoSummary(ctx context.Context) (*AutoSummaryRunResult, error) {
	config, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if config == nil || !config.Enabled || config.EnabledAt == nil {
		return &AutoSummaryRunResult{}, nil
	}
	result := &AutoSummaryRunResult{Enabled: true}
	candidates, err := s.contentRepo.ListAutoSummaryCandidates(ctx, repository.AutoSummaryCandidateFilter{
		EnabledAt:        config.EnabledAt.UTC(),
		ContentTypeScope: config.ContentTypeScope,
	}, autoSummaryBatchLimit)
	if err != nil {
		return nil, err
	}
	result.TotalCandidates = len(candidates)
	for _, candidate := range candidates {
		itemResult, err := s.contentService.GenerateAutoAISummary(ctx, candidate.Id)
		if err != nil {
			return nil, err
		}
		countAutoSummaryItemResult(result, itemResult)
	}
	return result, nil
}

func countAutoSummaryItemResult(runResult *AutoSummaryRunResult, itemResult *AutoAISummaryGenerationResult) {
	if runResult == nil || itemResult == nil {
		return
	}
	if itemResult.Skipped || itemResult.Response == nil {
		runResult.Skipped++
		return
	}
	switch model.AISummaryStatus(itemResult.Response.State) {
	case model.AISummaryStatusSuccess:
		runResult.Succeeded++
	case model.AISummaryStatusFailed:
		runResult.Failed++
	case model.AISummaryStatusInsufficientText:
		runResult.InsufficientText++
	default:
		runResult.Skipped++
	}
}
