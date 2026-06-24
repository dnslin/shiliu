# Design: User model, users migration, and repository

## First-Principles Reasoning

### 1. Challenge assumptions

- Assumption: Keep `Email`, `Nickname`, and profile fields because the Nunu template already has them. This is unverified template residue; the parent PRD explicitly says the single account logs in with `username + password` and does not carry email/profile semantics.
- Assumption: Keep separate `UserId` plus numeric `Id` because the scaffold does. This duplicates identity without a product requirement. The slice asks for a single `id` field.
- Assumption: Repository tests can use `AutoMigrate` because they already run on SQLite. This is wrong for this slice: the acceptance target is the checked-in `golang-migrate` schema, not a schema inferred from the current Go struct.
- Assumption: SQL string assertions or sqlmock would be faster. They would test generated SQL shape, not SQLite constraints, migration behavior, or Gorm/SQLite round trips.
- Assumption: Because login locking fields are added, this slice should implement login state transitions. That would merge #9 into #7 and make the slice harder to verify; this slice should only persist the data needed by later auth behavior.
- Assumption: The old service/API can keep compiling by preserving stale repository methods such as `GetByEmail`. That would leak deleted product concepts into the storage contract and make later slices fight the schema.

### 2. Bedrock truths

- A SQLite schema migration is durable state mutation. The true database contract is the SQL migration sequence plus the live SQLite engine behavior.
- The MVP supports exactly one user account, but the database row still needs a stable primary key for JWT/current-user/password-change slices.
- Authentication persistence needs only: identity, username, password hash, failed-login counter, lock expiry, and timestamps.
- A password hash is not a password. The table/model should make this distinction in the field name.
- A missing username during login is not a storage failure; auth code must be able to collapse missing user and wrong password into one invalid-credentials outcome.
- Failed-login count and lock expiry are data; the policy of “5 failures lock 15 minutes, success clears” belongs to #9 service logic.
- Tests that claim repository correctness must verify data survives create/retrieve/update and that SQLite enforces uniqueness.

### 3. Rebuild from truths

1. Define `users` explicitly in SQL migration `000002_users`, after the existing baseline, because the database contract must be versioned and reversible.
2. Define one primary key column `id` and one unique login name column `username`; do not keep scaffold `user_id`, `email`, or soft-delete columns.
3. Define `password_hash`, `failed_login_count`, and nullable `locked_until` so #9 can update authentication state without schema changes.
4. Let Gorm map `model.User` to the same column names, using explicit tags where they prevent drift.
5. Keep repository behavior narrow: create, get by username, update. Retain an ID lookup only if required to keep existing compile paths alive, and if retained it must use `id`, not `user_id`.
6. Test the repository by applying the checked-in migrations to a temporary SQLite file, constructing `repository.NewRepository` / `NewUserRepository`, then observing behavior through repository methods.
7. Update stale compile-time references from old model fields to the new vocabulary without implementing the later auth workflows.

### 4. Contrast with convention

A conventional scaffold-driven path would rename only a few fields or use `AutoMigrate` to let Gorm infer a table. That optimizes for local convenience but fails the fundamental constraint: this product’s SQLite file is durable user data and schema changes must be explicit, testable, and reversible. The essential difference is that the migration is the source of truth, not the transient Go struct.

### 5. Conclusion

The simplest correct design is a versioned SQLite `users` migration plus a small Gorm-backed repository whose public behavior is tested against real migrated SQLite. Stale Nunu user concepts must be removed from active contracts now, while login/initialization/password-change policy remains in later slices.

## Architecture and Boundaries

### Storage boundary

Add the next migration pair:

```text
migrations/000002_users.up.sql
migrations/000002_users.down.sql
```

Recommended SQLite schema:

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    failed_login_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
    locked_until DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_users_username ON users (username);
