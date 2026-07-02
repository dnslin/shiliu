# Issue 29 OPML batch import

## Goal

Implement OPML import for Shiliu so a user can batch-create subscription feeds from an OPML document while preserving the existing feed creation contract: a feed is persisted only after fetch and parse succeeds.

## Source Of Truth

- GitHub issue: #29 `[切片28] OPML 批量导入`
- Parent PRD: `.trellis/tasks/06-17-shiliu-subscription-center/PRD-ISSUE.md`
- Domain glossary: `CONTEXT.md`
- Existing design: `.trellis/tasks/06-17-shiliu-subscription-center/design.md`
- Existing implementation plan: `.trellis/tasks/06-17-shiliu-subscription-center/implement.md`

## Confirmed Facts

- OPML import is an MVP feature and means one-time batch import, not OPML sync.
- OPML import only reads feed URLs and ignores original OPML folders or grouping hierarchy.
- Import must not create Shiliu folders and must not assign imported feeds to folders.
- Every OPML feed URL must reuse the existing feed creation path: normalize URL, detect duplicates, fetch, parse, create feed, and persist initial content items.
- Failed items count as failures and must not create empty subscription feeds.
- Existing feeds count as duplicates by normalized feed URL and must not be fetched or recreated.
- Newly created feeds must reuse the existing first-fetch limit of at most 50 persisted content items.
- The implementation spans handler, service, router, API DTOs, tests, and Swagger artifacts.

## Requirements

- Add an authenticated HTTP endpoint for OPML import under the feed routes.
- Support pasted OPML through JSON request bodies.
- Support uploaded OPML through multipart form-data.
- Parse OPML by walking `outline` elements and reading feed URL attributes only.
- Accept common OPML feed URL attribute names, prioritizing `xmlUrl`; `xmlURL` and `url` are accepted as compatibility fallbacks.
- Ignore all OPML folder/group hierarchy and outline display metadata.
- Deduplicate repeated URLs inside the same OPML payload by normalized feed URL before attempting creation; count repeated payload entries as duplicates.
- For each distinct normalized feed URL:
  - If a feed already exists, count it as duplicate.
  - If fetch or parse fails, count it as failed.
  - If creation succeeds, count it as success.
- Return total, success, duplicate, and failed counts.
- Keep per-item details out of the MVP response unless they are needed for debugging later.
- Do not introduce OPML sync, folder mapping, automatic folder creation, background retry, failure drafts, or partial per-item error reporting in this issue.

## Acceptance Criteria

- [ ] OPML parsing extracts feed URLs from nested outlines while ignoring folders and grouping hierarchy.
- [ ] JSON pasted OPML imports through the authenticated feed API.
- [ ] Multipart uploaded OPML imports through the same handler/service behavior.
- [ ] Each imported feed reuses existing feed creation behavior; successful entries persist feeds and initial content items.
- [ ] Failed fetch/parse entries are counted as failed and do not create feed records.
- [ ] Existing feeds and duplicate URLs in the same OPML payload are counted as duplicate and are not recreated.
- [ ] Response data includes `total`, `success`, `duplicate`, and `failed` counts.
- [ ] Service tests use Fetcher fixtures/sample OPML and cover all-success, partial failure, existing duplicate, duplicate within payload, and mixed statistics.
- [ ] Handler tests use httpexpect and assert JSON and multipart import result counts.
- [ ] Route registration is covered by existing production router patterns.
- [ ] `go build ./...` and `go test ./...` pass before completion.

## Out Of Scope

- OPML sync or scheduled OPML re-import.
- Mapping OPML folders to Shiliu folders.
- Creating folders from OPML folder names.
- Assigning imported feeds to folders.
- Persisting failed feed drafts or retry queues.
- Import progress jobs or async batch status APIs.
- Detailed per-feed result reporting in the response.
