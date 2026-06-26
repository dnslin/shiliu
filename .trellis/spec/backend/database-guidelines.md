# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

Shiliu MVP is a single-instance backend with a SQLite-only data boundary. The repository layer may use GORM, but it must not preserve Nunu scaffold assumptions for MySQL, PostgreSQL, Redis, or MongoDB.

---

## Scenario: SQLite-only repository boundary

### 1. Scope / Trigger
- Trigger: editing `internal/repository/repository.go`, `config/*.yml`, repository tests, `go.mod`, or any Wire provider that constructs storage dependencies.
- Applies to the backend module root, repository tests under `test/server/repository`, and Wire entrypoints under `cmd/server/wire`, `cmd/task/wire`, and `cmd/migration/wire`.

### 2. Signatures
- Runtime constructor: `func NewDB(conf *viper.Viper, l *log.Logger) *gorm.DB`.
- Required SQLite driver import: `github.com/glebarez/sqlite`.
- Config keys:
  - `data.db.user.driver: sqlite` — optional guard key; non-empty values other than `sqlite` are unsupported.
  - `data.db.user.dsn: <sqlite dsn>` — required by runtime startup.
  - `data.db.user.debug: false` — optional bool, defaults to false when absent.
- Repository test seam: use `repository.NewDB`, `repository.NewRepository`, and concrete repository interfaces over a temporary SQLite database.

### 3. Contracts
- `NewDB` opens only `sqlite.Open(dsn)` through GORM.
- Redis, MongoDB, MySQL, and PostgreSQL constructors, provider entries, branches, imports, and test dependencies are forbidden in live backend/runtime/test code.
- `db.Debug()` must never be called unconditionally. It is allowed only when `conf.GetBool("data.db.user.debug")` is true.
- The repository/transaction seam remains stable: `NewRepository`, `NewTransaction`, `Repository.DB(ctx)`, and `Transaction(ctx, fn)` continue to be the public access path for downstream services.
- Checked-in `config/local.yml` and `config/prod.yml` must keep SQLite DSN and `security.jwt.key`, and must not include Redis/Mongo sections or MySQL/PostgreSQL examples.

### 4. Validation & Error Matrix
- `data.db.user.driver == ""` -> treat as SQLite, so older minimal configs still work if a DSN is present.
- `data.db.user.driver == "sqlite"` -> open SQLite.
- `data.db.user.driver` is any other non-empty value -> panic at startup with an actionable message that says only SQLite is supported.
- `data.db.user.debug` absent or false -> no GORM info trace logs for normal queries.
- `data.db.user.debug == true` -> GORM debug/info SQL tracing is enabled.
- Removed datastore import appears in live code/test/module files -> fail the slice; remove the import or the stale dependency.

### 5. Good/Base/Bad Cases
- Good: `NewDB` imports only `glebarez/sqlite` for the storage driver, checked-in configs set `driver: sqlite`, `dsn: ...`, `debug: false`, repository tests use temporary SQLite, and `go mod tidy` removes external datastore drivers.
- Base: a config omits `driver` but supplies SQLite `dsn`; `NewDB` still opens SQLite and keeps debug off unless `debug: true` is present.
- Bad: repository tests use `gorm.io/driver/mysql` or `go-sqlmock` to assert SQL strings; this retains the wrong driver and tests an implementation shape instead of SQLite behavior.
- Bad: local config keeps Redis/Mongo sections or commented MySQL/PostgreSQL DSNs after the runtime has removed those constructors.

### 6. Tests Required
- Startup assertion: `NewDB` opens a configured temporary SQLite database and `SELECT 1` succeeds.
- Unsupported-driver assertion: `driver=mysql` and/or `driver=postgres` panics with an "only sqlite is supported" message.
- Debug assertion: with `debug` absent/false, a normal query does not emit a GORM `trace` info log; with `debug=true`, it does.
- Repository behavior assertion: concrete repository methods work through public seams over real SQLite after schema setup (`AutoMigrate` is acceptable in tests until versioned migrations exist).
- Dependency assertion: static search over `internal`, `cmd`, `config`, `test`, `go.mod`, and `go.sum` finds no removed datastore references.
- Validation assertion: `nunu wire all`, `go test ./...`, `go build ./...`, and `go vet ./...` pass.

### 7. Wrong vs Correct

