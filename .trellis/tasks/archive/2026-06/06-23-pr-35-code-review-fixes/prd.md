# Fix PR 35 code review findings

## Goal

Fix the verified PR #35 code-review findings in the user account slice without expanding into later auth-policy slices (#8/#9/#10). The repaired code should return stable API errors for duplicate usernames, make repository update behavior update-only, handle profile lookup failures deliberately, and avoid platform-dependent user-id narrowing.

User value: account APIs should fail predictably for normal client errors and should not accidentally create or look up the wrong user when auth/account data is malformed or stale.

## Source Requirements

- Current branch / PR: `issue-7-user-model-repository`, PR #35.
- Review command result: five verified/non-refuted findings from `/code-review #PR 35`.
- Prior slice artifacts: `.trellis/tasks/archive/2026-06/06-23-issue-7-user-model-repository/`.
- Backend specs:
  - `.trellis/spec/backend/database-guidelines.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/guides/index.md`

## Confirmed Facts

- `internal/service/user.go` returns `v1.ErrUsernameAlreadyUse` when `GetByUsername` finds an existing user.
- `internal/handler/user.go` currently maps any `Register` service error to HTTP 500.
- `UserRepository.Create` returns SQLite/Gorm errors unchanged, so a concurrent unique-index failure can bypass the pre-check and bubble as a raw DB error.
- `internal/repository/user.go` uses `Save` in `Update`; Gorm `Save` may create a row when the primary key is zero.
- `internal/handler/user.go` currently maps any `GetProfile` service error to HTTP 400 Bad Request.
- `parseUserID` parses `uint64` and casts to `uint`, which can narrow on 32-bit targets.
- #7 remains a persistence/model/repository slice. Password policy, failed-login lock transitions, account initialization semantics, and full login behavior remain out of scope for this fix.

## Requirements

1. Duplicate username registration must return a stable account/client error instead of HTTP 500.
   - Normal duplicate path: service returns `v1.ErrUsernameAlreadyUse`; handler maps it to a non-500 response.
   - Race path: if the database unique constraint rejects `Create`, service/repository code maps that duplicate condition to `v1.ErrUsernameAlreadyUse` rather than leaking a raw DB error.
2. Keep missing username lookup semantics unchanged: `UserRepository.GetByUsername` returns `(nil, nil)` for not found.
3. `UserRepository.Update` must be update-only.
   - Calling `Update` with `Id == 0` must fail and must not insert a row.
   - Calling `Update` with a non-existent non-zero `Id` must fail instead of silently succeeding or inserting.
   - Calling `Update` for an existing row must continue to persist `PasswordHash`, `FailedLoginCount`, and `LockedUntil`.
4. `GetProfile` error mapping must be deliberate and testable.
   - Missing user for a valid numeric token subject should not be reported as Bad Request.
   - Invalid/non-numeric token subject remains an invalid request or auth failure, but must be explicitly tested.
5. `parseUserID` must reject values that cannot safely fit in `uint` on the running platform.
6. Tests must cover every fixed behavior through public seams:
   - handler/register duplicate response;
   - service/register duplicate race unique-constraint mapping;
   - repository/update zero ID and non-existent ID behavior;
   - handler/get-profile missing user response;
   - service/get-profile invalid ID overflow behavior.
7. Do not introduce new routes, first-initialization behavior, lockout policy, password-change behavior, or multi-user semantics.
8. Preserve SQLite-only and migration-backed repository test patterns; do not replace the existing subprocess migration seam with direct `internal/migration.Run` inside repository tests.

## Acceptance Criteria

- [ ] Duplicate username via handler returns a non-500 response with `ErrUsernameAlreadyUse` code/message.
- [ ] Concurrent/race-style duplicate username at service/create time maps to `ErrUsernameAlreadyUse`.
- [ ] Repository duplicate username behavior is still enforced by real SQLite unique constraints.
- [ ] `UserRepository.Update` rejects `Id == 0` without creating a user.
- [ ] `UserRepository.Update` rejects a non-existent non-zero `Id` without creating a user.
- [ ] Existing update success behavior remains covered and passing.
- [ ] `GetProfile` maps `v1.ErrNotFound` from service/repository to a non-400 not-found/auth response.
- [ ] Invalid or overflowing user-id strings are rejected deterministically before repository lookup.
- [ ] Requirement-driven focused tests pass for handler, service, and repository packages.
- [ ] `go test ./...` passes.
- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes.

## Out of Scope

- #8 first-time account initialization and second-account blocking.
- #9 password length policy, bcrypt cost 12, 30-day JWT changes, failed-login count transitions, lockout behavior, and invalid-credentials unification.
- #10 password-change endpoint/service.
- Changing username case-sensitivity; the prior PRD only required unique `username`, not case-insensitive uniqueness.
- Reworking repository test migration performance; the subprocess seam is currently required by `.trellis/spec/backend/database-guidelines.md` to avoid duplicate SQLite driver registration.

## Open Questions

None blocking. The response-status choices will follow the simplest stable mapping from existing error names unless implementation evidence shows an existing project helper that should be reused.
