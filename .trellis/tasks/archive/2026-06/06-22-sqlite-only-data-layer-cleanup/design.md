# Design: SQLite-only data layer cleanup

## First-Principles Reasoning

### Challenge assumptions

- Assumption: keeping driver switches is harmless because only SQLite is configured. This is unverified and potentially wrong: compile-time imports keep external database drivers in the module graph, and future code can accidentally rely on a scaffold-only backend.
- Assumption: local development should always run GORM `Debug()`. This is analogy from scaffold convenience, not a Shiliu requirement; unconditional SQL info logs increase noise and can expose query values.
- Assumption: repository tests can keep a MySQL sqlmock dialect while production is SQLite. This is wrong for this slice because the test import itself keeps the MySQL driver dependency alive.
- Assumption: commented Wire providers do not matter. They do not affect compilation, but they are stale interface documentation and can mislead future edits.
- Assumption: deploy-level MySQL/Redis cleanup belongs here. Issue #3 does not name deploy files; parent planning separates Docker Compose rewrite into later work.

### Bedrock truths

- Shiliu MVP's data boundary is one SQLite database file in a single-instance deployment.
- Go module dependencies are retained by live imports; `go mod tidy` cannot remove drivers while code or tests import them.
- Wire generated files are deterministic outputs of provider definitions and must be regenerated after provider-set edits.
- GORM `Debug()` is a runtime behavior change: it raises SQL logging to info for the returned DB session.
- A configuration key is an external contract for this repository's runtime; keeping it should mean the code has a clear single responsibility for it.

### Rebuild from truths

1. Since the product has exactly one database engine in this slice, `NewDB` should only instantiate `sqlite.Open(dsn)`.
2. Since no runtime path should use Redis, MongoDB, MySQL, or PostgreSQL, their constructors, branches, imports, and tidy-retained test imports must disappear.
3. Since SQL verbosity is optional behavior, the DB constructor should start from a non-debug logger and call `Debug()` only when explicit config asks for it.
4. Since generated Wire output must match provider sets, edit `wire.go` only, then regenerate `wire_gen.go` through `nunu wire all` with documented fallback only if needed.
5. Since tests should verify behavior through public seams, use `repository.NewDB` and `repository.NewRepository` with real temporary SQLite rather than testing SQL strings for another dialect.

### Contrast with convention

A conventional scaffold cleanup might leave multi-driver switches and commented providers because the configured default is SQLite. That optimizes for preserving scaffold flexibility. Shiliu's bedrock constraint is different: the MVP intentionally removes external datastore choices to simplify deployment and prevent later architectural drift.

### Conclusion

The simplest correct design is a SQLite-only repository boundary with an explicit opt-in SQL debug flag, cleaned config, regenerated Wire, and SQLite-backed tests. This conflicts with generic scaffold extensibility, but matches Shiliu's single-instance MVP constraints.

## Architecture and Boundaries

### Repository boundary

`internal/repository/repository.go` remains the owner of database construction and repository transaction context handling.

- Keep:
  - `Repository` with `db *gorm.DB` and `logger *log.Logger`.
  - `NewRepository(logger, db)`.
  - `NewTransaction` and `Repository.Transaction`.
  - `Repository.DB(ctx)` transaction-aware access.
- Change:
  - `NewDB(conf, logger)` opens only SQLite.
  - Remove Redis/Mongo constructor functions.
  - Remove MySQL/PostgreSQL driver imports and switch branches.
  - Keep or validate `data.db.user.driver` only as an explicit guard if the config key remains. A non-empty non-`sqlite` value should panic with an actionable message.
  - Use `zapgorm2.New(l.Logger)` as the GORM logger for SQLite so existing logging integration stays intact.

### Configuration boundary

Checked-in configs keep only the data needed for SQLite and auth:

```yaml
data:
  db:
    user:
      driver: sqlite
      dsn: storage/nunu-test.db?_busy_timeout=5000
      debug: false
security:
  jwt:
    key: ...
```

Design intent:

- `dsn` is required.
- `driver: sqlite` can remain as a declarative guard for existing config shape, but no alternative driver values are supported.
- `debug: false` makes SQL info tracing explicit and defaults to off in both local and prod.
- Redis/Mongo sections and commented MySQL/PostgreSQL examples are removed from `local.yml` and `prod.yml`.

### Wire boundary

Provider sets stay structurally the same but contain only active SQLite-compatible providers:

- `cmd/server/wire/wire.go`: `repository.NewDB`, `repository.NewRepository`, `repository.NewTransaction`, `repository.NewUserRepository`.
- `cmd/task/wire/wire.go`: same as server repository set.
- `cmd/migration/wire/wire.go`: `repository.NewDB`, `repository.NewRepository`, `repository.NewUserRepository`.

Remove stale commented deleted providers. Regenerate generated files; do not manually edit generated code except through Wire output.

### Dependency boundary

After repository/test imports change, `go mod tidy` should remove:

- `github.com/redis/go-redis/v9`
- `go.mongodb.org/mongo-driver`
- `gorm.io/driver/mysql`
- `gorm.io/driver/postgres`

Any transitive modules that disappear from `go.sum` are acceptable if tidy removes them because no live import needs them.

### Test boundary

Tests verify observable behavior:

- `repository.NewDB` can open a temporary SQLite DB from config and return a usable `*gorm.DB`.
- If `data.db.user.driver` remains in config, `NewDB` rejects non-`sqlite` values; this proves the single-engine contract without retaining driver imports.
- Default DB construction does not emit GORM info trace logs for a normal query.
- Setting `data.db.user.debug: true` emits GORM info trace logs for a normal query.
- Existing user repository CRUD tests should use real SQLite and `AutoMigrate(&model.User{})` instead of MySQL sqlmock expectations, so they keep testing repository behavior while removing the MySQL dependency.

## Data Flow

```text
config/local.yml or config/prod.yml
  -> pkg/config.NewConfig / viper
  -> repository.NewDB(conf, logger)
  -> sqlite.Open(dsn) + gorm.Open(...)
  -> repository.NewRepository(logger, db)
  -> service/task/migration code through existing repository seams
```

Error flow:

- Missing/invalid DSN or failed SQLite open: panic at startup, consistent with existing constructor behavior.
- Unsupported driver value if provided: panic at startup with an actionable message.
- Failed `db.DB()` extraction or pool configuration: panic at startup, consistent with existing behavior.

## Compatibility and Migration Notes

- This slice intentionally breaks compatibility with configs that set `data.db.user.driver` to `mysql` or `postgres`.
- Existing SQLite config remains compatible if it includes `driver: sqlite` and `dsn`.
- No database schema migration is introduced; existing `cmd/migration` behavior is unchanged.
- Runtime services still receive `*gorm.DB` through Wire, so downstream service/handler interfaces do not change.

## Trade-offs

- Explicit `debug` config key vs environment-derived debug:
  - Chosen: explicit `data.db.user.debug`, because it controls only database SQL tracing and defaults off.
  - Alternative rejected for this slice: enable debug when `env != prod` or `log.log_level == debug`; lower config churn but couples DB trace verbosity to broader runtime knobs.
- Keep `driver: sqlite` vs remove `driver` key:
  - Chosen: keep `driver: sqlite` as a guard during this cleanup because existing config already has it and tests can prove non-SQLite values are rejected. Later cleanup may remove it if the project wants an even narrower config contract.
  - Alternative rejected for this slice: remove `driver`; simpler config, but existing config overrides using the key silently stop being validated.
- Real SQLite tests vs sqlmock:
  - Chosen: real SQLite for repository seam, because this slice is specifically about the concrete persistence engine and dependency graph.

## Operational and Rollback Considerations

- Rollback point 1: repository/config/test edits before tidy. If behavior regresses, revert source changes before touching dependency graph.
- Rollback point 2: `go mod tidy` output. If unrelated dependencies churn unexpectedly, inspect live imports before accepting.
- Rollback point 3: Wire regeneration. If `nunu wire all` fails, capture error, retry once, then use documented per-entry fallback only after the retry fails.
- Manual deploy configs are intentionally not changed here; note remaining deploy MySQL/Redis references as later-slice work rather than a failed acceptance criterion for this slice.