Wrong:
```go
switch driver {
case "mysql":
    db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
case "postgres":
    db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
case "sqlite":
    db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
}
db = db.Debug()
```

Correct:
```go
if driver != "" && driver != "sqlite" {
    panic(fmt.Sprintf("unsupported db driver %q: only sqlite is supported", driver))
}
db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger})
if conf.GetBool("data.db.user.debug") {
    db = db.Debug()
}
```

Wrong:
```go
import "gorm.io/driver/mysql"
// Repository test asserts generated MySQL SQL strings with sqlmock.
```

Correct:
```go
db := repository.NewDB(sqliteTestConfig(t), logger)
require.NoError(t, db.AutoMigrate(&model.User{}))
userRepo := repository.NewUserRepository(repository.NewRepository(logger, db))
```

---

## Query Patterns

## Scenario: Repository error semantics — unique conflicts and update-only writes

### 1. Scope / Trigger
- Trigger: adding/changing repository write methods that must enforce uniqueness or must update an existing row without inserting; mapping SQLite/Gorm errors to `api/v1` errors.
- Applies to `internal/repository/*.go`, `internal/repository/repository.go` (`NewDB` Gorm config), and the service callers that translate repository errors into account/API errors.

### 2. Signatures
- DB constructor must translate driver errors:
  ```go
  gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger, TranslateError: true})
  ```
- Unique-conflict identity error: `gorm.ErrDuplicatedKey` (translated by `glebarez/sqlite` from `SQLITE_CONSTRAINT_UNIQUE`/`PRIMARYKEY`).
- Update-only method shape:
  ```go
  func (r *userRepository) Update(ctx context.Context, user *model.User) error
  ```

### 3. Contracts
- `TranslateError: true` is required so a SQLite `UNIQUE`/`PRIMARY KEY` violation surfaces as `gorm.ErrDuplicatedKey` instead of a raw driver string. Callers match with `errors.Is(err, gorm.ErrDuplicatedKey)`, never by parsing message text.
- `TranslateError` does not change `First()` not-found behavior: `gorm.ErrRecordNotFound` is still returned for empty reads, so existing `errors.Is(err, gorm.ErrRecordNotFound)` branches (e.g. `GetByID` → `v1.ErrNotFound`, `GetByUsername` → `(nil, nil)`) remain correct.
- An `Update` method must be update-only. It must:
  - reject a zero primary key with `v1.ErrBadRequest` before touching the DB;
  - scope the write with `Where("id = ?", id).Updates(map[string]interface{}{...})`;
  - return `v1.ErrNotFound` when `RowsAffected == 0` (no row matched).
- `Updates` with a map still triggers Gorm's `UpdatedAt` auto-timestamp callback; an explicit `updated_at` entry is not required.
- A read-before-write existence check is a UX optimization, not an atomicity guarantee. The database unique index is the only correct concurrency guard; the create path must still map `gorm.ErrDuplicatedKey` to the domain conflict error.

### 4. Validation & Error Matrix
- `Create` hits a unique index, `TranslateError: true` → `gorm.ErrDuplicatedKey`; service maps to `v1.ErrUsernameAlreadyUse`.
- `Create` hits a unique index, `TranslateError` unset → raw `constraint failed: UNIQUE ...` string leaks; service cannot match it and returns a 500. This is the bug to avoid.
- `Update` with `Id == 0` → `v1.ErrBadRequest`, no SQL executed, no row inserted.
- `Update` with a non-existent non-zero `Id` → `v1.ErrNotFound` (`RowsAffected == 0`), no row inserted.
- `Save(user)` with a zero PK → INSERT (wrong for an update-only contract). Do not use `Save` to express "update".

### 5. Good/Base/Bad Cases
- Good: `NewDB` sets `TranslateError: true`; `Create` returns `gorm.ErrDuplicatedKey` on conflict; `Update` uses scoped `Updates` and checks `RowsAffected`.
- Base: a single-write service still wraps `Create` so both the pre-check path and the race path converge on `v1.ErrUsernameAlreadyUse`.
- Bad: `Update` implemented as `r.DB(ctx).Save(user)` — silently inserts when `Id == 0` and "succeeds" for missing ids.
- Bad: detecting duplicates with `strings.Contains(err.Error(), "UNIQUE")`.

