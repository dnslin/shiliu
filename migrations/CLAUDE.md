# Migrations

## Purpose

Versioned SQLite schema changes live here and are applied by `cmd/migration`.

## File Rules

- Use paired SQL files with six-digit monotonic prefixes.
- Every `.up.sql` needs a matching `.down.sql`.
- Later feature slices own their own schema changes.
- Do not add future-domain tables before the feature requires them.

## SQLite Rules

- Keep schema portable to the configured SQLite runtime.
- Use constraints and indexes for user-facing filters and uniqueness rules.
- Preserve foreign-key behavior for dependent content rows.
- Quote or document DSNs with `#` and other YAML-sensitive characters in config docs.
- Rollbacks should remove the latest boundary without disturbing earlier versions.

## Runtime Contract

```bash
go run ./cmd/migration -conf config/local.yml -direction up
go run ./cmd/migration -conf config/local.yml -direction down
```

- `up` is deployment default.
- `down` rolls back one latest migration boundary.
- `migrate.ErrNoChange` is success.
- Dirty states must surface as errors; do not auto-repair in app code.

## Testing

- Prove `up` applies checked-in SQL.
- Prove rerunning `up` is idempotent.
- Prove `down` rolls back the latest boundary.
- Include repository tests over migrated schema when persistence behavior changes.
- Test invalid paths, unsupported directions, and empty DSNs near migration code.

## Anti-Patterns

- Do not rely on GORM `AutoMigrate` for production schema.
- Do not add only one side of a migration pair.
- Do not run migrations implicitly from `cmd/server` or `cmd/task`.
- Do not write custom unversioned SQL migration loops.
- Do not repair dirty migration state automatically in application startup.
