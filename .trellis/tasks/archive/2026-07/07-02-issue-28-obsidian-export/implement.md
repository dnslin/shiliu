# Implementation Plan

## Branch

- Create and work on `issue-28-obsidian-export` after planning review.
- Set Trellis task branch metadata after branch creation.

## TDD Vertical Slices

Follow tracer-bullet TDD. Do not write all tests first.

### Slice 1: Repository Aggregate Read

1. RED: Add a repository test over migrated SQLite proving `GetExportDataByID` returns content item fields, feed title/url, folder name, and sorted tag names.
2. GREEN: Add repository read model and method.
3. Verify focused repository test package.

### Slice 2: Service Markdown For Success Summary

1. RED: Add a service test for a successful summary export with metadata, tags, folder, summary markdown, and non-truncated excerpt.
2. GREEN: Add `ExportObsidianMarkdown` service method and formatter for that behavior.
3. Verify focused service test package.

### Slice 3: Summary State Matrix

1. RED: Add service table test for `none`, `pending`, `failed`, and `insufficient_text`.
2. GREEN: Implement state text mapping, including failed error text.
3. Verify focused service test package.

### Slice 4: Available Text Truncation

1. RED: Add service test for exactly first 2000 Unicode characters plus truncation notice, and a non-truncated case without the notice.
2. GREEN: Implement rune-safe truncation.
3. Verify focused service test package.

### Slice 5: Handler Route Contract

1. RED: Add handler test asserting `GET /content-items/:id/obsidian-export` calls the service and returns JSON envelope with `contentItemId`, `filename`, and `markdown`.
2. GREEN: Add API response structs, handler method, route, and fake service method updates.
3. Verify focused handler test package.

### Slice 6: Integration-Style Handler With Real Service

1. RED: Add handler test over migrated SQLite proving export works end-to-end through real service/repository for tags and folder metadata.
2. GREEN: Fill any gaps from the real path.
3. Verify focused handler test package.

### Slice 7: Generated/Contract Artifacts

1. Regenerate content item repository mock via existing Makefile/mockgen path.
2. Regenerate Swagger docs with `swag init -g cmd/server/main.go -o ./docs`.
3. Run compile-focused commands and fix drift.

## Files Likely To Change

- `api/v1/content_item.go`
- `internal/repository/content_item.go`
- `internal/service/content_item.go`
- `internal/handler/content_item.go`
- `internal/router/content_item.go`
- `test/mocks/repository/content_item.go`
- `test/server/repository/feed_content_test.go`
- `internal/service/content_item_test.go`
- `test/server/handler/content_item_test.go`
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`

## Validation Commands

Focused during TDD:

```bash
go test ./test/server/repository -run Export
go test ./internal/service -run Export
go test ./test/server/handler -run Export
```

Final:

```bash
go build ./...
go vet ./...
go test ./...
git diff --check
```

Conditional:

```bash
mockgen -source=internal/repository/content_item.go -destination test/mocks/repository/content_item.go
swag init -g cmd/server/main.go -o ./docs
```

`nunu wire all` is only required if provider sets, constructors, or generated entrypoints change unexpectedly.

## Review Gates

- Planning artifacts reviewed before `task.py start`.
- Never implement while RED beyond the current vertical slice.
- Do not add AI service calls to export.
- Do not add schema migrations unless implementation discovers an existing persisted item-level link is impossible and product scope changes.