### 6. Tests Required
- Repository: duplicate insert asserts `errors.Is(err, gorm.ErrDuplicatedKey)` over real SQLite.
- Repository: `Update` with `Id == 0` returns an error and a follow-up fetch proves no row was created.
- Repository: `Update` with a missing non-zero `Id` returns `v1.ErrNotFound` and creates no row.
- Repository: `Update` of an existing row still persists changed fields.
- Service: create-time `gorm.ErrDuplicatedKey` maps to `v1.ErrUsernameAlreadyUse` (transaction mock executes the callback).

### 7. Wrong vs Correct

Wrong:
```go
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
    return r.DB(ctx).Save(user).Error // inserts when Id == 0
}
```

Correct:
```go
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
    if user.Id == 0 {
        return v1.ErrBadRequest
    }
    result := r.DB(ctx).Model(&model.User{}).
        Where("id = ?", user.Id).
        Updates(map[string]interface{}{
            "password_hash":      user.PasswordHash,
            "failed_login_count": user.FailedLoginCount,
            "locked_until":       user.LockedUntil,
        })
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected == 0 {
        return v1.ErrNotFound
    }
    return nil
}
```

---

## Migrations

## Scenario: SQLite versioned migrations with golang-migrate

### 1. Scope / Trigger
- Trigger: adding or changing database schema, migration files, `cmd/migration`, migration tests, or module dependencies for migration tooling.
- Applies to the backend module root, `cmd/migration`, `internal/migration`, `migrations/`, SQLite config under `config/*.yml`, and tests that verify schema migration behavior.
- Long-running entrypoints `cmd/server` and `cmd/task` are consumers of an already-migrated SQLite file; they must not run schema migrations implicitly.

### 2. Signatures
- Migration command:
  ```bash
  go run ./cmd/migration -conf config/local.yml -direction up
  go run ./cmd/migration -conf config/local.yml -direction down
  ```
- Command defaults:
  - `-conf config/local.yml`
  - `-direction up`
  - `-path migrations`; relative paths resolve from the command's current working directory before being converted to absolute `file://` source URLs.
- Migration dependency: `github.com/golang-migrate/migrate/v4` with file source and SQLite database drivers.
- Migration source directory: repository-root `migrations/`.
- File naming convention: paired SQL files with six-digit monotonic prefixes:
  ```text
  000001_description.up.sql
  000001_description.down.sql
  ```
- SQLite DSN source: `data.db.user.dsn`; open the migrate SQLite database instance from this DSN so migration and runtime DB access use the same SQLite filename/query semantics, including query parameters such as `_busy_timeout=5000` and URL-reserved path characters such as spaces and `#`. Quote YAML DSNs that contain `#` or other comment-sensitive characters so config parsing preserves the full value.
- SQLite driver guard: `data.db.user.driver` must be empty or `sqlite`; any other value fails before opening the migration source or database.

### 3. Contracts
- `cmd/migration` is a one-shot command that applies versioned migrations and exits; it does not build or run a Nunu `app.App` server loop.
- `cmd/migration` supports public `up` and `down` directions. `up` is the deployment default; `down` is the public rollback/testing direction.
- `migrate.ErrNoChange` is successful idempotent behavior and must return exit success.
- Real migration failures, invalid sources, unsupported directions, empty database DSNs, and dirty migration states must surface as errors.
- Production code must not call Gorm `AutoMigrate` for schema management.
- `cmd/server` and `cmd/task` must not import or call the migration runner.
- `cmd/migration/wire` is obsolete while `cmd/migration` is a direct one-shot command; do not recreate Wire plumbing unless the command boundary changes.
- Initial/baseline migrations must be reversible and must not create future business-domain tables prematurely. Later feature slices own their own schema migrations.

### 4. Validation & Error Matrix
- `-direction up` on a fresh SQLite file -> applies pending migrations and creates/updates `schema_migrations` plus migration SQL effects.
- `-direction up` when no migration remains -> success via `migrate.ErrNoChange`.
- `-direction down` after `up` -> rolls back exactly one latest migration boundary and leaves earlier applied versions in place.
- `-direction <other>` -> error saying only `up` or `down` is supported.
- Missing or invalid `-path` / source URL -> error; do not silently succeed.
- Empty `data.db.user.dsn` -> error before opening the migrator.
- Dirty database version -> error from `golang-migrate`; do not force or repair automatically in application code.
- `AutoMigrate` found in production migration/server/task code -> fail the slice; remove it or confine test-only schema setup to tests.

