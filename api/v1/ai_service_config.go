package v1

type SaveAIServiceConfigRequest struct {
	APIBaseURL string `json:"apiBaseUrl" binding:"required" example:"https://api.example.com/v1"`
	Model      string `json:"model" binding:"required" example:"gpt-4.1-mini"`
	APIKey     string `json:"apiKey" binding:"required"`
}

type AIServiceConfigResponseData struct {
	APIBaseURL       string `json:"apiBaseUrl"`
	Model            string `json:"model"`
	Configured       bool   `json:"configured"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
}

type AIServiceConfigResponse struct {
	Response
	Data AIServiceConfigResponseData `json:"data"`
}

type TestAIServiceConfigResponseData struct {
	OK bool `json:"ok"`
}

type TestAIServiceConfigResponse struct {
	Response
	Data TestAIServiceConfigResponseData `json:"data"`
}
