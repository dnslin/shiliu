# Design

## Architecture

This is a cross-layer backend feature:

`HTTP route -> handler -> content item service -> content item repository -> SQLite aggregate read -> service Markdown mapping -> JSON envelope`

The repository owns the aggregate read because the export needs data spanning content items, feeds, folders, and tags. The service owns export business rules and Markdown formatting. The handler stays thin and maps HTTP boundary concerns.

## API Contract

Add API response structs in `api/v1/content_item.go`:

- `ExportContentItemObsidianResponseData`
  - `contentItemId uint`
  - `filename string`
  - `markdown string`
- `ExportContentItemObsidianResponse`
  - existing `Response`
  - `Data ExportContentItemObsidianResponseData`

Add route:

- `GET /api/v1/content-items/{id}/obsidian-export`

The route is registered in the existing strict-auth content item router group.

## Repository Contract

Add an export-specific read model to `internal/repository/content_item.go`:

- `ContentItemExportData`
  - content item id, title, type, available text, published at, AI summary fields
  - feed title and feed URL
  - folder name nullable
  - tag names slice

Add method:

- `GetExportDataByID(ctx context.Context, id uint) (*ContentItemExportData, error)`

Implementation details:

- Reject zero id with `v1.ErrBadRequest`.
- Use real SQLite/GORM reads.
- Main row joins `content_items` to `feeds` and left joins `folders`.
- Missing content item maps to `v1.ErrNotFound`.
- Tag names are loaded from `content_item_tags JOIN tags` ordered by `tags.name ASC, tags.id ASC`.
- If a content item has no tags, return an empty slice rather than nil-sensitive behavior.

## Service Contract

Extend `ContentItemService`:

- `ExportObsidianMarkdown(ctx context.Context, id uint) (*v1.ExportContentItemObsidianResponseData, error)`

Service rules:

- Validate id through repository method.
- Do not call AI config repository.
- Do not call chat completion.
- Convert repository export data into Markdown through a small formatter function owned by the service package.
- Truncate available text by Unicode runes at 2000.
- Produce deterministic Markdown for tests and frontend copy/download behavior.

## Markdown Shape

The exact shape may be refined during TDD, but it must be stable and human-readable:

```markdown
# <title>

## 元信息

- 标题：<title>
- 链接：<feed url or item link if available>
- 订阅源：<feed title>
- 发布时间：<RFC3339 or empty marker>
- 内容类型：<text|audio>
- 标签：<comma-separated names or 无>
- 订阅源文件夹：<folder name or 无>

## AI 摘要

<summary markdown or state-specific text>

## 可用文本摘录

<first 2000 runes>
<optional truncation notice>
```

Current schema does not expose an item-level original link in `model.ContentItem`; feed URL is the available link-like persisted source in the current model. If an item link field exists before implementation, use it. Otherwise, record the feed URL under the link metadata without adding a schema migration.

## AI Summary State Mapping

- `success`: current `ai_summary_markdown`, or an empty-state line if the markdown is unexpectedly empty.
- `none`: `未生成`.
- `pending`: `正在生成`.
- `failed`: `生成失败` plus `ai_summary_error` when present.
- `insufficient_text`: `可用文本不足`.

Unknown status should be treated as an export formatting failure rather than silently producing misleading output.

## Filename

Generate a suggested filename from title:

- trim spaces.
- replace path-invalid characters with `-`.
- collapse empty result to `content-item-<id>`.
- append `.md`.

This is a helper detail inside service formatting and should be covered by export behavior tests only where observable.

## Compatibility

- No database migration is required for the known current schema.
- Adding a repository interface method requires regenerating `test/mocks/repository/content_item.go`.
- Adding public route/response annotations requires `swag init -g cmd/server/main.go -o ./docs`.
- Wire generation is not expected because constructors/providers do not change.

## Error Handling

- Handler maps `v1.ErrBadRequest` to `400`.
- Handler maps `v1.ErrContentItemNotFound` / `v1.ErrNotFound` to `404`.
- Handler maps `v1.ErrExportFailed` to `500` unless a more specific mapping is added.
- Unexpected errors are logged and returned as `v1.ErrInternalServerError`.

## Risks

- Current model lacks item-level original link. The implementation must not invent a migration unless inspection finds an existing field.
- Markdown formatting can become brittle if tests assert every blank line. Prefer behavior-significant substrings plus focused formatter tests for exact truncation.
- Adding a method to `ContentItemRepository` affects generated mocks and test doubles in service/feed tests; update all compile failures deliberately.
