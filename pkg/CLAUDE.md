# Packages

## Purpose

`pkg` contains reusable infrastructure and utility code shared by backend layers.

## Patterns

- Keep package APIs small and behavior-focused.
- Prefer pure functions for content helpers.
- Keep server/app packages generic enough for entrypoints, but not wider than current runtime needs.
- Shared trust-boundary logic belongs here when multiple services need it.

## Boundaries

- `pkg/content` owns reusable sanitization and text extraction.
- `pkg/server` owns generic HTTP/gRPC server plumbing.
- `pkg/app` owns lifecycle orchestration.
- `pkg/config`, `pkg/log`, `pkg/jwt`, and `pkg/sid` should stay infrastructure-focused.
- Domain vocabulary belongs in `internal` or `api/v1`, not generic packages.

## Content Utilities

- HTML sanitization uses one shared policy.
- Available text is plain text after sanitization and whitespace normalization.
- Text fallback order is `content`, `show_notes`, `description`, `summary`, then `title`.
- Higher-priority unsafe-only input must fall through.
- Keep dangerous tag/URL handling covered by table tests.

## Testing

- Test public package APIs, not internal policy objects.
- Cover malicious HTML, dangerous URLs, disallowed tags, and whitespace normalization.
- Package changes should run `go test ./pkg/...` at minimum.
- Lifecycle package changes need cancellation and startup-error tests.

## Anti-Patterns

- Do not create grab-bag helpers for single-use logic.
- Do not branch sanitizer behavior by feed URL or source name.
- Do not put business workflow decisions in generic packages.
- Do not mutate global package state during tests without cleanup.
- Do not make reusable packages import `internal/handler` or repository code.
