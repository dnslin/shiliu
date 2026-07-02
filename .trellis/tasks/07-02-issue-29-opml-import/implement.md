# Issue 29 OPML batch import implementation plan

## TDD Strategy

Use vertical TDD slices. Each slice adds one observable behavior, gets it green, then moves to the next behavior. Tests should exercise public seams: `FeedService.ImportOPML` and `FeedHandler.ImportOPML`.

## Ordered Checklist

1. Service tracer: JSON-equivalent OPML all-success
   - Add one service test with a sample OPML containing one valid feed URL.
   - Use the existing fixture Fetcher and real SQLite harness.
   - Implement `ImportOPMLRequest`, `ImportOPMLResponseData`, `FeedService.ImportOPML`, and OPML URL extraction enough to pass.

2. Service mixed statistics
   - Add service coverage for one successful feed, one existing duplicate, one fetch/parse failure, and one duplicate URL inside the same OPML payload.
   - Implement full counting rules and cancellation-aware loop behavior.

3. OPML parser hierarchy behavior
   - Add service/parser coverage for nested folder outlines and folder metadata.
   - Ensure only feed URL attributes are read; folder names and nesting never create folders or assignments.

4. Handler JSON pasted OPML
   - Add httpexpect test for `POST /feeds/import-opml` with JSON body and count response.
   - Implement API DTOs, handler method, error mapping, and route registration.

5. Handler multipart uploaded OPML
   - Add httpexpect test for multipart `file` upload and matching count response.
   - Implement multipart branch and size-conscious request reading using Gin/request helpers.

6. Error behavior
   - Add handler/service coverage for empty or invalid OPML.
   - Map to `ErrOPMLInvalid` with HTTP 400.

7. Generated and integration updates
   - Regenerate mocks if the `FeedService` interface affects generated test mocks.
   - Regenerate Swagger artifacts with `swag init -g cmd/server/main.go -o ./docs`.
   - Run formatting and full validation.

## Expected Files

- `api/v1/feed.go`
- `api/v1/errors.go` if mapping needs adjustment
- `internal/service/feed.go`
- `internal/service/feed_opml.go` or equivalent focused service file
- `internal/handler/feed.go`
- `internal/router/feed.go`
- `test/server/handler/feed_test.go`
- `internal/service/feed_fetch_test.go` or a focused `feed_opml_test.go`
- `docs/docs.go`
- `docs/swagger.json`
- `docs/swagger.yaml`

## Validation Commands

- Focused service tests: `go test ./internal/service -run 'TestFeedServiceImportOPML'`
- Focused handler tests: `go test ./test/server/handler -run 'TestFeedHandler_ImportOPML'`
- Full tests: `go test ./...`
- Static checks: `go vet ./...`
- Build: `go build ./...`
- Whitespace check: `git diff --check`

## Review Gates

- Confirm no code path writes feeds directly from OPML without `CreateFeed`.
- Confirm OPML folder/group metadata is ignored.
- Confirm failed entries do not leave feed records.
- Confirm existing duplicate and in-payload duplicate stats are deterministic.
- Confirm handler tests assert the response envelope, not only status codes.

## Rollback Points

- If route contract needs to change, adjust only API DTOs, handler tests, and router wiring before service implementation.
- If `CreateFeed` behavior is insufficient, stop and revise design before adding repository-level import code.
