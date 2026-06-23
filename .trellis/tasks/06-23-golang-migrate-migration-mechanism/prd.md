# [切片04] golang-migrate 迁移机制接入

## Goal

把拾流后端的数据库迁移机制从裸 Gorm `AutoMigrate` 切换为 `golang-migrate/migrate` 的显式版本化迁移，让 SQLite schema 变化可以在 `server` / `task` 正常启动前独立执行、可追踪、可回滚。

User value: 拾流是自托管单实例产品，SQLite 文件是核心持久化资产。显式版本化迁移让后续用户账户、订阅源、内容、FTS5 等 schema 切片有确定顺序和回滚边界，避免长期运行服务在启动时偷偷改变数据库结构。

## Source Requirements

- GitHub issue: https://github.com/dnslin/shiliu/issues/5
- Parent PRD: https://github.com/dnslin/shiliu/issues/1
- Local source slice: `.trellis/tasks/06-17-shiliu-subscription-center/issues/slice-04.md`
- Parent design: `.trellis/tasks/06-17-shiliu-subscription-center/design.md`
- Blocker completed by archived task: `.trellis/tasks/archive/2026-06/06-22-sqlite-only-data-layer-cleanup/`

## Confirmed Facts from Repository Inspection

- `go.mod` currently has SQLite/Gorm dependencies but does not yet require `github.com/golang-migrate/migrate/v4`.
- `internal/repository/repository.go` is already SQLite-only after issue #3 and keeps `data.db.user.driver`, `dsn`, and `debug` config keys.
- `config/local.yml` and `config/prod.yml` both point `data.db.user.dsn` at `storage/nunu-test.db?_busy_timeout=5000`.
- `cmd/migration/main.go` currently builds a Nunu app through `cmd/migration/wire` and runs it like a server.
- `internal/server/migration.go` defines `MigrateServer.Start`, which calls `m.db.AutoMigrate(&model.User{})`, logs `AutoMigrate success`, and exits the process.
- `cmd/server` and `cmd/task` startup paths open the repository DB through Wire but do not call `AutoMigrate` or any migration runner today.
- Parent design requires `cmd/migration` to execute `golang-migrate/migrate`, use paired SQL files such as `000001_init.up.sql` / `000001_init.down.sql`, run before `server` / `task`, and avoid implicit migrations in long-running services.
- Context7 documentation for `/golang-migrate/migrate` confirms Go-library usage via `migrate.New(sourceURL, databaseURL)`, blank source/database driver imports, `Up()`, `Down()`, `ErrNoChange`, and SQLite URL forms such as `sqlite://path/to/database`.

## Requirements

1. Add `golang-migrate/migrate/v4` and the needed file-source / SQLite database driver imports to the codebase; run `go mod tidy` so the module graph is clean.
2. Establish a checked-in migration directory at repository root, named `migrations/`.
3. Use paired SQL migration filenames with six-digit monotonic prefixes and explicit directions: `000001_<description>.up.sql` and `000001_<description>.down.sql`.
4. Add an initial baseline migration that is intentionally non-business-schema and reversible. Later feature slices must create their own business tables through additional migrations.
5. Replace `internal/server/migration.go`'s Gorm `AutoMigrate` behavior with a versioned `golang-migrate` runner.
6. Keep `cmd/migration` as the explicit migration entrypoint that can be run before `server` / `task`.
7. Ensure `cmd/server` and `cmd/task` startup paths still do not trigger schema migration implicitly.
8. The migration runner must treat `migrate.ErrNoChange` as successful/idempotent behavior.
9. Migration failures and dirty database state must surface as errors instead of being swallowed.
10. Document the migration directory, filename convention, and the command shape in README/deployment-facing docs or concise code comments.
11. Use TDD vertical slices: add one behavior-level test, observe it fail, implement the smallest correct production change for that behavior, make it pass, then proceed to the next behavior. Do not write all tests first.
12. Preserve the SQLite-only boundary from issue #3; do not reintroduce MySQL, PostgreSQL, Redis, MongoDB, or multi-database abstractions.

## Acceptance Criteria

- [ ] `github.com/golang-migrate/migrate/v4` is present in `go.mod`, and `go mod tidy` leaves the module files clean.
- [ ] `migrations/` exists with paired files matching `000001_<description>.up.sql` and `000001_<description>.down.sql`.
- [ ] Initial migration is reversible and does not create future business-domain tables prematurely.
- [ ] `internal/server/migration.go` no longer imports `internal/model` for `AutoMigrate` and does not call `AutoMigrate`.
- [ ] `cmd/migration` executes versioned migrations through `golang-migrate/migrate`.
- [ ] Re-running an already-applied up migration exits successfully as no-op behavior.
- [ ] Running up and then down against a SQLite file database succeeds cleanly.
- [ ] `cmd/server` and `cmd/task` startup paths do not invoke migration logic.
- [ ] Migration directory and paired SQL naming convention are documented for future slices.
- [ ] Requirement-driven tests cover:
  - successful up migration against a temporary SQLite file;
  - idempotent up when no migration changes remain;
  - successful down after up;
  - error propagation for an invalid/missing migration source;
  - no `AutoMigrate` call remains in production migration code.
- [ ] `go test ./...` passes.
- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes.

## Out of Scope

- Creating `users`, feeds, content items, tags, folders, AI summary, or FTS5 business tables in this slice.
- Rewriting Docker Compose; issue #6 owns migration-as-preflight deployment wiring.
- Changing repository CRUD behavior, HTTP API behavior, authentication model, or frontend behavior.
- Introducing Goose, a custom unversioned migration runner, database multi-backend support, or runtime `server` / `task` auto-migration.
- Replacing Gorm as the repository access layer.

## Resolved Decision

`cmd/migration` must publicly support both directions:

```bash
go run ./cmd/migration -conf config/local.yml -direction up
go run ./cmd/migration -conf config/local.yml -direction down
```

Default direction is `up`. Trade-off accepted: the CLI exposes one additional operational flag, but rollback verification and future deployment operations use the same public command seam instead of hidden test-only behavior.
