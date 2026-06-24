package pi_test

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTemplateHashesExcludeRuntimeStateAndBytecode(t *testing.T) {
	content, err := os.ReadFile("../../.trellis/.template-hashes.json")
	if err != nil {
		t.Fatalf("read template hashes: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("parse template hashes: %v", err)
	}
	hashes, ok := payload["hashes"].(map[string]any)
	if !ok {
		t.Fatalf("template hashes payload must contain hashes object")
	}
	for path := range hashes {
		if strings.Contains(path, ".trellis/.runtime/") || strings.Contains(path, "__pycache__") || strings.HasSuffix(path, ".pyc") {
			t.Fatalf("template hash manifest must not track runtime or bytecode path %q", path)
		}
	}
}

func TestArchivedPR35ContextManifestsUseFileField(t *testing.T) {
	for _, path := range []string{
		"../../.trellis/tasks/archive/2026-06/06-23-pr-35-code-review-fixes/implement.jsonl",
		"../../.trellis/tasks/archive/2026-06/06-23-pr-35-code-review-fixes/check.jsonl",
	} {
		t.Run(path, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open manifest: %v", err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			lineNo := 0
			for scanner.Scan() {
				lineNo++
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				var row map[string]any
				if err := json.Unmarshal([]byte(line), &row); err != nil {
					t.Fatalf("line %d invalid JSON: %v", lineNo, err)
				}
				file, ok := row["file"].(string)
				if !ok || strings.TrimSpace(file) == "" {
					t.Fatalf("line %d must contain non-empty file field: %s", lineNo, line)
				}
				if _, ok := row["path"]; ok {
					t.Fatalf("line %d must not use stale path field: %s", lineNo, line)
				}
				if _, ok := row["type"]; ok {
					t.Fatalf("line %d must not use stale type field: %s", lineNo, line)
				}
				if strings.Contains(file, "/06-23-pr-35-code-review-fixes/") && !strings.Contains(file, "/archive/2026-06/") {
					t.Fatalf("line %d task artifact path must use archived location: %s", lineNo, line)
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan manifest: %v", err)
			}
		})
	}
}
