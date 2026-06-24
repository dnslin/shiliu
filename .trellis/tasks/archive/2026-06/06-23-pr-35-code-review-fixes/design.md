# Design: PR #35 code-review fixes

## First-Principles Reasoning

### 1. Challenge assumptions

- Assumption: a service-level duplicate username error can use HTTP 500 because the scaffold handler already does that. This is wrong: duplicate input is not a server failure, and a stable account error already exists.
- Assumption: a pre-insert `GetByUsername` check is sufficient for uniqueness. This is incomplete: the database unique index is the only atomic uniqueness guard.
- Assumption: `Save` means update. In Gorm, `Save` is an upsert-like operation for zero primary keys, so it does not express an update-only repository contract.
- Assumption: parsing into `uint64` is safe because database IDs are unsigned. This ignores the raw resource fact that Go `uint` width changes by architecture.
- Assumption: all profile lookup errors are bad requests. This collapses distinct facts: malformed token subject, missing user row, and storage failure.
- Assumption: fixing these findings should implement the later auth slice. That would violate the already approved #7 boundary; this fix should repair existing seams without adding lockout/init/password-change policy.

### 2. Bedrock truths

- HTTP 5xx means the server failed to process a valid class of request; duplicate username is a predictable account-state/client conflict.
- A database unique constraint is atomic; a read-before-write check is not atomic under concurrent requests.
- Repository methods are public seams. Their names must match their effects: `Update` must not create.
- Go `uint` is either 32 or 64 bits depending on target architecture. Narrowing from `uint64` to `uint` without checking can change the numeric value.
- A JWT subject/current-user id is untrusted input at the service boundary even when the token signature is valid; it must be parsed into the exact domain type safely.
- Existing error constants already express `ErrUsernameAlreadyUse`, `ErrBadRequest`, `ErrUnauthorized`, `ErrNotFound`, and `ErrInternalServerError`; adding new error types is unnecessary unless a requirement lacks representation.

### 3. Rebuild from truths

1. Keep the database unique index as the source of truth for duplicate username enforcement.
2. Teach the service or repository boundary to translate a duplicate-key create failure into `v1.ErrUsernameAlreadyUse` so both normal and race paths converge.
3. Teach the register handler to map `ErrUsernameAlreadyUse` to a non-500 response. Prefer `409 Conflict` because the request shape is valid but conflicts with account state.
4. Replace `Save` with an update-only Gorm operation scoped by primary key. Check `RowsAffected` so missing IDs become `ErrNotFound` instead of success.
5. Reject `Update` with `Id == 0` before hitting the DB because zero is not an existing persisted user identity.
6. Parse user IDs using `strconv.ParseUint` with the exact platform width (`strconv.IntSize`) before converting to `uint`.
7. Map `GetProfile` service errors by known type: bad/overflowing id remains `400 Bad Request`, `ErrNotFound` becomes `404 Not Found`, unknown errors become `500` or existing handler default behavior.

### 4. Contrast with convention

A conventional patch might add one if-statement in the handler for the visible duplicate case and leave repository `Save` untouched. That fixes one symptom but not the atomic source of uniqueness or the update contract. The fundamental repair is to align each boundary with the invariant it owns: database enforces uniqueness, service normalizes account errors, repository guarantees update-only behavior, handler maps known domain errors to HTTP status.

### 5. Conclusion

The smallest correct design is boundary-specific: normalize duplicate create errors, make repository updates update-only, parse IDs without narrowing, and map known service errors explicitly. This fixes the review findings while preserving the #7 slice boundary.

## Architecture and Boundaries

### API / handler boundary

`UserHandler.Register` should distinguish known account/client errors from server failures:

- `v1.ErrUsernameAlreadyUse` -> HTTP 409 Conflict with the same API error body.
- Other errors -> existing internal-server handling.

`UserHandler.GetProfile` should distinguish:

- missing context user id -> HTTP 401 Unauthorized (unchanged);
- service parse/validation error -> HTTP 400 Bad Request;
- `v1.ErrNotFound` -> HTTP 404 Not Found;
- other errors -> HTTP 500 Internal Server Error or existing safe default.

### Service boundary

`Register` still performs a pre-check for the normal duplicate path, because it gives a cheap friendly error before attempting a write. But the transaction/create path must also normalize unique-constraint failures to `v1.ErrUsernameAlreadyUse` so concurrency cannot leak raw DB errors.

Implementation option:

```go
if err := s.userRepo.Create(ctx, user); err != nil {
    if repository.IsUniqueConstraintError(err) {
        return v1.ErrUsernameAlreadyUse
    }
    return err
}
```

Keep this helper in the repository package if it is tied to SQLite/Gorm error text, because the repository layer owns storage error mechanics. Do not add MySQL/PostgreSQL handling.

`parseUserID` should parse with `strconv.IntSize`:

```go
id, err := strconv.ParseUint(userId, 10, strconv.IntSize)
if err != nil { return 0, err }
return uint(id), nil
```

This rejects overflow for the running platform before conversion.

### Repository boundary

`Update` should express update-only behavior. Preferred shape:

```go
if user.Id == 0 { return v1.ErrBadRequest }
result := r.DB(ctx).Model(&model.User{}).
    Where("id = ?", user.Id).
    Updates(map[string]interface{}{...})
if result.Error != nil { return result.Error }
if result.RowsAffected == 0 { return v1.ErrNotFound }
return nil
```

Fields required by #7 update behavior:

- `password_hash`
- `failed_login_count`
- `locked_until`

Also preserve timestamp update behavior if Gorm updates `updated_at`; if explicit map updates bypass automatic timestamps, set `updated_at` to current time only if tests or existing behavior require it. Do not add business policy around `failed_login_count` / `locked_until`.

### Testing strategy

Use vertical, behavior-level tests through public seams:

1. Handler duplicate username maps to non-500 conflict.
2. Service create-time duplicate error maps to `ErrUsernameAlreadyUse`.
3. Repository update rejects zero ID and does not create a row.
4. Repository update rejects missing non-zero ID and does not create a row.
5. Handler get-profile maps not found to HTTP 404.
6. Service get-profile overflow/non-numeric id rejects before repository lookup.

Repository tests must continue using real SQLite and checked-in migrations through the command seam documented in `.trellis/spec/backend/database-guidelines.md`.

## Compatibility Notes

- No schema migration is required; all changes are behavioral around existing `users` schema.
- Existing JWT claim name `UserId` remains unchanged in this slice because the broader auth/JWT cleanup belongs to later auth work.
- Existing login behavior remains deliberately out of scope except where tests need compile-safe vocabulary.

## Rollback Points

- Handler error mapping can be reverted independently if response contracts need a different status code.
- Repository `Update` can be reverted independently, but doing so reintroduces the insert-on-zero-ID risk.
- Duplicate unique-error normalization can be adjusted if future repository error wrapping is introduced.
