package content_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shiliu/pkg/content"
)

func TestSanitizeHTMLRemovesDangerousMarkup(t *testing.T) {
	raw := `<p onclick="steal()">Safe <strong>text</strong> <a href="javascript:alert(1)" onmouseover="steal()">link</a></p><img src="https://evil.example/pixel.png" alt="tracking" onerror="steal()"><script>alert("x")</script><iframe src="https://evil.example/embed">bad</iframe>`

	got := content.SanitizeHTML(raw)

	require.Contains(t, got, "Safe")
	assert.Contains(t, got, "<strong>text</strong>")
	assert.Contains(t, got, "link")

	lower := strings.ToLower(got)
	assert.NotContains(t, lower, "<script")
	assert.NotContains(t, lower, "alert")
	assert.NotContains(t, lower, "onclick")
	assert.NotContains(t, lower, "onmouseover")
	assert.NotContains(t, lower, "javascript:")
	assert.NotContains(t, lower, "<iframe")
	assert.NotContains(t, lower, "<img")
	assert.NotContains(t, lower, "pixel.png")
	assert.NotContains(t, lower, "tracking")
	assert.NotContains(t, lower, "evil.example")
}

func TestAvailableTextUsesFirstNonEmptySourceByPriority(t *testing.T) {
	tests := []struct {
		name   string
		fields content.TextFields
		want   string
	}{
		{
			name: "content wins over lower-priority fields",
			fields: content.TextFields{
				Content:     "<p>Full content</p>",
				ShowNotes:   "Show notes",
				Description: "Description",
				Summary:     "Summary",
				Title:       "Title",
			},
			want: "Full content",
		},
		{
			name: "show notes fill missing content",
			fields: content.TextFields{
				ShowNotes:   "<p>Show notes</p>",
				Description: "Description",
				Summary:     "Summary",
				Title:       "Title",
			},
			want: "Show notes",
		},
		{
			name: "description fills missing content and show notes",
			fields: content.TextFields{
				Description: "<p>Description</p>",
				Summary:     "Summary",
				Title:       "Title",
			},
			want: "Description",
		},
		{
			name: "summary fills missing description",
			fields: content.TextFields{
				Summary: "<p>Summary</p>",
				Title:   "Title",
			},
			want: "Summary",
		},
		{
			name:   "title is final fallback",
			fields: content.TextFields{Title: "<p>Title</p>"},
			want:   "Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, content.AvailableText(tt.fields))
		})
	}
}

func TestAvailableTextStripsTagsAndNormalizesWhitespace(t *testing.T) {
	got := content.AvailableText(content.TextFields{
		Content: "\n<article><p> First&nbsp; paragraph </p><p>Second<br>\tline</p></article>",
	})

	assert.Equal(t, "First paragraph Second line", got)
}

func TestAvailableTextStripsVoidElementsWithoutDroppingFollowingText(t *testing.T) {
	got := content.AvailableText(content.TextFields{
		Content: `<img src="https://tracker.example/pixel.png"><p>Article text</p><input value="ignored"> tail`,
	})

	assert.Equal(t, "Article text tail", got)
}

func TestAvailableTextReturnsEmptyWhenNoSafeTextExists(t *testing.T) {
	tests := []struct {
		name   string
		fields content.TextFields
	}{
		{name: "all fields empty"},
		{
			name: "only unsafe or whitespace markup",
			fields: content.TextFields{
				Content:     `<script>alert("x")</script>`,
				ShowNotes:   "\t\n ",
				Description: "<p> </p>",
				Summary:     "<br>",
				Title:       " ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, content.AvailableText(tt.fields))
		})
	}
}