### 5. Good/Base/Bad Cases
- Good: `migrations/000002_users.up.sql` and `migrations/000002_users.down.sql` are added together, `cmd/migration -direction up` applies them on a temp SQLite file, `-direction down` rolls them back, and server/task startup code is unchanged.
- Base: a no-op rerun of `cmd/migration -direction up` logs/returns already-current success and exits with status 0.
- Bad: adding a Gorm model and relying on `AutoMigrate` from `cmd/server`, `cmd/task`, or `internal/server/migration.go` to create tables.
- Bad: adding only an `.up.sql` file without the matching `.down.sql` file.
- Bad: adding custom unversioned SQL execution loops instead of using `golang-migrate`'s version and dirty-state handling.

### 6. Tests Required
- Integration test with a temporary SQLite file proving `up` applies the checked-in migration source and leaves an observable schema/version effect.
- Integration test proving `up` is idempotent when no changes remain.
- Integration test proving `down` after `up` reverses the migration effect.
- Error test proving an invalid/missing migration source returns an error.
- Static or regression assertion proving production migration files no longer call `AutoMigrate`.
- Static searches before completion:
  - production `AutoMigrate`
  - `cmd/server` or `cmd/task` imports/calls to the migration runner
  - stale `cmd/migration/wire` imports
- Full validation: `go mod tidy`, `go test ./...`, `go build ./...`, `go vet ./...`.

### 7. Wrong vs Correct

Wrong:
```go
func (m *MigrateServer) Start(ctx context.Context) error {
    return m.db.AutoMigrate(&model.User{})
}
```

Correct:
```go
err := migration.Run(context.Background(), migration.Config{
    DatabaseDSN: conf.GetString("data.db.user.dsn"),
    SourceURL:   migration.FileSourceURL("migrations"),
    Direction:   migration.DirectionUp,
}, logger)
```

Wrong:
```text
migrations/000002_users.up.sql
# missing migrations/000002_users.down.sql
```

Correct:
```text
migrations/000002_users.up.sql
migrations/000002_users.down.sql
```

---

## Scenario: Migration-backed repository tests without duplicate SQLite driver registration

### 1. Scope / Trigger
- Trigger: repository tests need to verify schema created by checked-in `golang-migrate` SQL files while also constructing repositories through `repository.NewDB` / Gorm.
- Applies to `test/server/repository`, helpers that open temporary SQLite files, and any test package that wants both migrated schema and concrete repository behavior.

### 2. Signatures
- Migration command seam for repository tests:
  ```bash
  go run ./cmd/migration -conf <temp-yml> -direction up -path migrations
  go run ./cmd/migration -conf <temp-yml> -direction down -path migrations
  ```
- Temporary config must provide:
  ```yaml
  data:
    db:
      user:
        driver: sqlite
        dsn: "<temp-db-path>?_busy_timeout=5000"
        debug: false
  ```
- Repository construction after migration remains:
  ```go
  db := repository.NewDB(conf, logger)
  repo := repository.NewUserRepository(repository.NewRepository(logger, db))
  ```

### 3. Contracts
- Migration-backed repository tests must apply checked-in SQL migrations before opening the Gorm repository DB.
- A single Go test binary must not import both:
  - the Gorm SQLite path used by `repository.NewDB` (`github.com/glebarez/sqlite`), and
  - the golang-migrate SQLite database driver path used by `internal/migration.Run`.
- If a repository test already imports code that registers the Gorm SQLite driver, run migrations through the public `cmd/migration` subprocess instead of calling `internal/migration.Run` directly in the same test package.
- Repository tests may inspect SQLite metadata with `database/sql` after migration, but behavior assertions should go through repository public methods unless the test is specifically checking table existence / rollback boundaries.

### 4. Validation & Error Matrix
- Test imports both SQLite driver registration paths in one binary -> may panic with `sql: Register called twice for driver sqlite`; split migration into the `cmd/migration` subprocess or move the assertion to a different package/binary.
- Repository test calls `AutoMigrate` for a table with checked-in migrations -> fail; it no longer verifies the shipped schema.
- Repository test runs `cmd/migration` from the wrong working directory -> `-path migrations` may not resolve; set `cmd.Dir` to the repository root or pass an absolute migration path.
- Temporary YAML DSN omits quotes around special characters such as `#` -> config parsing may truncate the DSN; quote the DSN.

