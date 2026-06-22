# SQLite-only data layer cleanup

## Goal

Clean the Nunu scaffold data layer into the Shiliu MVP backend's SQLite-only runtime boundary. The application should no longer compile, configure, inject, or assume MySQL, PostgreSQL, Redis, or MongoDB for this slice.

User value: Shiliu is a single-instance subscription center with one local SQLite database. Removing unused external datastore assumptions makes local/VPS deployment simpler, reduces operational dependencies, and prevents later slices from accidentally building on scaffold-only infrastructure.

## Source Requirement

- GitHub issue: https://github.com/dnslin/shiliu/issues/3
- Parent PRD: https://github.com/dnslin/shiliu/issues/1
- Local source slice: `.trellis/tasks/06-17-shiliu-subscription-center/issues/slice-02.md`
- Domain context: `CONTEXT.md`

## Confirmed Facts from Repository Inspection

- `go.mod` already declares `module shiliu` and still directly requires Redis, MongoDB, MySQL, and PostgreSQL drivers.
- `internal/repository/repository.go` currently:
  - imports `github.com/glebarez/sqlite`, Redis, MongoDB, MySQL, and PostgreSQL drivers;
  - has a `NewDB` switch with `mysql`, `postgres`, and `sqlite` cases;
  - unconditionally calls `db.Debug()`;
  - still defines `NewRedis` and `NewMongo`.
- `config/local.yml` and `config/prod.yml` currently keep SQLite under `data.db.user` but also include Redis and MongoDB config sections plus commented MySQL/PostgreSQL examples.
- Wire injector files under `cmd/server/wire`, `cmd/task/wire`, and `cmd/migration/wire` only actively use `repository.NewDB`, but still contain stale commented `NewRedis`/`NewMongo` provider lines.
- Generated `wire_gen.go` files currently do not inject Redis or MongoDB, but the project spec requires regenerating Wire after provider-set changes.
- `test/server/repository/user_test.go` imports `gorm.io/driver/mysql`; leaving it unchanged would keep MySQL in `go.mod` after `go mod tidy`.
- Out-of-scope evidence: `deploy/docker-compose/docker-compose.yml` still references MySQL/Redis, but issue #3 only names repository, Wire, `go.mod`, and `config/local.yml` / `config/prod.yml`; Docker Compose is covered by later parent work.

## Requirements

1. `internal/repository/repository.go` must expose a SQLite-only `NewDB` based on `github.com/glebarez/sqlite`.
2. Remove runtime constructors and imports for Redis and MongoDB from the repository layer.
3. Remove MySQL and PostgreSQL branches and driver imports from the repository layer.
4. Keep the existing repository/transaction public seam intact for current services (`NewRepository`, `NewTransaction`, `Repository.DB`, and `Transaction`).
5. GORM debug logging must not be enabled unconditionally. Recommended decision for this slice: use an explicit `data.db.user.debug` boolean, defaulting to `false` in checked-in configs; operators can opt in locally without tying SQL verbosity to environment names.
6. Update Wire provider sets in all three entrypoints so deleted dependencies are not referenced, even as stale comments.
7. Regenerate generated Wire files through the project-approved command path.
8. Remove Redis, MongoDB, MySQL, and PostgreSQL driver dependencies from `go.mod` / `go.sum` by eliminating imports and running `go mod tidy`.
9. Clean `config/local.yml` and `config/prod.yml` so they no longer contain Redis or MongoDB config sections or MySQL/PostgreSQL examples; keep SQLite DSN and `security.jwt.key`.
10. Update tests that currently depend on MySQL test infrastructure so the suite compiles and verifies behavior against SQLite.
11. Do not introduce `golang-migrate`, production migrations, Docker Compose rewrite, or later subscription-center schema work in this slice.

## Acceptance Criteria

- [ ] `internal/repository/repository.go` only opens SQLite in `NewDB`; no `NewRedis`, `NewMongo`, MySQL branch, or PostgreSQL branch remains.
- [ ] GORM `Debug()` is not called unconditionally; SQL info logging is controlled by explicit config or equivalent environment/config behavior.
- [ ] `cmd/server/wire/wire.go`, `cmd/task/wire/wire.go`, and `cmd/migration/wire/wire.go` have provider sets aligned with the SQLite-only repository layer.
- [ ] `cmd/server/wire/wire_gen.go`, `cmd/task/wire/wire_gen.go`, and `cmd/migration/wire/wire_gen.go` are regenerated, not hand-edited.
- [ ] `go.mod` and `go.sum` no longer contain direct or tidy-retained dependencies for `github.com/redis/go-redis`, `go.mongodb.org/mongo-driver`, `gorm.io/driver/mysql`, or `gorm.io/driver/postgres`.
- [ ] `config/local.yml` and `config/prod.yml` contain SQLite DSN and JWT key, and do not contain Redis/Mongo sections or MySQL/PostgreSQL examples.
- [ ] Repository tests no longer depend on the MySQL GORM dialect.
- [ ] Requirement-driven tests cover:
  - SQLite `NewDB` opens a configured database successfully;
  - non-SQLite driver configuration is rejected if the driver key remains supported as an explicit guard;
  - SQL info tracing is disabled by default and enabled only when the debug config is true;
  - existing user repository CRUD behavior still works through a real SQLite-backed repository seam.
- [ ] `go test ./...` passes.
- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] Static searches show no runtime/config/test references to removed Redis/Mongo/MySQL/PostgreSQL drivers outside intentionally out-of-scope historical Trellis task docs and deploy files.

## Out of Scope

- Introducing `golang-migrate` or rewriting `cmd/migration` to versioned SQL migrations.
- Subscription source/content schema, FTS5 tables, feed fetching, AI summary, OPML import, Obsidian export, or auth model changes.
- Docker Compose rewrite and removal of deploy-level MySQL/Redis services.
- Changing API response envelopes or frontend behavior.
- Replacing the Nunu layer structure.

## Resolved Decision

GORM SQL debug uses explicit configuration: add `data.db.user.debug: false` to both checked-in configs and make `NewDB` call `db.Debug()` only when that boolean is true.

Trade-off accepted: this adds one DB-specific config key, but keeps SQL verbosity independent from broader environment or log-level settings and makes the default runtime quiet.
