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

(To be filled by the team)

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

## Naming Conventions

(To be filled by the team)

---

## Common Mistakes

- Keeping scaffold-only datastore imports in tests can prevent `go mod tidy` from removing drivers even after runtime code is cleaned up.
- Using `db.Debug()` for local convenience changes runtime logging behavior globally; keep SQL trace verbosity behind `data.db.user.debug`.
