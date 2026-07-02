# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

### Scenario: Go Module Path and Wire Regeneration

#### 1. Scope / Trigger
- Trigger: changing `go.mod`, internal Go import paths, Wire provider sets, or generated Wire files.
- Applies to the backend module root and the Wire entrypoints under `cmd/server/wire`, `cmd/task/wire`, and `cmd/migration/wire`.

#### 2. Signatures
- Module declaration: `module shiliu` in `go.mod`.
- Primary generation command: `nunu wire all`.
- Per-entry fallback only after a tool panic: `cd cmd/<entrypoint>/wire && go run -mod=mod github.com/google/wire/cmd/wire`.

#### 3. Contracts
- All internal imports must use `shiliu/...`.
- `wire_gen.go` files are generated artifacts; do not hand-edit them.
- If `go.sum` changes during Wire generation, verify the additions belong to the Go/Wire toolchain before committing.

#### 4. Validation & Error Matrix
- `shuliu/` found in any `.go` file -> fail the slice and replace the stale import.
- `nunu wire all` fails transiently -> retry once after capturing the error.
- A single Wire entrypoint panics after retry -> generate only that entrypoint with the per-entry fallback, then rerun `nunu wire all` to prove the full command is reproducible.
- `go build ./...`, `go vet ./...`, or `go test ./...` fails -> fix before reporting completion.

#### 5. Good/Base/Bad Cases
- Good: `go.mod` says `module shiliu`, all internal imports are `shiliu/...`, all three `wire_gen.go` files are regenerated, and build/vet/test pass.
- Base: non-Wire Go import replacement still requires `nunu wire all`, because generated code may contain stale import strings.
- Bad: manually editing `wire_gen.go` without rerunning Wire, or committing a stale `shuliu/...` import in tests.

#### 6. Tests Required
- Search assertion: no `shuliu/` remains in `*.go` files.
- Generation assertion: `nunu wire all` exits successfully and touches all three Wire entrypoints when needed.
- Build assertion: `go build ./...` succeeds.
- Vet assertion: `go vet ./...` succeeds.
- Test assertion: `go test ./...` succeeds so existing test packages still compile.

#### 7. Wrong vs Correct

Wrong:
```go
import "shuliu/internal/repository"
```

Correct:
```go
import "shiliu/internal/repository"
```

Wrong:
```text
Edit cmd/server/wire/wire_gen.go by hand to fix imports.
```

Correct:
```text
Run nunu wire all and commit the generated wire_gen.go output.
```

---

## Testing Requirements

### Scenario: Handler error mapping and untrusted token-subject parsing

#### 1. Scope / Trigger
- Trigger: a handler maps service/repository errors to HTTP status; a service parses a JWT subject / context user id into a domain integer type.
- Applies to `internal/handler/*.go`, `internal/service/*.go`, and `api/v1/errors.go` error definitions.

#### 2. Signatures
- Error inspection: `errors.Is(err, v1.ErrNotFound)` etc. (errors must be compared by identity, not message).
- Safe id parse: `strconv.ParseUint(userId, 10, strconv.IntSize)` then `uint(id)`.

#### 3. Contracts
- A handler must not map every service error to a single status. Map known domain errors explicitly and reserve `500` for genuine server faults:
  - `v1.ErrUsernameAlreadyUse` → `409 Conflict`
  - `v1.ErrNotFound` → `404 Not Found`
  - `v1.ErrBadRequest` → `400 Bad Request`
  - default/unknown → `500 Internal Server Error`
- A normal client error (duplicate username, missing record for a valid token) is not a server fault and must not be logged at `Error` level; log only the default/unknown branch.
- A JWT subject / context user id is untrusted input even when the token signature is valid. Parse it with the exact platform width (`strconv.IntSize`) before converting to `uint`, so an out-of-range value is rejected instead of silently truncating on 32-bit targets. The service should translate a parse failure to `v1.ErrBadRequest` so the handler never reaches a repository lookup with a bad id.

#### 4. Validation & Error Matrix
- Duplicate username at register → service returns `v1.ErrUsernameAlreadyUse`, handler returns `409` with code `1001`.
- Valid numeric token subject for a missing/deleted user → repo `v1.ErrNotFound` → handler `404`.
- Non-numeric / negative / `>= 2^IntSize` subject → service `v1.ErrBadRequest` → handler `400`, with no repository call.
- Unexpected error (e.g. DB failure) → handler `500`, logged at `Error`.

