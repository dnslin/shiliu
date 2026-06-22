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

(To be filled by the team)

---

## Naming Conventions

(To be filled by the team)

---

## Common Mistakes

- Keeping scaffold-only datastore imports in tests can prevent `go mod tidy` from removing drivers even after runtime code is cleaned up.
- Using `db.Debug()` for local convenience changes runtime logging behavior globally; keep SQL trace verbosity behind `data.db.user.debug`.