### 5. Good/Base/Bad Cases
- Good: repository test writes a temp config, executes `go run ./cmd/migration -conf <temp-yml> -direction up -path migrations` with `cmd.Dir` at repo root, then opens `repository.NewDB` and verifies repository behavior over real SQLite.
- Base: migration runner unit tests under `internal/migration` call `migration.Run` directly because they do not also construct the Gorm repository DB in the same test binary.
- Bad: `test/server/repository` imports `shiliu/internal/migration` and calls `migration.Run` directly while also importing `shiliu/internal/repository`, causing duplicate SQLite driver registration.
- Bad: repository tests recreate tables with `AutoMigrate(&model.User{})` and pass even when checked-in SQL migrations are missing or wrong.

### 6. Tests Required
- Migration-backed repository test helper applies `up` to a temp SQLite file through the public command seam before repository construction.
- Repository tests assert public behavior after migration: create, fetch, update, unique constraints, and not-found semantics.
- Rollback tests may use the command seam with `-direction down` and inspect `sqlite_master` to prove the latest migration boundary is removed.
- Focused validation includes `go test ./test/server/repository ./internal/migration` to cover both the command-backed repository tests and direct migration-runner tests.

### 7. Wrong vs Correct

Wrong:
```go
import "shiliu/internal/migration"

require.NoError(t, migration.Run(ctx, migration.Config{DatabaseDSN: dsn}, nil))
db := repository.NewDB(conf, logger) // same test binary can double-register sqlite
```

Correct:
```go
cmd := exec.Command("go", "run", "./cmd/migration", "-conf", tempConfig, "-direction", "up", "-path", "migrations")
cmd.Dir = repoRoot
require.NoError(t, cmd.Run())

db := repository.NewDB(conf, logger)
```

Wrong:
```go
require.NoError(t, db.AutoMigrate(&model.User{}))
```

Correct:
```go
runMigrations(t, conf.GetString("data.db.user.dsn"), "up")
repo := repository.NewUserRepository(repository.NewRepository(logger, repository.NewDB(conf, logger)))
```

## Scenario: Feed and content item persistence

### 1. Scope / Trigger
- Trigger: adding or changing subscription feed / content item schema, models, repositories, or repository tests.
- Applies to `migrations/`, `internal/model/feed.go`, `internal/model/content_item.go`, `internal/repository/feed.go`, `internal/repository/content_item.go`, and `test/server/repository`.

### 2. Signatures
- Feed table: `feeds(feed_url UNIQUE, type rss|podcast, fetch_status idle|fetching|success|failed, fetch_started_at, last_fetched_at, last_fetch_error, folder_id NULL)`.
- Content item table: `content_items(feed_id, dedupe_key, UNIQUE(feed_id,dedupe_key), type text|audio, title, description, content, show_notes, description_safe, content_safe, show_notes_safe, available_text, published_at, fetched_at, processing_status unprocessed|completed, marked_later bool, favorited bool, audio_progress_seconds)`.
- Repository constructors: `repository.NewFeedRepository(*Repository)` and `repository.NewContentItemRepository(*Repository)`.
- Runtime SQLite DSNs used by `repository.NewDB` must include `_pragma=foreign_keys(1)` so repository writes enforce `content_items.feed_id -> feeds.id`.

### 3. Contracts
- `feeds.feed_url` is the dedupe boundary for subscription feeds; duplicate inserts must surface as `gorm.ErrDuplicatedKey` through `TranslateError: true`.
- `content_items` dedupe is scoped per feed with `(feed_id, dedupe_key)`, so the same `dedupe_key` may exist under different feeds but not twice under one feed.
- Deleting a feed may cascade to its content items through the SQLite foreign key; repository tests must exercise foreign key enforcement rather than assuming it from SQL text.
- `processing_status`, `marked_later`, and `favorited` are persisted list-filter fields; migrations must add `CHECK` constraints/defaults plus indexes for each filterable field.
- `folder_id` is nullable and not a foreign key until a folders table exists.
- Repository reads should use public domain lookups (`GetByURL`, `GetByFeedAndDedupeKey`, `ListByFeedID`) and return `nil, nil` for missing optional lookups; direct SQL metadata reads are reserved for migration boundary tests.