#### 5. Good/Base/Bad Cases
- Good: handler `switch`/`errors.Is` maps each known error to its status; default is `500`.
- Bad: `if err != nil { HandleError(ctx, http.StatusBadRequest, ...) }` — collapses not-found and server faults into `400`.
- Bad: `strconv.ParseUint(userId, 10, 64)` then `uint(id)` — truncates on 32-bit builds.

#### 6. Tests Required
- Handler test asserts the exact HTTP status and envelope `code`/`message` for each mapped error, through the real handler method + middleware seam.
- Service test asserts a malformed/out-of-range id is rejected (`v1.ErrBadRequest`) and that the repository mock is never called (gomock fails on an unexpected call).
- Service test asserts `v1.ErrNotFound` from the repository is propagated unchanged.

#### 7. Wrong vs Correct

Wrong:
```go
user, err := h.userService.GetProfile(ctx, userId)
if err != nil {
    v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil) // 404 and 500 both become 400
    return
}
```

Correct:
```go
user, err := h.userService.GetProfile(ctx, userId)
if err != nil {
    switch {
    case errors.Is(err, v1.ErrNotFound):
        v1.HandleError(ctx, http.StatusNotFound, v1.ErrNotFound, nil)
    case errors.Is(err, v1.ErrBadRequest):
        v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
    default:
        h.logger.WithContext(ctx).Error("userService.GetProfile error", zap.Error(err))
        v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
    }
    return
}
```

---

### Scenario: API Contract and Route Registration Tests

#### 1. Scope / Trigger
- Trigger: changing backend API route prefixes, Swagger/OpenAPI runtime metadata, response envelope helpers, pagination helpers, or any frontend-facing JSON contract.
- Applies to `api/v1`, `internal/server`, `internal/router`, handler tests, and any test that claims to verify a public API contract.

#### 2. Signatures
- Runtime server constructor: `func NewHTTPServer(deps router.RouterDeps) *http.Server`.
- API response envelope: `v1.Response{Code, Message, Data}` serialized as `{"code":...,"message":"...","data":...}`.
- Pagination request contract: `page` and `pageSize` query fields normalized by the shared API helper.
- Route prefix contract: production routes are registered by `NewHTTPServer` and router init functions, not by ad-hoc test-only groups.

#### 3. Contracts
- Route prefix tests must exercise the same registration path production uses. For server-level route prefixes, instantiate `NewHTTPServer` with minimal test dependencies and send HTTP requests through the returned handler.
- Swagger/BasePath assertions must read the runtime metadata set by the real server setup.
- Response and pagination helper tests may use focused Gin test contexts because the helper itself is the public seam.
- Tests must assert observable HTTP behavior: status code, JSON fields, registered route exists, old route does not exist when the contract intentionally changed.
- `PageRequest.LimitOffset` must avoid integer overflow for every normalized `pageSize`, including `pageSize=1`; never compute `math.MaxInt + 1` while deriving the maximum safe page.

#### 4. Validation & Error Matrix
- Test recreates `router.Group("/api/v1")` locally and then asserts `/api/v1` works -> invalid test; it proves only the test setup.
- Test calls `NewHTTPServer` and asserts `/api/v1/register` reaches the registered handler while `/v1/register` returns 404 -> valid route-prefix contract test.
- Test checks only `docs.SwaggerInfo.BasePath` but not an HTTP route -> incomplete when route registration also changed.
- Test checks only an HTTP route but not Swagger/BasePath after a BasePath change -> incomplete when runtime docs metadata also changed.
- `page=2&pageSize=1` -> `limit=1, offset=1`; a huge page with any valid `pageSize` -> clamp to a representable page before `(page-1)*pageSize`.

#### 5. Good/Base/Bad Cases
- Good: `NewHTTPServer(testDeps)` is used, `/api/v1/<route>` returns the expected handler-level status, `/v1/<route>` is not registered, and `docs.SwaggerInfo.BasePath` matches the new prefix.
- Base: focused helper tests construct `gin.CreateTestContext` for `ParsePageRequest` or `HandleSuccess`, because those helpers are the unit under test.
- Bad: a route-prefix test creates a local `gin.New()` and manually registers the expected group; this is tautological and can pass while production server setup is wrong.

#### 6. Tests Required
- Changed response helper -> test serialized envelope fields and HTTP status through a Gin response recorder.
- Changed pagination helper -> test defaults, boundary normalization, malformed query fallback, `LimitOffset`, `pageSize=1` overflow regression, and `total=0` metadata.
- Changed route prefix -> test the real server/router registration path accepts the new prefix and rejects the old prefix when no compatibility alias is intended.
- Changed Swagger runtime metadata -> test the runtime `docs.SwaggerInfo.BasePath` value after server setup.

