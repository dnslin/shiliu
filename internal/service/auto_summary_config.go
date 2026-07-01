package service

import (
	"context"
	"strings"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
)

type AutoSummaryConfigService interface {
	SaveConfig(ctx context.Context, req *v1.SaveAutoSummaryConfigRequest) (*v1.AutoSummaryConfigResponseData, error)
	GetConfig(ctx context.Context) (*v1.AutoSummaryConfigResponseData, error)
}

func NewAutoSummaryConfigService(
	service *Service,
	configRepo repository.AutoSummaryConfigRepository,
	aiConfigRepo repository.AIServiceConfigRepository,
) AutoSummaryConfigService {
	return &autoSummaryConfigService{Service: service, configRepo: configRepo, aiConfigRepo: aiConfigRepo}
}

type autoSummaryConfigService struct {
	configRepo   repository.AutoSummaryConfigRepository
	aiConfigRepo repository.AIServiceConfigRepository
	*Service
}

func (s *autoSummaryConfigService) SaveConfig(ctx context.Context, req *v1.SaveAutoSummaryConfigRequest) (*v1.AutoSummaryConfigResponseData, error) {
	scope, err := autoSummaryScopeFromRequest(req)
	if err != nil {
		return nil, err
	}
	existing, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	config := &model.AutoSummaryConfig{
		Enabled:          req.Enabled,
		ContentTypeScope: scope,
	}
	if req.Enabled {
		if err := s.ensureAIServiceConfigExists(ctx); err != nil {
			return nil, err
		}
		enabledAt := time.Now().UTC()
		if existing != nil && existing.Enabled && existing.ContentTypeScope == scope && existing.EnabledAt != nil {
			enabledAt = existing.EnabledAt.UTC()
		}
		config.EnabledAt = &enabledAt
	}
	if err := s.configRepo.Save(ctx, config); err != nil {
		return nil, err
	}
	response := autoSummaryConfigResponseFromModel(config)
	return &response, nil
}

func (s *autoSummaryConfigService) GetConfig(ctx context.Context) (*v1.AutoSummaryConfigResponseData, error) {
	config, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	response := autoSummaryConfigResponseFromModel(config)
	return &response, nil
}

func (s *autoSummaryConfigService) ensureAIServiceConfigExists(ctx context.Context) error {
	if s.aiConfigRepo == nil {
		return v1.ErrAIConfigMissing
	}
	config, err := s.aiConfigRepo.Get(ctx)
	if err != nil {
		return err
	}
	if config == nil {
		return v1.ErrAIConfigMissing
	}
	return nil
}

func autoSummaryScopeFromRequest(req *v1.SaveAutoSummaryConfigRequest) (model.AutoSummaryContentTypeScope, error) {
	if req == nil {
		return "", v1.ErrBadRequest
	}
	scope := model.AutoSummaryContentTypeScope(strings.TrimSpace(req.ContentTypeScope))
	if !validAutoSummaryContentTypeScope(scope) {
		return "", v1.ErrBadRequest
	}
	return scope, nil
}

func validAutoSummaryContentTypeScope(scope model.AutoSummaryContentTypeScope) bool {
	switch scope {
	case model.AutoSummaryContentTypeScopeText, model.AutoSummaryContentTypeScopeAudio, model.AutoSummaryContentTypeScopeAll:
		return true
	default:
		return false
	}
}

func autoSummaryConfigResponseFromModel(config *model.AutoSummaryConfig) v1.AutoSummaryConfigResponseData {
	if config == nil {
		return v1.AutoSummaryConfigResponseData{
			Enabled:          false,
			ContentTypeScope: string(model.AutoSummaryContentTypeScopeAll),
		}
	}
	return v1.AutoSummaryConfigResponseData{
		Enabled:          config.Enabled,
		ContentTypeScope: string(config.ContentTypeScope),
		EnabledAt:        config.EnabledAt,
	}
}
