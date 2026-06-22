# Implementation Plan: SQLite-only data layer cleanup

## Pre-start Checklist

- [ ] User approves planning artifacts and implementation start.
- [ ] Run `python ./.trellis/scripts/task.py start 06-22-sqlite-only-data-layer-cleanup` before editing implementation files.
- [ ] Create a feature branch from `main`, recommended name: `issue-3-sqlite-only-data-layer`.
- [ ] Run `python ./.trellis/scripts/task.py set-branch 06-22-sqlite-only-data-layer-cleanup issue-3-sqlite-only-data-layer` after branch creation.
- [ ] Load `trellis-before-dev` for backend conventions before code edits.

## Test Scenarios from Requirements

1. SQLite-only startup:
   - Given Viper config with `data.db.user.driver=sqlite` and a temp SQLite DSN, `repository.NewDB` returns a usable `*gorm.DB` and a trivial query succeeds.
2. Unsupported driver rejection:
   - Given Viper config with `data.db.user.driver=mysql`, `repository.NewDB` panics with an unsupported-driver message and does not require a MySQL driver import.
3. Debug off by default:
   - Given `data.db.user.debug=false` or unset, a trivial GORM query does not emit info-level SQL trace logs.
4. Debug opt-in:
   - Given `data.db.user.debug=true`, a trivial GORM query emits info-level SQL trace logs.
5. User repository behavior on SQLite:
   - Create, update, get-by-id, and get-by-email work through `repository.NewRepository` + `repository.NewUserRepository` over real SQLite after `AutoMigrate(&model.User{})`.
6. Dependency/config cleanup:
   - Static checks show removed drivers and Redis/Mongo config no longer appear in live source/config/test files covered by the slice.
7. Wire and module health:
   - `nunu wire all`, `go mod tidy`, `go test ./...`, `go build ./...`, and `go vet ./...` succeed.

## TDD Vertical Slices

### Slice 1: SQLite `NewDB` happy path

- [ ] Add/adjust a repository test that constructs an in-memory/temp SQLite config and calls `repository.NewDB`.
- [ ] Run the targeted test and see it fail if current code shape does not yet satisfy planned config/debug behavior.
- [ ] Simplify `NewDB` to open SQLite only while preserving pool setup and logger integration.
- [ ] Run targeted test to green.

### Slice 2: Unsupported driver guard

- [ ] Add one test proving non-`sqlite` driver config panics with an actionable error.
- [ ] Implement the guard without importing any non-SQLite driver.
- [ ] Run targeted test to green.

### Slice 3: GORM debug config behavior

- [ ] Add tests for debug disabled and enabled behavior using a captured zap observer/core or temporary log sink.
- [ ] Implement `if conf.GetBool("data.db.user.debug") { db = db.Debug() }`.
- [ ] Run targeted tests to green.

### Slice 4: User repository SQLite behavior

- [ ] Replace MySQL sqlmock-based user repository tests with real SQLite-backed tests.
- [ ] Use public repository seams only: `NewRepository`, `NewUserRepository`, `Create`, `Update`, `GetByID`, `GetByEmail`.
- [ ] Run repository tests to green.

### Slice 5: Remove deleted datastore runtime code

- [ ] Remove Redis/Mongo imports, fields/comments, constructors, and MySQL/PostgreSQL switch branches from `internal/repository/repository.go`.
- [ ] Remove stale commented provider lines from all three `wire.go` files.
- [ ] Run static searches for live references in `internal`, `cmd`, `config`, `test`, `go.mod`, and `go.sum`.

### Slice 6: Config and dependency cleanup

- [ ] Update `config/local.yml` and `config/prod.yml`: remove Redis/Mongo sections and MySQL/PostgreSQL examples; add/keep `data.db.user.debug: false`.
- [ ] Run `go mod tidy`.
- [ ] Verify removed driver modules are gone from `go.mod` and `go.sum` unless a transitive dependency still legitimately requires one. If a removed driver remains, trace the live import and fix it.

### Slice 7: Wire regeneration

- [ ] Run `nunu wire all`.
- [ ] If it fails transiently, capture error and retry once.
- [ ] If a single entrypoint still fails after retry, use `.trellis/spec/backend/quality-guidelines.md` fallback: `cd cmd/<entrypoint>/wire && go run -mod=mod github.com/google/wire/cmd/wire`, then rerun `nunu wire all`.
- [ ] Do not hand-edit `wire_gen.go`.

## Validation Commands

Run from repository root unless noted:

```bash
go test ./...
go build ./...
go vet ./...
```

Additional static checks:

```bash
# live source/config/test files should not keep removed runtime datastore assumptions
rg -n "github.com/redis/go-redis|go.mongodb.org/mongo-driver|gorm.io/driver/mysql|gorm.io/driver/postgres|NewRedis|NewMongo|data\.redis|data\.mongo|driver: mysql|driver: postgres" internal cmd config test go.mod go.sum

# expected out-of-scope deploy docs may still mention MySQL/Redis; do not count them for issue #3 acceptance
rg -n "redis|mongo|mysql|postgres" deploy .trellis/tasks/06-17-shiliu-subscription-center || true
```

## Risky Files and Rollback Points

- `internal/repository/repository.go`: startup path for all binaries. Roll back this file first if all entrypoints fail to boot.
- `test/server/repository/user_test.go`: current tests are implementation-coupled to MySQL SQL strings; rewrite carefully to keep user repository behavior coverage.
- `go.mod` / `go.sum`: accept only tidy changes caused by removed imports. If unrelated large churn appears, inspect before proceeding.
- `cmd/*/wire/wire_gen.go`: generated only. If output is unexpected, revert generated files, rerun Wire, and compare provider sets.
- `config/local.yml` / `config/prod.yml`: preserve `env`, `http`, `security.jwt.key`, and logging sections while removing only datastore leftovers.

## Completion Review Gate

Before reporting completion:

- [ ] PRD acceptance criteria checked off or explicitly noted if blocked.
- [ ] `git diff` reviewed for accidental deploy/parent-scope edits.
- [ ] Trellis check skill or equivalent quality review run after edits.
- [ ] If a new reusable convention is learned, update `.trellis/spec/backend/quality-guidelines.md` through `trellis-update-spec`.
- [ ] Prepare commit message, but commit only when requested by the user or during Trellis finish phase.