#### 7. Wrong vs Correct

Wrong:
```go
router := gin.New()
api := router.Group("/api/v1")
api.GET("/contract", handler)
// This only proves the test registered /api/v1, not production code.
```

Correct:
```go
server := NewHTTPServer(newTestRouterDeps())
request := httptest.NewRequest(http.MethodPost, "/api/v1/register", nil)
response := httptest.NewRecorder()
server.ServeHTTP(response, request)
require.NotEqual(t, http.StatusNotFound, response.Code)
require.Equal(t, "/api/v1", docs.SwaggerInfo.BasePath)
```

Wrong:
```go
maxPage := math.MaxInt/page.PageSize + 1 // overflows when pageSize == 1
```

Correct:
```go
maxPage := math.MaxInt / page.PageSize
if maxPage < math.MaxInt {
    maxPage++
}
```

### Scenario: OPML Import API Contract

#### 1. Scope / Trigger
- Trigger: adding or changing OPML import parsing, feed batch-import request/response DTOs, routes, or handler/service behavior.
- Applies to `api/v1/feed.go`, `internal/handler/feed.go`, `internal/router/feed.go`, `internal/service/feed*.go`, Swagger docs, and service/handler/server tests.

#### 2. Signatures
- Route: `POST /api/v1/feeds/import-opml` in the strict authenticated feed route group.
- JSON request:
  ```json
  { "opml": "<opml xml text>" }
  ```
- Multipart request:
  - file field: `file`
  - optional text fallback field: `opml`
- Service method:
  ```go
  ImportOPML(ctx context.Context, req *v1.ImportOPMLRequest) (*v1.ImportOPMLResponseData, error)
  ```
- Response data:
  ```go
  type ImportOPMLResponseData struct {
      Total     int `json:"total"`
      Success   int `json:"success"`
      Duplicate int `json:"duplicate"`
      Failed    int `json:"failed"`
  }
  ```

#### 3. Contracts
- OPML import is a one-time batch import, not OPML sync.
- The parser reads only feed URL attributes from `outline` elements: `xmlUrl`, `xmlURL`, then `url`.
- OPML folder/group hierarchy and outline display fields such as `text` or `title` must not create Shiliu folders or assign feeds to folders.
- Each distinct normalized feed URL must reuse the existing single-feed creation path; do not duplicate fetch, parse, sanitize, dedupe, or persistence logic in import code.
- Duplicate normalized URLs inside the same OPML payload count as duplicates and must not be fetched.
- Existing subscription feeds count as duplicates and must not be recreated.
- Failed fetch/parse/invalid URL candidates count as failed and must not create empty subscription feeds.
- The batch returns count totals in the normal JSON envelope; per-feed details are out of scope unless the product contract changes.

#### 4. Validation & Error Matrix
- Empty request, malformed XML, or OPML with no feed URL candidates -> `v1.ErrOPMLInvalid`, HTTP `400`.
- Invalid individual feed URL candidate -> increment `failed`, continue importing remaining candidates.
- Existing normalized feed URL -> increment `duplicate`, continue.
- Duplicate normalized feed URL within the same request -> increment `duplicate`, do not fetch.
- Individual fetch or parse failure -> increment `failed`, do not create a feed record.
- Request cancellation or deadline -> stop and return the context error.
- Unexpected repository/service failure -> wrap as `v1.ErrOPMLImportFailed`, HTTP `500`.

#### 5. Good/Base/Bad Cases
- Good: handler accepts both pasted JSON and multipart upload, service streams OPML XML, normalizes candidates, reuses `CreateFeed`, and tests prove success/duplicate/failed counts.
- Base: nested folder outlines are ignored while child feed outlines are imported.
- Bad: OPML import directly writes `feeds` or `content_items`, bypassing `CreateFeed`.
- Bad: OPML folder names create Shiliu folders or assign imported feeds.
- Bad: a repeated URL in the same OPML payload is fetched twice.

#### 6. Tests Required
- Service test with Fetcher fixtures proving all-success import persists feeds and initial content through the existing fetch pipeline.
- Service test covering mixed success, existing duplicate, in-payload duplicate, parse/fetch failure, and invalid URL counts.
- Service test covering invalid OPML input and no-feed-url OPML.
- Handler httpexpect tests for JSON pasted OPML and multipart file upload response counts.
- Handler test for `ErrOPMLInvalid` status mapping.
- Server route/auth test proving `POST /api/v1/feeds/import-opml` is registered under strict auth.
- Swagger test proving the path and `v1.ImportOPMLResponse` schema are documented after `swag init`.

