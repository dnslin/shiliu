# Implementation Plan: User model, users migration, and repository

## Preconditions

- [x] Branch created: `issue-7-user-model-repository`.
- [x] Trellis task created: `.trellis/tasks/06-23-issue-7-user-model-repository`.
- [x] GitHub issue #7 read with `gh issue view 7`.
- [x] Parent issue #1 and local parent task artifacts inspected.
- [x] Blocker #5 migration mechanism confirmed complete in current code.
- [ ] User reviews/approves this planning package before `task.py start`.
- [ ] Run `python ./.trellis/scripts/task.py start 06-23-issue-7-user-model-repository` before implementation edits.
- [ ] Run `trellis-before-dev` before editing code to refresh backend/migration conventions.

## Public Interface Decision

Primary repository contract for this slice:

```go
Create(ctx context.Context, user *model.User) error
GetByUsername(ctx context.Context, username string) (*model.User, error)
Update(ctx context.Context, user *model.User) error
```

Model field decision:

```text
id, username, password_hash, failed_login_count, locked_until, created_at, updated_at
```

SQL migration decision:

```text
migrations/000002_users.up.sql
migrations/000002_users.down.sql
```

Rationale: this is the smallest durable storage contract that satisfies #7 and prepares #8/#9/#10 without preserving stale email/profile template concepts.

## Requirement-Driven Test Scenarios

TDD must proceed as vertical slices. Do not write all tests first.

1. **Tracer bullet: migrations create users table**
   - Given a fresh temporary SQLite database, running checked-in migrations succeeds and `users` exists.
2. **Down migration removes latest users boundary**
   - Given baseline + users are applied, running migration `down` once removes `users` while leaving the baseline boundary intact.
3. **Create and fetch by username**
   - Given a new user with username/password hash, repository create succeeds and fetch by username returns the same account.
4. **Missing username is not a repository error**
   - Given no matching row, `GetByUsername` returns `nil, nil`.
5. **Duplicate username is rejected by SQLite**
   - Given an existing username, creating another row with the same username returns an error from the real unique constraint.
6. **Update persists auth fields**
   - Given an existing user, updating password hash, failed login count, and locked-until persists those values.
7. **Deleted vocabulary regression**
   - Active code/tests no longer reference `model.User` fields `UserId`, `Nickname`, `Email`, `Password`, `DeletedAt`, or repository method `GetByEmail`.

## Vertical Slice Checklist

### Slice 1: Migration tracer bullet

- [ ] Add one focused failing test that applies all checked-in migrations to a temp SQLite DB and asserts the `users` table exists.
- [ ] Add `migrations/000002_users.up.sql` and `migrations/000002_users.down.sql` only enough to satisfy the test.
- [ ] Run the focused migration/repository test and make it pass.

### Slice 2: Migration rollback boundary

- [ ] Add one failing test for up then one `down` step.
- [ ] Ensure `down` removes `users` but not the baseline marker.
- [ ] Run the focused test and make it pass.

### Slice 3: Model and create/fetch repository behavior

- [ ] Refactor `internal/model/user.go` to the new field set.
- [ ] Refactor `internal/repository/user.go` from `GetByEmail` to `GetByUsername`.
- [ ] Replace repository test setup so it runs checked-in migrations instead of `AutoMigrate`.
- [ ] Add one failing create/fetch-by-username repository test.
- [ ] Implement create/fetch behavior and make the focused test pass.

### Slice 4: Missing username and duplicate username

- [ ] Add one failing test for missing username returning `nil, nil`; implement if needed.
- [ ] Add one failing test for duplicate username; rely on the real SQLite unique constraint.
- [ ] Run focused repository tests and make them pass.

### Slice 5: Update auth fields

- [ ] Add one failing test that updates `PasswordHash`, `FailedLoginCount`, and `LockedUntil`.
- [ ] Implement/adjust `Update` so the fetched row round-trips those fields.
- [ ] Run focused repository tests and make them pass.

### Slice 6: Compile fallout and generated mocks

- [ ] Search for deleted model fields and `GetByEmail` across active Go/test code.
- [ ] Update stale service/API/test references only enough to align vocabulary and compile.
- [ ] Regenerate `test/mocks/repository/user.go` from `internal/repository/user.go` if the interface changes.
- [ ] If Wire provider sets are unchanged, do not touch generated `wire_gen.go`; if a provider signature changes, run `nunu wire all` rather than hand-editing generated files.

### Slice 7: Validation

- [ ] Run focused tests for repository/migration packages.
- [ ] Run static searches:
  - `AutoMigrate` in user repository tests or production migration/server/task paths;
  - `GetByEmail`;
  - `.Email`, `.Nickname`, `.Password`, `.UserId`, `.DeletedAt` on `model.User` usages;
  - `go-sqlmock`, `sqlmock`, and `gorm.io/driver/mysql` in active tests.
- [ ] Run `go test ./...`.
- [ ] Run `go build ./...`.
- [ ] If `go test ./...` or `go build ./...` fails, capture the exact output, fix slice-owned failures, and retry once if the failure is transient.

## Focused Validation Commands

```bash
go test ./test/server/repository ./internal/migration
go test ./...
go build ./...
```

Static checks can use repository-aware search tooling or shell equivalents:

```bash
rg "GetByEmail|UserId|Nickname|Email|Password|DeletedAt|AutoMigrate|go-sqlmock|sqlmock|gorm.io/driver/mysql" internal test migrations api cmd
```

## Likely Files to Change

- `internal/model/user.go`
- `internal/repository/user.go`
- `migrations/000002_users.up.sql`
- `migrations/000002_users.down.sql`
- `test/server/repository/user_test.go`
- `test/mocks/repository/user.go`
- Stale compile references as needed in:
  - `api/v1/user.go`
  - `internal/service/user.go`
  - `internal/handler/user.go`
  - `test/server/service/user_test.go`
  - `test/server/handler/user_test.go`

## Risky Files / Rollback Points

- `migrations/000002_users.*`: schema mistakes affect SQLite state; verify up/down on temp DB before broad code changes.
- `internal/model/user.go`: deleting fields will surface compile errors across service/handler/tests; fix by new vocabulary, not by re-adding fields.
- `internal/repository/user.go`: interface rename requires mock regeneration and test updates in one slice.
- `test/server/repository/user_test.go`: must not fall back to `AutoMigrate`; the test harness should prove migration-backed schema.
- `test/mocks/repository/user.go`: generated file; regenerate from source rather than hand-maintain beyond emergency compile diagnosis.

## Completion Gate Before Finish Phase

- [ ] PRD acceptance criteria are satisfied or explicitly marked out of scope with reason.
- [ ] TDD cycle evidence is recorded in final response.
- [ ] Reusable migration/repository testing insight is added to `.trellis/spec/` only if implementation reveals a new convention not already documented.
- [ ] Changes are committed only during Trellis Finish phase after verification.