### 4. Validation & Error Matrix
- Duplicate `feeds.feed_url` -> `gorm.ErrDuplicatedKey`.
- Duplicate `(feed_id, dedupe_key)` -> `gorm.ErrDuplicatedKey`.
- `content_items.feed_id` without a matching `feeds.id` -> foreign key error from SQLite/Gorm.
- `UpdateFetchState` with `feedID == 0` or empty status -> `v1.ErrBadRequest`; missing feed -> `v1.ErrNotFound`.
- `UpdateAudioProgress` with `itemID == 0` or negative seconds -> `v1.ErrBadRequest`; missing item -> `v1.ErrNotFound`.

### 5. Good/Base/Bad Cases
- Good: migration creates both tables, enum/boolean `CHECK` constraints, unique indexes, filter indexes for content item state/marks, FK cascade, and a down migration that drops state/mark columns before dropping `content_items` before `feeds`.
- Base: repository tests apply checked-in migrations through `cmd/migration`, open repositories with `repository.NewDB`, then assert unique conflicts and FK behavior over real SQLite.
- Bad: repository tests use `AutoMigrate`, omit `_pragma=foreign_keys(1)`, or assert only that SQL files contain `FOREIGN KEY` without proving the DB rejects orphan content items.

### 6. Tests Required
- Migration test: `up` creates `feeds` and `content_items`, expected unique indexes exist, duplicate feed URL fails, duplicate feed-scoped dedupe fails, and orphan content item fails with foreign keys enabled.
- Migration test: `down` after `up` rolls back only the latest feeds/content-items boundary and leaves earlier user/baseline migrations intact.
- Repository test: Feed create/read/update fetch diagnostics and duplicate URL behavior through `FeedRepository`.
- Repository test: Content item create/read/list/update audio progress, feed-scoped duplicate behavior, and foreign key rejection through `ContentItemRepository`.

### 7. Wrong vs Correct

Wrong:
```go
db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
// content_items orphan inserts may pass because SQLite foreign keys are off by default.
```

Correct:
```go
db, err := gorm.Open(sqlite.Open(sqliteDSNWithForeignKeys(dsn)), &gorm.Config{TranslateError: true})
```

Wrong:
```sql
UNIQUE (dedupe_key)
```

Correct:
```sql
CREATE UNIQUE INDEX idx_content_items_feed_dedupe_key ON content_items (feed_id, dedupe_key);
```

---


## Deployment

## Scenario: Docker Compose SQLite runtime roles

### 1. Scope / Trigger
- Trigger: editing `deploy/docker-compose/docker-compose.yml`, `deploy/build/Dockerfile`, `config/prod.yml`, deployment docs, or tests that claim to verify the self-hosted deployment path.
- Applies to the Compose deployment for the SQLite-only backend and the three runtime entrypoints: `cmd/migration`, `cmd/server`, and `cmd/task`.

### 2. Signatures
- Backend image: one image tag/build identity, currently `shiliu-backend:local`, containing:
  - `./bin/server` built from `./cmd/server`
  - `./bin/task` built from `./cmd/task`
  - `./bin/migration` built from `./cmd/migration`
- Compose services:
  - `migration` command: `./bin/migration -conf config/prod.yml -direction up -path migrations`
  - `server` command: `./bin/server -conf config/prod.yml`
  - `task` command: `./bin/task -conf config/prod.yml`
- Shared storage mount: `shiliu_storage:/data/app/storage` on `migration`, `server`, and `task`.
- Docker volume runtime name: `volumes.shiliu_storage.name: shiliu_storage` when docs or backup commands reference `shiliu_storage` directly.
- Production SQLite DSN: `data.db.user.dsn` points under the mounted `storage/` directory, e.g. `storage/shiliu.db?_busy_timeout=5000`.

### 3. Contracts
- `server`, `task`, and `migration` must use the same backend image. Service-specific behavior is selected by Compose `command`, not by building separate images.
- The runtime image must copy `bin/`, `config/`, and `migrations/` into `/data/app` so all three commands can run without Go tooling.
- `migration` is a one-shot job and must complete successfully before `server` and `task` start normally.
- `server` and `task` must not import or call the migration runner and must not run `AutoMigrate`.
- All three services must mount the same SQLite volume at the same container path so migration and long-running processes operate on the same database file.
- `task` does not publish an HTTP port; only `server` publishes the HTTP API port.
- Compose deployment must not include MySQL, PostgreSQL, Redis, or MongoDB services for the MVP SQLite-only path.
- Deployment docs must describe the SQLite volume/database backup boundary and state that TLS/HTTPS is provided by a reverse proxy or hosting platform.