#### 7. Wrong vs Correct

Wrong:
```go
for _, url := range urls {
    feedRepo.Create(ctx, &model.Feed{FeedURL: url}) // bypasses fetch/parse and creates empty feeds
}
```

Correct:
```go
for _, feedURL := range normalizedUniqueURLs {
    _, err := s.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: feedURL})
    // count success, duplicate, and per-feed failures
}
```

Wrong:
```go
folderID := folderRepo.Create(ctx, outline.Text)
feedRepo.AssignFolder(ctx, feedID, &folderID) // OPML folder mapping is out of scope
```

Correct:
```go
// Only outline feed URL attributes are read; OPML grouping metadata is ignored.
```

### Scenario: Feed HTML Sanitization and Available Text

#### 1. Scope / Trigger
- Trigger: adding or changing feed-provided HTML sanitization, safe text fields, or `available_text` derivation.
- Applies to reusable `pkg` content utilities and any fetch service that prepares `description_safe`, `content_safe`, `show_notes_safe`, or `available_text` before persistence.

#### 2. Signatures
- Sanitizer entrypoint: `func SanitizeHTML(raw string) string`.
- Available text entrypoint: `func AvailableText(fields TextFields) string`.
- Text candidates: `TextFields{Content, ShowNotes, Description, Summary, Title}`.

#### 3. Contracts
- HTML sanitization is a package-level trust boundary: one shared bluemonday policy, not per-source or per-site branches.
- Sanitized HTML may keep safe text structure and links, but must strip script/style/object/iframe/svg/math content and disallowed media/form tags.
- `available_text` is always plain text extracted after sanitization, with normalized whitespace and priority `content` -> `show_notes` -> `description` -> `summary` -> `title`.
- A higher-priority field that sanitizes to empty text must fall through to the next candidate.

#### 4. Validation & Error Matrix
- `<script>` / event attributes / `javascript:` URL -> removed from sanitized output.
- Disallowed void tag before safe text, such as `<img><p>Article</p>` -> tag removed, following safe text preserved.
- All candidates empty, whitespace-only, or unsafe-only -> `available_text == ""`.
- Multiple whitespace forms and HTML block boundaries -> one-space normalized plain text.
- Allowed structural tags such as `<details><summary>...` and `<hgroup>...` -> visible adjacent text remains separated.

#### 5. Good/Base/Bad Cases
- Good: a single package function sanitizes all feed HTML, and `AvailableText` strips tags from that sanitized output before applying fallback order.
- Base: `description` and `summary` are separate fallback candidates; `description` wins when both are present.
- Bad: branching on feed URL, host, source name, or site-specific quirks to change sanitization behavior.
- Bad: putting void elements such as `img`, `input`, or `source` in `SkipElementsContent`; bluemonday can enter skip-content mode and drop following safe text when the input uses normal non-self-closing HTML.

#### 6. Tests Required
- Sanitizer test through the public package API asserts malicious tags, event attributes, media/form tags, and dangerous URLs are removed.
- Available-text tests assert each fallback level, whitespace normalization, unsafe-only empty output, allowed structural-tag boundaries, and the void-element preservation case.
- Tests must not inspect policy internals; they verify public behavior only.

#### 7. Wrong vs Correct

Wrong:
```go
policy := bluemonday.UGCPolicy()
policy.SkipElementsContent("img", "input", "script") // img can swallow following text
```

Correct:
```go
policy := bluemonday.NewPolicy()
policy.AllowStandardURLs()
policy.AllowAttrs("href").OnElements("a")
policy.SkipElementsContent("script", "style", "iframe")
```

---

### Scenario: Content Item Obsidian Markdown Export

#### 1. Scope / Trigger
- Trigger: adding or changing the single-content-item Obsidian Markdown export endpoint, response payload, Markdown formatter, summary-state mapping, or export aggregate repository read.
- Applies to `api/v1/content_item.go`, `internal/router/content_item.go`, `internal/handler/content_item.go`, `internal/service/content_item.go`, `internal/repository/content_item.go`, generated Swagger docs, and export tests.

