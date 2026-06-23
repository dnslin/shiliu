# Design: golang-migrate migration mechanism

## First-Principles Reasoning

### Challenge assumptions

- Assumption: `AutoMigrate` is acceptable because the app only has one SQLite database. This is wrong for this task: `AutoMigrate` infers schema from current Go structs, has no explicit version history, and cannot express reversible SQL migration contracts for later slices.
- Assumption: migration can run inside `server` or `task` startup for convenience. This is unverified and risky: long-running services would gain hidden write side effects before serving or scheduling work, making deploy order and rollback ambiguous.
- Assumption: a custom migration loop over SQL files would be simpler. This is analogy from small projects, not a bedrock requirement; it would recreate version tracking, dirty-state handling, source ordering, and rollback semantics that `golang-migrate` already provides.
- Assumption: the initial migration can create the `users` table because a Gorm `User` model exists. This is potentially wrong: issue #5 explicitly reserves concrete business tables for later slices, especially the upcoming user model refactor.
- Assumption: down migration can be only a test helper. This is rejected for this slice: acceptance requires up/down verification, and future operational rollback needs a visible command contract.

### Bedrock truths

- A database schema change is durable state mutation; once applied to a SQLite file, the only safe way to reason about it is by a stable ordered version record and reversible change scripts.
- `server` and `task` are long-running processes; their core purpose is serving HTTP and scheduled work, not one-shot schema mutation.
- `cmd/migration` is already a distinct process boundary in the project and parent design says it must run before `server` / `task`.
- SQLite is the only supported database engine in this MVP; migration code only needs the SQLite database driver and file source driver.
- `golang-migrate` exposes the needed primitives directly: source URL + database URL, `Up`, `Down`, version/dirty state, and `ErrNoChange`.
- Future schema slices need a stable file convention that humans and tools can sort deterministically.

### Rebuild from truths

1. Since schema mutation is durable and ordered, migration files must be checked in as explicit paired SQL files under a deterministic directory.
2. Since only SQLite is supported, the migration database URL should be derived from the existing SQLite DSN and passed to the `golang-migrate` SQLite driver; no multi-database abstraction is needed.
3. Since `server` and `task` should not mutate schema implicitly, only `cmd/migration` should call the migration runner.
4. Since later slices own business schema, the first migration should be an operational baseline/placeholder that proves the mechanism without creating premature domain tables.
5. Since tests should verify public behavior, use temporary SQLite files and the migration runner/command seam rather than mocking the migrate library internals.
6. Since `ErrNoChange` means the database is already at the requested version, treat it as success while preserving real errors and dirty state.

### Contrast with convention

A conventional scaffold path would leave `AutoMigrate` in a startup-like server object and maybe add SQL files later. That optimizes for short-term convenience. The fundamental constraint here is a self-hosted SQLite file whose state must survive deployments; therefore explicit one-shot migration is the simpler long-term system even if it adds a small command/runner abstraction now.

### Conclusion

The correct design is a small explicit migration runner owned by `internal/server/migration.go` and invoked only by `cmd/migration`, backed by `golang-migrate` file-source + SQLite drivers, paired SQL files under `migrations/`, and integration tests against real temporary SQLite files. This intentionally replaces scaffold convenience with durable state control.

## Architecture and Boundaries

### Migration runner boundary

`internal/server/migration.go` remains the migration feature's production home, but its role changes from Gorm `AutoMigrate` server adapter to an explicit versioned migration runner.

Recommended shape:

- `type MigrationDirection string` with supported values `up` and `down`.
- `type MigrationConfig` carrying:
  - SQLite DSN or database URL derived from Viper config;
  - migration source URL, defaulting to `file://migrations`;
  - direction, defaulting to `up`.
- `RunMigrations(ctx, config, logger) error` or equivalent small seam that calls `migrate.New(sourceURL, databaseURL)` and then `Up` / `Down`.
- `ErrNoChange` is logged as already-current and returned as `nil`.
- `m.Close()` is deferred and close source/database errors are surfaced or logged without hiding the primary migration error.

The exact names can follow surrounding Go style during implementation; the important contract is a small public seam for `cmd/migration` and tests, not a large framework.

### Command boundary

