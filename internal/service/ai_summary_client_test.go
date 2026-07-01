package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
)

func TestOpenAICompatibleChatCompletionPostsNonStreamingRequest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	client := NewOpenAICompatibleChatCompletion(&http.Client{Transport: aiSummaryRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"## TL;DR\n结构化摘要"}}]}`)),
			Header:     make(http.Header),
		}, nil
	})})

	content, err := client.ChatCompletion(context.Background(), model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	}, []ChatCompletionMessage{{Role: "user", Content: "请总结"}})

	require.NoError(t, err)
	assert.Equal(t, "## TL;DR\n结构化摘要", content)
	assert.Equal(t, "/v1/chat/completions", gotPath)
	assert.Equal(t, "Bearer sk-secret-value", gotAuth)
	assert.Equal(t, "gpt-4.1-mini", gotBody["model"])
	assert.Equal(t, false, gotBody["stream"])
	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].(map[string]any)["role"])
	assert.Equal(t, "请总结", messages[0].(map[string]any)["content"])
}

func TestOpenAICompatibleChatCompletionRejectsEmptyChoiceContent(t *testing.T) {
	client := NewOpenAICompatibleChatCompletion(&http.Client{Transport: aiSummaryRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"   "}}]}`)),
			Header:     make(http.Header),
		}, nil
	})})

	_, err := client.ChatCompletion(context.Background(), model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	}, []ChatCompletionMessage{{Role: "user", Content: "请总结"}})

	require.ErrorIs(t, err, v1.ErrAISummaryFailed)
	assert.NotContains(t, err.Error(), "sk-secret-value")
}

type aiSummaryRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn aiSummaryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