```

`down` should drop the index/table for this migration boundary only. Because `cmd/migration -direction down` uses `Steps(-1)`, applying baseline + users then down should leave the baseline migration in place and remove only `users`.

### Model boundary

`internal/model.User` should map directly to `users` and contain no template-only fields.

Recommended Go shape:

```go
type User struct {
    Id               uint       `gorm:"primaryKey;column:id"`
    Username         string     `gorm:"column:username;not null;uniqueIndex"`
    PasswordHash     string     `gorm:"column:password_hash;not null"`
    FailedLoginCount int        `gorm:"column:failed_login_count;not null;default:0"`
    LockedUntil      *time.Time `gorm:"column:locked_until"`
    CreatedAt        time.Time  `gorm:"column:created_at"`
    UpdatedAt        time.Time  `gorm:"column:updated_at"`
}
```

Notes:

- `LockedUntil` is nullable; unlocked accounts should not need a sentinel timestamp.
- `FailedLoginCount` is stored but not interpreted in this slice.
- `PasswordHash` must replace raw `Password` vocabulary everywhere it touches the model.
- `TableName() string { return "users" }` remains useful because it binds the model to the migration table.

### Repository boundary

Required behavior:

```go
type UserRepository interface {
    Create(ctx context.Context, user *model.User) error
    GetByUsername(ctx context.Context, username string) (*model.User, error)
    Update(ctx context.Context, user *model.User) error
}
```

Implementation contracts:

- `Create` uses the existing `Repository.DB(ctx)` seam and returns SQLite/Gorm errors unchanged, including unique constraint errors.
- `GetByUsername` queries `username = ?`; if Gorm returns `ErrRecordNotFound`, return `nil, nil`.
- `Update` persists the row by primary key and must round-trip `PasswordHash`, `FailedLoginCount`, and `LockedUntil`.
- Do not keep or add `GetByEmail`.
- If existing service/API compile paths require an ID lookup before later auth slices, implement it as `GetByID(ctx, id uint)` or equivalent using `id = ?`; keep it out of the core acceptance criteria unless a requirement-driven test already exercises it.

### Cross-layer compatibility boundary

This slice changes a storage vocabulary that old scaffold code still consumes.

Allowed compatibility edits:

- Update request/response/test fixtures from email/profile vocabulary to username/id vocabulary only where required to compile.
- Update mocks generated from `UserRepository` after interface changes.
- Remove stale references to `model.User.Email`, `Nickname`, `Password`, `UserId`, and `DeletedAt`.

Disallowed scope expansion:

- Do not implement first-initialization gating.
- Do not implement password policy, bcrypt cost 12 changes, JWT 30-day behavior, failure-count transitions, or account-lock decisions.
- Do not introduce a user session table or any multi-user model.

If a stale service test fails because it asserts old scaffold behavior, rewrite or remove only the stale assertion needed for compile/test hygiene; do not invent new auth behavior outside #7 acceptance.

## Data Flow

```text
migrations/000001_baseline.*
  -> migrations/000002_users.*
  -> temp/prod SQLite file
  -> repository.NewDB / NewRepository
  -> UserRepository.Create / GetByUsername / Update
  -> later #8/#9/#10 service behavior
```

For this slice, repository tests stop at the repository seam. Later service/handler tests own auth policy and HTTP response mapping.

## Test Strategy

Use behavior-level integration tests under `test/server/repository` with a real temporary SQLite file.

Test harness requirements:

- Build a temp SQLite DSN with `_busy_timeout=5000`.
- Run `internal/migration.Run` against the checked-in `migrations/` directory before constructing the repository.
- Close the underlying SQL DB in cleanup.
- Use `repository.NewRepository` and `repository.NewUserRepository`; do not instantiate Gorm tables with `AutoMigrate`.

Requirement-driven scenarios:

1. **Migration creates users table**
   - Given a fresh SQLite file, applying all migrations succeeds.
   - `users` table exists and duplicate `username` is rejected by real SQLite.
2. **Migration down removes only latest users boundary**
   - Given baseline + users are applied, running down once removes `users` and leaves baseline behavior/table in place.
3. **Create then get by username**
   - Given a valid `model.User`, `Create` succeeds and fills a non-zero `Id`; `GetByUsername` returns the same username/hash/state.
4. **Missing username**
   - `GetByUsername` for an absent username returns `(nil, nil)`.
5. **Duplicate username**
   - Creating a second row with the same username returns an error from the real unique constraint.
6. **Update auth state**
   - After create, changing `PasswordHash`, `FailedLoginCount`, and `LockedUntil` then calling `Update` persists those fields when fetched again.
7. **Stale field regression**
   - Code/tests no longer reference deleted model fields or `GetByEmail`.

TDD execution must remain vertical: write scenario 1 test, implement only enough to pass, then scenario 2, and so on. The scenario list is not permission to write all tests before production code.

## Compatibility and Migration Notes

- New development databases migrate from baseline version 1 to users version 2.
- Existing local SQLite files created by old `AutoMigrate(&model.User{})` may contain incompatible scaffold columns. This slice does not promise data migration from unpublished scaffold schemas; use a fresh DB or backup/reset local dev DB if needed.
- Production deployment should continue to run `cmd/migration` before `server` / `task`; this slice only adds a new migration to that existing mechanism.
- The model’s timestamp fields use Gorm conventions, while SQL defaults make direct SQL inserts safe in tests or future tooling.

## Operational and Rollback Considerations

- Rollback point 1: revert only `000002_users.*` before any data-bearing follow-up migrations depend on it.
- Rollback point 2: repository interface rename from `GetByEmail` to `GetByUsername` requires mock regeneration and service/test compile fixes together.
- Rollback point 3: if broad service/API scaffold tests fail, keep the repository slice intact and adapt stale tests to new vocabulary rather than reintroducing email/profile fields.
- Running `cmd/migration -direction down` in a dev DB after this slice removes the `users` table and any user rows; this is expected for rollback testing.
