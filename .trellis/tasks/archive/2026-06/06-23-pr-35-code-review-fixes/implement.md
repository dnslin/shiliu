# Implementation Plan: PR #35 code-review fixes

## Ordered Checklist

1. Activate the task after artifact review.
2. Refresh pre-development context with Trellis specs before editing.
3. Add/fix tests one vertical slice at a time:
   - Handler register duplicate username returns HTTP 409 and `ErrUsernameAlreadyUse` body.
   - Service register create-time unique constraint error returns `ErrUsernameAlreadyUse`.
   - Repository update with `Id == 0` fails and does not create a row.
   - Repository update with a non-existent non-zero `Id` fails with `ErrNotFound` and does not create a row.
   - Handler get-profile missing user returns HTTP 404.
   - Service get-profile invalid/overflowing id returns an error without repository lookup.
4. Implement duplicate username HTTP mapping in `internal/handler/user.go`.
5. Implement duplicate unique-constraint normalization for service create path.
   - Prefer a small repository helper for SQLite/Gorm duplicate errors if direct error matching is needed.
   - Keep `GetByUsername` missing semantics unchanged.
6. Implement update-only repository behavior in `internal/repository/user.go`.
   - Reject `Id == 0` before DB write.
   - Use `WHERE id = ?` update and check `RowsAffected`.
   - Preserve successful update of `PasswordHash`, `FailedLoginCount`, and `LockedUntil`.
7. Implement safe user-id parsing with `strconv.IntSize`.
8. Implement explicit `GetProfile` error mapping in `internal/handler/user.go`.
9. Run focused tests after each slice when practical.
10. Run final validation.

## Validation Commands

Focused:

```bash
go test ./test/server/handler ./test/server/service ./test/server/repository
```

Full:

```bash
go test ./...
go build ./...
go vet ./...
```

Optional static checks if touched paths suggest drift:

```bash
git diff --check
git diff --name-only main...HEAD
```

## Risky Files / Rollback Points

- `internal/handler/user.go`: response status mapping changes API behavior; tests must assert exact status and response code.
- `internal/service/user.go`: unique-error normalization must not collapse all DB errors into duplicate username.
- `internal/repository/user.go`: update-only implementation must still persist nullable `LockedUntil` correctly.
- `test/server/repository/user_test.go`: keep the migration command seam; do not import `internal/migration` in this package.
- `test/mocks/repository/user.go` and `test/mocks/service/user.go`: regenerate/update only if interfaces change. Prefer no interface shape change.

## Completion Gate

Implementation is complete only when:

- All six requirement scenarios have tests.
- Focused handler/service/repository tests pass.
- `go test ./...`, `go build ./...`, and `go vet ./...` pass, or any external/non-task failure is captured with exact output and justified.
- No scope expansion into #8/#9/#10 is introduced.
