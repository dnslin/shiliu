# Issue 28 Obsidian Markdown export

## Goal

Implement single-content-item Obsidian Markdown export for Shiliu.

The export lets the user copy or download one content item as plain Markdown that preserves context from the content item, its subscription feed, assigned tags, feed folder, current AI summary state, and available-text excerpt.

## Source Of Truth

- GitHub issue: #28 `[切片27] Obsidian 单条 Markdown 导出`
- Parent PRD: #1 `拾流订阅中心 MVP 后端实现 PRD`
- Local issue copy: `.trellis/tasks/06-17-shiliu-subscription-center/issues/slice-27.md`
- Local design boundary: `.trellis/tasks/06-17-shiliu-subscription-center/design.md`, "Obsidian Export Boundary"

## Confirmed Facts

- Existing schema already contains the required content item fields: title, type, link-adjacent feed data, published time, `available_text`, current AI summary markdown, AI summary status, generated time, and error.
- Existing schema already contains tags and feed folders through `content_item_tags`, `tags`, `feeds.folder_id`, and `folders`.
- Existing errors reserve `v1.ErrExportFailed`.
- Existing content item routes are JWT-protected under `/api/v1`.
- Existing API contract uses JSON envelopes: `{ "code": 0, "message": "ok", "data": ... }`.
- Export must be a pure mapping. It must not call AI config, AI service, chat completion, or summary generation.

## Requirements

1. Add a public content-item export endpoint:
   - `GET /content-items/{id}/obsidian-export`
   - route is protected by the same strict auth group as other content item routes.
   - response uses the existing JSON envelope.
2. Response data includes:
   - `contentItemId`
   - `filename`
   - `markdown`
3. Markdown includes ordinary Markdown metadata, not frontmatter:
   - title
   - original content or episode link when available
   - subscription feed name
   - published time
   - content type
   - assigned tag names
   - subscription feed folder name, or a clear empty value when no folder exists
4. Markdown always includes an `AI 摘要` section:
   - `success`: write current summary Markdown.
   - `none`: write that no summary has been generated.
   - `pending`: write that the summary is being generated.
   - `failed`: write that generation failed, including the current summary error when present.
   - `insufficient_text`: write that available text is insufficient.
5. Markdown includes an available-text excerpt section:
   - use the first 2000 characters of `available_text`, counting Unicode runes rather than bytes.
   - when truncated, append `已截断，请打开原文链接查看完整内容`.
   - when not truncated, do not append the truncation message.
6. Tag names appear in deterministic order.
7. Export succeeds for every current AI summary state and does not require AI service configuration.
8. Missing or invalid content item ids map to existing content item error semantics:
   - malformed / zero id -> `400`.
   - missing item -> `404` with content item not found code.
   - unexpected export failure -> mapped through `v1.ErrExportFailed` or generic `500`, depending on whether the failure is domain-known or unexpected.

## Public Interface Decision

Use a JSON export response instead of a direct `text/markdown` file response for this slice.

Reason: existing API contract is JSON-envelope based, and a single `markdown` string supports both copy and frontend-driven download. The response also carries `filename` without introducing a second content-negotiation path.

## Acceptance Criteria

- [ ] Export response includes `contentItemId`, `filename`, and `markdown`.
- [ ] Export Markdown includes title / link / subscription feed / published time / content type / tags / feed folder metadata.
- [ ] Export Markdown never uses Obsidian frontmatter.
- [ ] Export Markdown always includes `AI 摘要`.
- [ ] `success` summary state writes current summary Markdown.
- [ ] `none`, `pending`, `failed`, and `insufficient_text` summary states write the corresponding status text.
- [ ] Available text excerpt is at most the first 2000 Unicode characters.
- [ ] Truncated excerpts append `已截断，请打开原文链接查看完整内容`.
- [ ] Non-truncated excerpts do not append the truncation message.
- [ ] Export does not call AI service configuration or chat completion.
- [ ] Service tests cover all summary states, truncated / non-truncated excerpts, tags, and folder metadata.
- [ ] Handler tests assert the export HTTP response envelope.
- [ ] Relevant repository tests cover the aggregate export read over real SQLite.
- [ ] `go test` for related packages passes.
- [ ] Final backend gates run before completion: `go build ./...`, `go vet ./...`, `go test ./...`, `git diff --check`.

## Out Of Scope

- Obsidian frontmatter.
- Obsidian plugin integration.
- Vault synchronization.
- Template configuration.
- Full-text export beyond the 2000-character available-text excerpt.
- Auto-generating or retrying AI summary during export.
- Adding a separate binary/file download endpoint in this slice.
