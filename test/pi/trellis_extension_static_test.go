package pi_test

import (
	"os"
	"strings"
	"testing"
)

func readExtension(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("../../.pi/extensions/trellis/index.ts")
	if err != nil {
		t.Fatalf("read extension: %v", err)
	}
	return string(content)
}

func TestTrellisExtensionRejectsPathTraversalSources(t *testing.T) {
	src := readExtension(t)

	for _, forbidden := range []string{
		"readText(join(root, f))",
		"existsSync(join(root, \".pi\", \"agents\", `${agent}.md`))",
		"readText(join(root, \".pi\", \"agents\", `${agent}.md`))",
		"readText(join(root, \".pi\", \"agents\", `${agentName}.md`))",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("extension still contains unsafe path construction %q", forbidden)
		}
	}

	for _, required := range []string{
		"resolveInsideRoot",
		"resolveAgentFile",
		"resolveManifestFile",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("extension must define/use %s to keep paths inside the repo", required)
		}
	}
}

func TestTrellisExtensionDoesNotRewriteBashCommands(t *testing.T) {
	src := readExtension(t)

	if strings.Contains(src, "export TRELLIS_CONTEXT_ID") {
		t.Fatalf("extension must not prepend POSIX export syntax to bash commands")
	}
	if strings.Contains(src, "ev.input.command =") {
		t.Fatalf("extension must pass Trellis context through tool metadata/env, not by mutating command text")
	}
	if !strings.Contains(src, "ev.input.env") {
		t.Fatalf("extension should set bash env metadata for TRELLIS_CONTEXT_ID")
	}
}

func TestTrellisExtensionAvoidsBarePiWindowsShimFallback(t *testing.T) {
	src := readExtension(t)

	if strings.Contains(src, "return { command: \"pi\", args: [] }") {
		t.Fatalf("extension must not fall back to bare pi with shell:false; Windows npm shims need an explicit executable")
	}
	if !strings.Contains(src, "pi.cmd") {
		t.Fatalf("extension should probe platform-specific Pi CLI shim names")
	}
}

func TestTrellisExtensionCancellationTargetsProcessTree(t *testing.T) {
	src := readExtension(t)

	if !strings.Contains(src, "detached:") && !strings.Contains(src, "taskkill") {
		t.Fatalf("subagent cancellation must account for child tool processes, not only the direct Pi process")
	}
}

func TestTrellisExtensionRendersFinalSubagentText(t *testing.T) {
	src := readExtension(t)

	if !strings.Contains(src, ".final") || !strings.Contains(src, "result.content?.[0]?.text") {
		t.Fatalf("renderResult should render final subagent text, not only the progress card")
	}
}

func TestTrellisExtensionDoesNotExecuteRepoPythonForContext(t *testing.T) {
	src := readExtension(t)

	if strings.Contains(src, "spawnSync(py, [script]") || strings.Contains(src, "get_context.py") {
		t.Fatalf("extension hooks must not execute repository-controlled Python while injecting context")
	}
}

func TestTrellisExtensionPreservesActiveTaskFirstLine(t *testing.T) {
	src := readExtension(t)

	if !strings.Contains(src, "activeTaskLine") {
		t.Fatalf("buildPrompt should preserve a delegated Active task line before its own prelude")
	}
}

func TestTrellisExtensionFailureOutputPrefersErrors(t *testing.T) {
	src := readExtension(t)

	if !strings.Contains(src, "finalizeError") {
		t.Fatalf("failed child process output should prefer stderr/error text over stale assistant text")
	}
}