#### 2. Signatures
- Route: `GET /api/v1/content-items/:id/obsidian-export` in the strict authenticated content item route group.
- Service method:
  ```go
  ExportObsidianMarkdown(ctx context.Context, id uint) (*v1.ExportContentItemObsidianResponseData, error)
  ```
- Repository method:
  ```go
  GetExportDataByID(ctx context.Context, id uint) (*ContentItemExportData, error)
  ```
- Response payload:
  ```go
  type ExportContentItemObsidianResponseData struct {
      ContentItemId uint   `json:"contentItemId"`
      Filename      string `json:"filename"`
      Markdown      string `json:"markdown"`
  }
  ```

#### 3. Contracts
- The endpoint returns the existing JSON envelope, not `text/markdown`, so the frontend can support both copy and download from one response.
- Export is a pure projection. It must not call AI config repositories, AI services, chat completion, summary generation, or retry logic.
- Markdown must use ordinary Markdown sections, not Obsidian/YAML frontmatter.
- Metadata must include title, link, subscription feed name, published time, content type, deterministic tag names, and subscription feed folder name or an explicit empty marker.
- The current schema has no item-level original URL; use `feeds.feed_url` for link metadata unless a persisted item-level link is added by a later schema change.
- The available-text excerpt is rune-limited to the first 2000 Unicode characters. Append the truncation notice only when the source text was longer than 2000 runes.
- Suggested filenames are derived from the title, sanitized for Windows path-invalid characters, and fall back to `content-item-<id>.md` when the title cannot produce a usable filename.

#### 4. Validation & Error Matrix
- `id == 0` or malformed route id -> `v1.ErrBadRequest` and HTTP `400`.
- Missing content item -> `v1.ErrNotFound` / content-item not-found semantics and HTTP `404`.
- Unknown AI summary status -> `v1.ErrExportFailed` and HTTP `500`; do not silently produce misleading Markdown.
- Repository/database failure that is not a known domain error -> logged by the handler and HTTP `500`.
- AI summary `success` -> write current summary Markdown; if unexpectedly empty, write an explicit "not generated" line.
- AI summary `none` -> write the not-generated line.
- AI summary `pending` -> write the in-progress line.
- AI summary `failed` -> write the failed line and append the current summary error when present.
- AI summary `insufficient_text` -> write the insufficient-text line.

#### 5. Good/Base/Bad Cases
- Good: handler returns `{code,message,data:{contentItemId,filename,markdown}}`, service formats deterministic Markdown, repository loads feed/folder/tags over real SQLite, and tests prove all summary states plus truncation behavior.
- Base: an item without tags or folder still exports successfully with explicit empty markers and an empty tag slice from the repository.
- Bad: direct Markdown/file response that bypasses the envelope while the rest of the API is JSON-envelope based.
- Bad: export invokes summary generation or requires AI configuration before it can return Markdown.
- Bad: byte-based truncation that can split a multibyte character.

#### 6. Tests Required
- Repository integration test over migrated SQLite for `GetExportDataByID`: content fields, feed title/url, nullable folder name, sorted tag names, and missing-row not-found behavior.
- Service tests for success, none, pending, failed, insufficient-text, unknown status, deterministic metadata, sanitized filename, and rune-safe truncation with and without the truncation notice.
- Handler tests for the HTTP envelope, known error mapping, and at least one real service/repository path proving tags and folder metadata reach the response.
- Generated contract artifacts must be refreshed with `swag init -g cmd/server/main.go -o ./docs` after route or response annotation changes.
- Final validation must include `go test ./...`, `go build ./...`, `go vet ./...`, and `git diff --check`.

#### 7. Wrong vs Correct

Wrong:
```go
func (s *contentItemService) ExportObsidianMarkdown(ctx context.Context, id uint) (*v1.ExportContentItemObsidianResponseData, error) {
    summary, err := s.aiService.GenerateSummary(ctx, id) // export must not generate
    if err != nil {
        return nil, err
    }
    return markdownFromSummary(summary), nil
}
```

Correct:
```go
func (s *contentItemService) ExportObsidianMarkdown(ctx context.Context, id uint) (*v1.ExportContentItemObsidianResponseData, error) {
    data, err := s.contentItemRepo.GetExportDataByID(ctx, id)
    if err != nil {
        return nil, err
    }
    return formatObsidianExport(data)
}
```

Wrong:
```go
excerpt := text[:2000] // byte count can split UTF-8
```

Correct:
```go
runes := []rune(text)
if len(runes) > 2000 {
    excerpt := string(runes[:2000])
    // append truncation notice
}
```

---


## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
