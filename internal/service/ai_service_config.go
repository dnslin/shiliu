package service

import (
	"context"
	"net/url"
	"strings"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
)

type AIServiceConfigService interface {
	SaveConfig(ctx context.Context, req *v1.SaveAIServiceConfigRequest) (*v1.AIServiceConfigResponseData, error)
	GetConfig(ctx context.Context) (*v1.AIServiceConfigResponseData, error)
	TestConfig(ctx context.Context) (*v1.TestAIServiceConfigResponseData, error)
}

type AIServiceConfigTester interface {
	TestAIServiceConfig(ctx context.Context, config model.AIServiceConfig) error
}

func NewAIServiceConfigService(service *Service, configRepo repository.AIServiceConfigRepository, tester AIServiceConfigTester) AIServiceConfigService {
	return &aiServiceConfigService{Service: service, configRepo: configRepo, tester: tester}
}

type aiServiceConfigService struct {
	configRepo repository.AIServiceConfigRepository
	tester     AIServiceConfigTester
	*Service
}

func (s *aiServiceConfigService) SaveConfig(ctx context.Context, req *v1.SaveAIServiceConfigRequest) (*v1.AIServiceConfigResponseData, error) {
	config, err := aiConfigFromRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.configRepo.Save(ctx, config); err != nil {
		return nil, err
	}
	response := aiConfigResponseFromModel(config)
	return &response, nil
}

func (s *aiServiceConfigService) GetConfig(ctx context.Context) (*v1.AIServiceConfigResponseData, error) {
	config, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	response := aiConfigResponseFromModel(config)
	return &response, nil
}

func (s *aiServiceConfigService) TestConfig(ctx context.Context) (*v1.TestAIServiceConfigResponseData, error) {
	config, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, v1.ErrAIConfigMissing
	}
	if s.tester != nil {
		if err := s.tester.TestAIServiceConfig(ctx, *config); err != nil {
			return nil, err
		}
	}
	return &v1.TestAIServiceConfigResponseData{OK: true}, nil
}

func aiConfigFromRequest(req *v1.SaveAIServiceConfigRequest) (*model.AIServiceConfig, error) {
	if req == nil {
		return nil, v1.ErrBadRequest
	}
	baseURL, err := normalizeAIBaseURL(req.APIBaseURL)
	if err != nil {
		return nil, err
	}
	modelName := strings.TrimSpace(req.Model)
	apiKey := strings.TrimSpace(req.APIKey)
	if modelName == "" || apiKey == "" {
		return nil, v1.ErrBadRequest
	}
	return &model.AIServiceConfig{APIBaseURL: baseURL, Model: modelName, APIKey: apiKey}, nil
}

func normalizeAIBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", v1.ErrBadRequest
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || validateAIServiceBaseURL(parsed) != nil {
		return "", v1.ErrBadRequest
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func validateAIServiceBaseURL(parsed *url.URL) error {
	if parsed == nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" {
		return v1.ErrBadRequest
	}
	if err := validateHTTPFeedURL(parsed); err != nil {
		return v1.ErrBadRequest
	}
	return nil
}

func aiConfigResponseFromModel(config *model.AIServiceConfig) v1.AIServiceConfigResponseData {
	if config == nil {
		return v1.AIServiceConfigResponseData{}
	}
	return v1.AIServiceConfigResponseData{
		APIBaseURL:       config.APIBaseURL,
		Model:            config.Model,
		Configured:       true,
		APIKeyConfigured: config.APIKey != "",
	}
}
