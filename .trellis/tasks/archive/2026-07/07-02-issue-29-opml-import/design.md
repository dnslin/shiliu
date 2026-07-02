# Issue 29 OPML batch import design

## Architecture

This feature extends the existing feed workflow instead of creating a separate import pipeline.

Data flow:

`HTTP request -> FeedHandler.ImportOPML -> FeedService.ImportOPML -> parse OPML URLs -> FeedService.CreateFeed -> repository/content persistence -> count response`

The existing `CreateFeed` method remains the single owner for feed URL normalization, repository duplicate checks, fetch/parse, feed creation, and first-fetch content item persistence.

## API Contract

Route:

- `POST /api/v1/feeds/import-opml`
- Protected by the existing strict JWT feed route group.

Request forms:

- JSON pasted OPML:
  - `Content-Type: application/json`
  - body: `{ "opml": "<opml xml text>" }`
- Multipart upload:
  - `Content-Type: multipart/form-data`
  - form file field: `file`
  - optional text field fallback: `opml`

Response:

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "total": 4,
    "success": 1,
    "duplicate": 2,
    "failed": 1
  }
}
```

Known errors:

- Invalid or missing OPML input maps to `ErrOPMLInvalid` and HTTP 400.
- Unexpected import-level faults map to `ErrOPMLImportFailed` or the existing internal error handling.
- Per-feed fetch, parse, and duplicate outcomes are counted in the batch response, not returned as HTTP failures for the whole batch.

## Service Contract

Add to `FeedService`:

```go
ImportOPML(ctx context.Context, req *v1.ImportOPMLRequest) (*v1.ImportOPMLResponseData, error)
```

Service responsibilities:

- Validate non-empty OPML input.
- Parse OPML XML into candidate feed URLs.
- Normalize each candidate using existing `NormalizeFeedURL`.
- Count empty/invalid URL candidates as failures.
- Count duplicate normalized URLs within the same payload as duplicates.
- Call existing `CreateFeed` for distinct normalized URLs.
- Count `ErrFeedAlreadyExists` as duplicate.
- Count feed URL, fetch, or parse failures as failed.
- Propagate request cancellation/deadline errors to stop import.
- Avoid logging or returning raw OPML content.

## OPML Parsing

Use `encoding/xml.Decoder` and stream through XML tokens.

For each `outline` start element, inspect attributes:

- `xmlUrl`
- `xmlURL`
- `url`

The parser does not inspect `text`, `title`, folder names, nesting depth, or grouping metadata. It only returns discovered feed URL strings.

An OPML document with no feed URL candidates is invalid for this feature.

## Reuse And Boundaries

- Do not copy fetch, parse, sanitization, dedupe, or persistence logic into OPML code.
- Do not bypass `CreateFeed` with direct repository writes.
- Keep handler thin; request-form branching stays at HTTP boundary.
- Keep OPML parsing in service-level code where tests can exercise behavior without HTTP.
- Do not add a repository method unless `CreateFeed` cannot express a needed behavior.

## Compatibility

- Existing `POST /feeds`, refresh, list, and delete behavior must remain unchanged.
- Existing `CreateFeed` duplicate and first-fetch semantics remain the source of truth.
- Swagger artifacts must be regenerated after adding handler annotations.

## Tradeoffs

- Reusing `CreateFeed` means import performs one feed at a time. This keeps MVP behavior simple and consistent with manual feed creation.
- Duplicate URLs inside a single OPML payload are counted as duplicates after normalization. This gives the user honest batch statistics without extra persistence work.
- The response omits per-item detail to keep the MVP API compact. Detailed diagnostics can be added later without changing the count contract.
