package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
)

const aiConfigTestTimeout = 10 * time.Second

type openAICompatibleAIServiceTester struct {
	client *http.Client
}

func NewOpenAICompatibleAIServiceTester(client *http.Client) AIServiceConfigTester {
	if client == nil {
		client = &http.Client{Timeout: aiConfigTestTimeout}
	}
	clientCopy := *client
	safeTransport, resolveBeforeFetch := safeFeedTransport(clientCopy.Transport, net.DefaultResolver)
	clientCopy.Transport = safeTransport
	previousCheckRedirect := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateAIServiceBaseURL(req.URL); err != nil {
			return err
		}
		if resolveBeforeFetch {
			if _, err := resolvePublicHostIPs(req.Context(), net.DefaultResolver, req.URL.Hostname()); err != nil {
				return err
			}
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &openAICompatibleAIServiceTester{client: &clientCopy}
}

func NewDefaultAIServiceConfigTester() AIServiceConfigTester {
	return NewOpenAICompatibleAIServiceTester(nil)
}

func (t *openAICompatibleAIServiceTester) TestAIServiceConfig(ctx context.Context, config model.AIServiceConfig) error {
	body, err := json.Marshal(map[string]any{
		"model": config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with ok to confirm connectivity."},
		},
		"stream": false,
	})
	if err != nil {
		return v1.ErrAIConfigTestFailed
	}
	endpoint, err := aiChatCompletionsEndpoint(config.APIBaseURL)
	if err != nil {
		return v1.ErrAIConfigTestFailed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return v1.ErrAIConfigTestFailed
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return v1.ErrAIConfigTestFailed
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: upstream returned status %d", v1.ErrAIConfigTestFailed, resp.StatusCode)
	}
	if err := validateChatCompletionProbeResponse(resp.Body); err != nil {
		return err
	}
	return nil
}

func aiChatCompletionsEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || validateAIServiceBaseURL(parsed) != nil {
		return "", v1.ErrAIConfigTestFailed
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	return parsed.String(), nil
}

func validateChatCompletionProbeResponse(body io.Reader) error {
	var response struct {
		Object  string            `json:"object"`
		Choices []json.RawMessage `json:"choices"`
	}
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return v1.ErrAIConfigTestFailed
	}
	if response.Object != "chat.completion" || len(response.Choices) == 0 {
		return v1.ErrAIConfigTestFailed
	}
	return nil
}