### 4. Validation & Error Matrix
- Different image names/builds for `server` and `task` -> fail; rebuild as one multi-binary image.
- Image contains only one binary -> fail; Dockerfile must build `server`, `task`, and `migration`.
- `server` or `task` starts without `depends_on.migration.condition: service_completed_successfully` -> fail; startup order alone is not enough.
- A service lacks `shiliu_storage:/data/app/storage` -> fail; it would not share the SQLite file.
- Docs reference `shiliu_storage` but Compose omits `volumes.shiliu_storage.name` -> fail; Compose may create a project-prefixed volume and backup commands can target the wrong name.
- MySQL/Redis scaffold service appears in Compose or prod config -> fail; remove it.
- Local environment lacks Docker -> mark `docker compose config/build` unverified and rely on static/Go deployment contract tests; do not report Docker build as passed.

### 5. Good/Base/Bad Cases
- Good: one Compose service owns the build for `shiliu-backend:local`, all roles use that image, all roles mount `shiliu_storage:/data/app/storage`, `server`/`task` wait for successful `migration`, and docs back up the explicitly named `shiliu_storage` volume.
- Base: Docker is unavailable locally, but `go test ./test/deploy`, static Compose/config assertions, `go test ./...`, `go build ./...`, and `go vet ./...` pass; Docker build remains explicitly skipped/unverified.
- Bad: `server` builds `cmd/server`, `task` builds `cmd/task`, and `migration` builds `cmd/migration` as separate images; this violates the same-image deployment contract.
- Bad: README backup commands use `docker run -v shiliu_storage:/data ...` while Compose leaves the runtime volume name implicit.

### 6. Tests Required
- Parse `deploy/docker-compose/docker-compose.yml` and assert services contain `migration`, `server`, and `task`, and do not contain scaffold services such as `user-db` or `cache-redis`.
- Assert all three services use the same image; if only one service declares `build`, assert the others reuse the built image instead of declaring separate builds.
- Assert all three services mount `shiliu_storage:/data/app/storage` and that `volumes.shiliu_storage.name == "shiliu_storage"` when docs reference the runtime volume name.
- Assert `server` and `task` depend on `migration` with `condition: service_completed_successfully`.
- Assert commands invoke `./bin/migration`, `./bin/server`, and `./bin/task` with `config/prod.yml`; assert only `server` publishes HTTP.
- Parse `config/prod.yml` and assert the production SQLite DSN points under `storage/`, the fetch interval default exists, and AI placeholders do not hardcode a real secret.
- Inspect deployment docs for `Docker Compose`, `SQLite`, `volume`, `backup`, `TLS`, and `reverse proxy`/platform boundary language.
- Inspect Dockerfile for builds of `./cmd/server`, `./cmd/task`, and `./cmd/migration`, plus copies of `bin`, `config`, and `migrations` into the runtime image.

### 7. Wrong vs Correct

Wrong:
```yaml
services:
  server:
    build:
      args:
        APP_RELATIVE_PATH: ./cmd/server
  task:
    build:
      args:
        APP_RELATIVE_PATH: ./cmd/task
```

Correct:
```yaml
services:
  migration:
    image: shiliu-backend:local
    build:
      context: ../..
      dockerfile: deploy/build/Dockerfile
    command: ["./bin/migration", "-conf", "config/prod.yml", "-direction", "up", "-path", "migrations"]
    volumes:
      - shiliu_storage:/data/app/storage

  server:
    image: shiliu-backend:local
    command: ["./bin/server", "-conf", "config/prod.yml"]
    volumes:
      - shiliu_storage:/data/app/storage
    depends_on:
      migration:
        condition: service_completed_successfully

volumes:
  shiliu_storage:
    name: shiliu_storage
```

---

## Naming Conventions

(To be filled by the team)

---

## Common Mistakes

- Keeping scaffold-only datastore imports in tests can prevent `go mod tidy` from removing drivers even after runtime code is cleaned up.
- Using `db.Debug()` for local convenience changes runtime logging behavior globally; keep SQL trace verbosity behind `data.db.user.debug`.
- Referencing a Docker named volume in docs without pinning `volumes.<key>.name` can make backup commands target a non-existent or stale volume because Compose otherwise prefixes names with the project.
