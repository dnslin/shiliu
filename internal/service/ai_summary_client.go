package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
)

const aiSummaryTimeout = 60 * time.Second

type openAICompatibleChatCompletion struct {
	client *http.Client
}

func NewOpenAICompatibleChatCompletion(client *http.Client) ChatCompletion {
	if client == nil {
		client = &http.Client{Timeout: aiSummaryTimeout}
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
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &openAICompatibleChatCompletion{client: &clientCopy}
}

func NewDefaultChatCompletion() ChatCompletion {
	return NewOpenAICompatibleChatCompletion(nil)
}

func (c *openAICompatibleChatCompletion) ChatCompletion(ctx context.Context, config model.AIServiceConfig, messages []ChatCompletionMessage) (string, error) {
	endpoint, err := aiChatCompletionsEndpoint(config.APIBaseURL)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{
		"model":    config.Model,
		"stream":   false,
		"messages": aiChatMessagesPayload(messages),
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", v1.ErrAISummaryFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("%w", v1.ErrAISummaryFailed)
	}
	content, err := decodeChatCompletionContent(resp.Body)
	if err != nil {
		return "", err
	}
	return content, nil
}

func aiChatMessagesPayload(messages []ChatCompletionMessage) []map[string]string {
	payload := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		payload = append(payload, map[string]string{
			"role":    message.Role,
			"content": message.Content,
		})
	}
	return payload
}

func decodeChatCompletionContent(body io.Reader) (string, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return "", fmt.Errorf("%w", v1.ErrAISummaryFailed)
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("%w", v1.ErrAISummaryFailed)
	}
	content := strings.TrimSpace(payload.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("%w", v1.ErrAISummaryFailed)
	}
	return content, nil
}
