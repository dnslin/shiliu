# Repositories

## Purpose

Repositories are the SQLite persistence boundary. They use GORM but expose project-specific interfaces and constructors.

## SQLite Contract

- Runtime storage is SQLite only.
- `repository.NewDB` opens `github.com/glebarez/sqlite`.
- Empty driver means SQLite; any non-empty non-`sqlite` driver must fail fast.
- Use `TranslateError: true` so unique conflicts surface as `gorm.ErrDuplicatedKey`.
- Preserve foreign-key enforcement in DSNs that write related rows.

## Read Patterns

- Optional lookup methods may return `nil, nil` for absent rows when callers expect that contract.
- Required lookup methods should preserve `gorm.ErrRecordNotFound` or map to `v1.ErrNotFound`.
- Keep list filters explicit and backed by indexes when they are user-facing.
- Prefer repository methods that express domain lookups over open-ended query builders.

## Write Patterns

- Use scoped `Updates` for update-only methods.
- Reject zero primary keys before touching the DB.
- Return `v1.ErrNotFound` when an update affects no rows.
- Let unique indexes be the concurrency guard.
- Keep create paths race-safe by mapping database conflicts, not only pre-checking existence.

## Testing

- Repository behavior tests use real temporary SQLite databases.
- Apply checked-in migrations through `cmd/migration` when testing shipped schema.
- Assert public repository behavior, not generated SQL strings.
- Test conflict, missing-row, and update-only behavior at the repository seam.

## Anti-Patterns

- Do not use `Save` for update-only contracts.
- Do not use `AutoMigrate` in production schema management.
- Do not use sqlmock to assert SQLite behavior.
- Do not import removed scaffold datastore drivers.
- Do not hide SQLite constraint failures behind generic errors that handlers cannot map.
