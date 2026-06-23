# Implementation Plan: golang-migrate migration mechanism

## Preconditions

- [x] Branch created: `issue-5-golang-migrate-migrations`.
- [x] Trellis task created: `.trellis/tasks/06-23-golang-migrate-migration-mechanism`.
- [x] User resolved the public `cmd/migration` direction contract: expose `up` and `down`, default `up`.
- [x] Planning artifacts reviewed/approved.
- [x] Run `python ./.trellis/scripts/task.py start 06-23-golang-migrate-migration-mechanism` before editing implementation files.
- [x] Run `trellis-before-dev` before implementation to refresh relevant specs.

## Public Interface Decision

Recommended CLI contract:

```bash
go run ./cmd/migration -conf config/local.yml -direction up
go run ./cmd/migration -conf config/local.yml -direction down
```

Defaults:

- `-direction up`
- `-path migrations`

Reason: deployment needs up by default, while acceptance and future rollback need an explicit down path. This is still small and avoids hidden test-only rollback behavior.

## Requirement-Driven Test Scenarios

TDD must proceed as vertical slices. Do not write all tests first.

1. **Tracer bullet: up applies versioned SQL**
   - Given a temporary SQLite file and the checked-in migration source, running the public migration seam with direction `up` succeeds.
   - Verify through public/observable database state: migration version or baseline marker is present.

2. **Idempotent up**
   - Given the same SQLite file after scenario 1, running `up` again succeeds and treats `migrate.ErrNoChange` as no-op success.

3. **Down after up**
   - Given the same SQLite file after up, running direction `down` succeeds and removes the baseline marker/effect or returns migration version to nil clean state.

4. **Invalid source errors**
   - Given an invalid migration source path, running migration returns an error instead of silently succeeding.

5. **No implicit startup migration**
   - Static validation confirms `cmd/server` and `cmd/task` do not import/call the migration runner and no production code calls `AutoMigrate`.

## Vertical Slice Checklist

### Slice 1: Up migration tracer bullet

- [x] Add one failing integration test for applying `up` to a temp SQLite file.
- [x] Add `golang-migrate/migrate/v4` imports and minimal runner seam.
- [x] Add `migrations/000001_<baseline>.up.sql` and matching down file only as needed to pass the tracer bullet.
- [x] Run the focused test and make it pass.

### Slice 2: Idempotent no-change behavior

- [x] Add one failing test for running `up` twice.
- [x] Treat `migrate.ErrNoChange` as success.
- [x] Run the focused test and make it pass.

### Slice 3: Down migration behavior

- [x] Add one failing test for `up` then `down`.
- [x] Implement direction handling in the runner and command seam.
- [x] Run the focused test and make it pass.

### Slice 4: Failure propagation

- [x] Add one failing test for invalid source path.
- [x] Ensure runner returns the underlying migration creation/run error.
- [x] Run the focused test and make it pass.

### Slice 5: Command and documentation integration

- [x] Rewrite `cmd/migration/main.go` from long-running `app.App` style to one-shot migration command if approved.
- [x] Remove `AutoMigrate` and stale `internal/model` dependency from `internal/server/migration.go`; delete or repurpose `MigrateServer` according to final seam.
- [x] Remove obsolete `cmd/migration/wire` usage if the direct command seam makes it unnecessary, or regenerate Wire if it remains.
- [x] Document migration directory, filename convention, and commands in README/README_zh or deployment-facing docs.

### Slice 6: Module hygiene and full validation

- [x] Run `go mod tidy`.
- [x] Run static searches:
  - `AutoMigrate`
  - `NewMigrateServer`
  - migration runner imports from `cmd/server` / `cmd/task`
- [x] Run `go test ./...`.
- [x] Run `go build ./...`.
- [x] Run `go vet ./...`.
- [x] Capture any failure, retry once if transient, and document fallback if needed.

## Risky Files / Rollback Points

- `cmd/migration/main.go`: command contract and process exit behavior.
- `internal/server/migration.go`: removal of `AutoMigrate` path.
- `cmd/migration/wire/*`: may become obsolete if migration command no longer builds an app; delete only after confirming no code imports it.
- `migrations/*.sql`: irreversible mistakes affect SQLite state; keep baseline non-business and test up/down on temp files first.
- `go.mod` / `go.sum`: tidy may add multiple migrate driver transitive dependencies; review for unexpected non-SQLite database drivers.

## Validation Commands

```bash
go test ./...
go build ./...
go vet ./...
go mod tidy
```

Focused commands will be chosen after the test file path is created.

## Completion Gate Before Finish Phase

- [x] PRD acceptance criteria all satisfied or explicitly marked out of scope with reason.
- [x] TDD cycle evidence recorded in final response.
- [x] Task specs updated if a reusable migration convention is learned.
- [x] Changes committed only during Trellis Finish phase after verification.
