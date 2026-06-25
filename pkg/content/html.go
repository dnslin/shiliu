package content

import (
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var htmlPolicy = newHTMLPolicy()

// TextFields contains the feed text candidates used to derive available_text.
// AvailableText accepts raw or already sanitized HTML and always extracts text
// from sanitized content before applying the fallback order.
type TextFields struct {
	Content     string
	ShowNotes   string
	Description string
	Summary     string
	Title       string
}

// This mirrors UGCPolicy's text/link/list/table subset instead of starting from
// UGCPolicy directly: bluemonday cannot remove already-allowed void elements
// like img without risking skipped following text.
func newHTMLPolicy() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowStandardURLs()
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowAttrs("cite").OnElements("blockquote", "q")
	policy.AllowElements(
		"abbr", "acronym", "article", "aside", "b", "blockquote", "br", "cite", "code",
		"dd", "details", "dfn", "div", "dl", "dt", "em", "figcaption", "figure", "h1",
		"h2", "h3", "h4", "h5", "h6", "hgroup", "hr", "i", "mark", "p", "pre", "q",
		"rp", "rt", "ruby", "s", "samp", "section", "small", "span", "strike", "strong",
		"sub", "summary", "sup", "time", "tt", "u", "var", "wbr",
	)
	policy.AllowLists()
	policy.AllowTables()
	policy.SkipElementsContent("iframe", "math", "object", "script", "style", "svg")
	return policy
}

// SanitizeHTML returns safe HTML for feed-provided text using Shiliu's single
// package-level allowlist policy.
func SanitizeHTML(raw string) string {
	if raw == "" {
		return ""
	}
	return htmlPolicy.Sanitize(raw)
}

// AvailableText returns normalized plain text using the priority order
// content, show_notes, description, summary, then title.
func AvailableText(fields TextFields) string {
	candidates := [...]string{fields.Content, fields.ShowNotes, fields.Description, fields.Summary, fields.Title}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		text := plainTextFromSanitizedHTML(SanitizeHTML(candidate))
		if text != "" {
			return text
		}
	}
	return ""
}

func plainTextFromSanitizedHTML(safeHTML string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(safeHTML))
	var text strings.Builder
	lastWasSpace := true
	wrote := false

	writeSpace := func() {
		if wrote && !lastWasSpace {
			text.WriteByte(' ')
			lastWasSpace = true
		}
	}
	writeText := func(value []byte) {
		for _, r := range string(value) {
			if unicode.IsSpace(r) {
				writeSpace()
				continue
			}
			text.WriteRune(r)
			wrote = true
			lastWasSpace = false
		}
	}

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(text.String())
		case html.TextToken:
			writeText(tokenizer.Text())
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			tagName, _ := tokenizer.TagName()
			if createsTextBoundary(string(tagName)) {
				writeSpace()
			}
		}
	}
}

func createsTextBoundary(tagName string) bool {
	switch tagName {
	case "address", "article", "aside", "blockquote", "br", "caption", "col", "colgroup", "dd", "details", "div", "dl", "dt", "figcaption", "figure", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hgroup", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "summary", "table", "tbody", "td", "tfoot", "th", "thead", "tr", "ul":
		return true
	default:
		return false
	}
}
