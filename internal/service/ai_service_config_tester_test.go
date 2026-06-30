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

func TestOpenAICompatibleAIServiceTesterPostsChatCompletionProbe(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	tester := NewOpenAICompatibleAIServiceTester(&http.Client{Transport: aiConfigTestRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"ok"}}]}`)),
		}, nil
	})})

	err := tester.TestAIServiceConfig(context.Background(), model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	})

	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", gotPath)
	assert.Equal(t, "Bearer sk-secret-value", gotAuth)
	assert.Equal(t, "gpt-4.1-mini", gotBody["model"])
	assert.Equal(t, false, gotBody["stream"])
	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)
	assert.Equal(t, "user", messages[len(messages)-1].(map[string]any)["role"])
}

func TestOpenAICompatibleAIServiceTesterReturnsSanitizedError(t *testing.T) {
	tester := NewOpenAICompatibleAIServiceTester(&http.Client{Transport: aiConfigTestRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"error":"bad key sk-secret-value"}`)),
		}, nil
	})})

	err := tester.TestAIServiceConfig(context.Background(), model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	})

	require.ErrorIs(t, err, v1.ErrAIConfigTestFailed)
	assert.NotContains(t, err.Error(), "sk-secret-value")
	assert.NotContains(t, strings.ToLower(err.Error()), "bad key")
}

func TestOpenAICompatibleAIServiceTesterRejectsUnsafeBaseURLBeforeTransport(t *testing.T) {
	called := false
	tester := NewOpenAICompatibleAIServiceTester(&http.Client{Transport: aiConfigTestRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"ok"}}]}`)),
		}, nil
	})})

	err := tester.TestAIServiceConfig(context.Background(), model.AIServiceConfig{
		APIBaseURL: "http://127.0.0.1:8080/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	})

	require.ErrorIs(t, err, v1.ErrAIConfigTestFailed)
	assert.False(t, called)
}

func TestOpenAICompatibleAIServiceTesterRejectsMalformedSuccessResponse(t *testing.T) {
	tester := NewOpenAICompatibleAIServiceTester(&http.Client{Transport: aiConfigTestRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"object":"not-a-chat-completion"}`)),
		}, nil
	})})

	err := tester.TestAIServiceConfig(context.Background(), model.AIServiceConfig{
		APIBaseURL: "https://api.example.com/v1",
		Model:      "gpt-4.1-mini",
		APIKey:     "sk-secret-value",
	})

	require.ErrorIs(t, err, v1.ErrAIConfigTestFailed)
	assert.NotContains(t, err.Error(), "sk-secret-value")
}

type aiConfigTestRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn aiConfigTestRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