`cmd/migration` should remain a one-shot command:

- Read `-conf` as today.
- Read migration-specific flags for source path and direction if approved.
- Construct logger/config.
- Run migrations.
- Exit `0` on success/no-change, non-zero via panic/logged fatal on real error.

`cmd/migration` does not need to build a long-running `app.App`, receive OS signals, or call `os.Exit` from a server's `Start` method. A direct one-shot command is easier to test and matches the process purpose.

### SQLite URL conversion

Current repository DSNs look like `storage/nunu-test.db?_busy_timeout=5000`. `golang-migrate` SQLite docs accept `sqlite://path/to/database?query`.

Design constraints:

- Preserve query parameters such as `_busy_timeout=5000` when converting DSN to URL.
- Reject empty DSN early with an actionable error.
- Keep the conversion SQLite-only; do not add driver switches.

A small helper can convert:

```text
storage/nunu-test.db?_busy_timeout=5000 -> sqlite://storage/nunu-test.db?_busy_timeout=5000
```

Absolute Windows paths must be handled carefully during implementation because this repository runs on Windows. Tests should use temporary file paths and assert the migration runner can open them.

### Migration file convention

Directory:

```text
migrations/
```

Naming:

```text
000001_<description>.up.sql
000001_<description>.down.sql
```

Initial migration intent:

- Non-business baseline only.
- Reversible.
- Safe to apply before user/feed/content migrations exist.
- Later slices append `000002_...`, `000003_...`, etc.

Candidate baseline: create and drop a small operational baseline marker table such as `shiliu_migration_baseline`. If implementation evidence shows an empty SQL migration is fully supported and cleaner, prefer the cleaner option only if tests prove up/down behavior and no-change semantics.

### Startup boundaries

- `cmd/server` keeps building HTTP/job servers only.
- `cmd/task` keeps building the scheduled task server only.
- Neither imports migration runner code nor runs migrations before opening the DB.

### Documentation boundary

Update README/README_zh or a deployment-facing note with:

- `migrations/` directory.
- paired filename convention.
- example run command for up.
- example run command for down if CLI exposes it.
- reminder that `cmd/migration` must run before `cmd/server` / `cmd/task` in deployment.

## Data Flow

```text
config/local.yml or config/prod.yml
  -> cmd/migration -conf <path> [-direction up|down] [-path migrations]
  -> viper config reads data.db.user.dsn
  -> build file:// migration source URL
  -> build sqlite:// database URL
  -> golang-migrate Up or Down
  -> schema_migrations version table + SQL file effects in SQLite file
```

`cmd/server` / `cmd/task` flow stays:

```text
config -> repository.NewDB -> service/task/http runtime
```

No migration call is inserted into these flows.

## TDD Test Strategy

Behavior tests should be integration-style and use real temporary SQLite files. Preferred test package can be `internal/server` or `cmd/migration` depending on the public seam chosen during implementation.

Planned behavior scenarios:

1. Applying up migrations to a temp SQLite file succeeds and leaves the baseline version/effect visible through the database.
2. Applying up a second time returns success as no-change.
3. Applying down after up succeeds and removes the baseline effect or returns the database to nil/clean version state.
4. Supplying an invalid migration source returns an error.
5. Static/compile-level guard: production migration code no longer calls `AutoMigrate`; this can be covered by targeted search in validation, not necessarily a Go unit test.

## Compatibility and Migration Notes

- Existing SQLite config keys remain unchanged.
- Existing local/prod DB files with no `schema_migrations` table will be initialized by golang-migrate on first migration run.
- This slice may leave old Gorm-created tables untouched if a developer already has a local DB; it does not attempt data cleanup.
- Future business-table migrations start after the baseline version.

## Operational and Rollback Considerations

- Rollback point 1: migration runner tests before deleting `AutoMigrate` path.
- Rollback point 2: initial SQL migration files before `go mod tidy`.
- Rollback point 3: command rewiring; if direct command shape breaks Wire-related builds, remove stale migration Wire use or regenerate only after tests explain the needed boundary.
- Dirty migration state must fail loudly; operators should inspect the SQLite file and migration file before forcing versions.
- Deployment wiring for running migration before long-running services is intentionally deferred to issue #6.
