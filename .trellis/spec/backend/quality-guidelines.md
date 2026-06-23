# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

### Scenario: Go Module Path and Wire Regeneration

#### 1. Scope / Trigger
- Trigger: changing `go.mod`, internal Go import paths, Wire provider sets, or generated Wire files.
- Applies to the backend module root and the Wire entrypoints under `cmd/server/wire`, `cmd/task/wire`, and `cmd/migration/wire`.

#### 2. Signatures
- Module declaration: `module shiliu` in `go.mod`.
- Primary generation command: `nunu wire all`.
- Per-entry fallback only after a tool panic: `cd cmd/<entrypoint>/wire && go run -mod=mod github.com/google/wire/cmd/wire`.

#### 3. Contracts
- All internal imports must use `shiliu/...`.
- `wire_gen.go` files are generated artifacts; do not hand-edit them.
- If `go.sum` changes during Wire generation, verify the additions belong to the Go/Wire toolchain before committing.

#### 4. Validation & Error Matrix
- `shuliu/` found in any `.go` file -> fail the slice and replace the stale import.
- `nunu wire all` fails transiently -> retry once after capturing the error.
- A single Wire entrypoint panics after retry -> generate only that entrypoint with the per-entry fallback, then rerun `nunu wire all` to prove the full command is reproducible.
- `go build ./...`, `go vet ./...`, or `go test ./...` fails -> fix before reporting completion.

#### 5. Good/Base/Bad Cases
- Good: `go.mod` says `module shiliu`, all internal imports are `shiliu/...`, all three `wire_gen.go` files are regenerated, and build/vet/test pass.
- Base: non-Wire Go import replacement still requires `nunu wire all`, because generated code may contain stale import strings.
- Bad: manually editing `wire_gen.go` without rerunning Wire, or committing a stale `shuliu/...` import in tests.

#### 6. Tests Required
- Search assertion: no `shuliu/` remains in `*.go` files.
- Generation assertion: `nunu wire all` exits successfully and touches all three Wire entrypoints when needed.
- Build assertion: `go build ./...` succeeds.
- Vet assertion: `go vet ./...` succeeds.
- Test assertion: `go test ./...` succeeds so existing test packages still compile.

#### 7. Wrong vs Correct

Wrong:
```go
import "shuliu/internal/repository"
```

Correct:
```go
import "shiliu/internal/repository"
```

Wrong:
```text
Edit cmd/server/wire/wire_gen.go by hand to fix imports.
```

Correct:
```text
Run nunu wire all and commit the generated wire_gen.go output.
```

---

## Testing Requirements

### Scenario: API Contract and Route Registration Tests

#### 1. Scope / Trigger
- Trigger: changing backend API route prefixes, Swagger/OpenAPI runtime metadata, response envelope helpers, pagination helpers, or any frontend-facing JSON contract.
- Applies to `api/v1`, `internal/server`, `internal/router`, handler tests, and any test that claims to verify a public API contract.

#### 2. Signatures
- Runtime server constructor: `func NewHTTPServer(deps router.RouterDeps) *http.Server`.
- API response envelope: `v1.Response{Code, Message, Data}` serialized as `{"code":...,"message":"...","data":...}`.
- Pagination request contract: `page` and `pageSize` query fields normalized by the shared API helper.
- Route prefix contract: production routes are registered by `NewHTTPServer` and router init functions, not by ad-hoc test-only groups.

#### 3. Contracts
- Route prefix tests must exercise the same registration path production uses. For server-level route prefixes, instantiate `NewHTTPServer` with minimal test dependencies and send HTTP requests through the returned handler.
- Swagger/BasePath assertions must read the runtime metadata set by the real server setup.
- Response and pagination helper tests may use focused Gin test contexts because the helper itself is the public seam.
- Tests must assert observable HTTP behavior: status code, JSON fields, registered route exists, old route does not exist when the contract intentionally changed.

#### 4. Validation & Error Matrix
- Test recreates `router.Group("/api/v1")` locally and then asserts `/api/v1` works -> invalid test; it proves only the test setup.
- Test calls `NewHTTPServer` and asserts `/api/v1/register` reaches the registered handler while `/v1/register` returns 404 -> valid route-prefix contract test.
- Test checks only `docs.SwaggerInfo.BasePath` but not an HTTP route -> incomplete when route registration also changed.
- Test checks only an HTTP route but not Swagger/BasePath after a BasePath change -> incomplete when runtime docs metadata also changed.

#### 5. Good/Base/Bad Cases
- Good: `NewHTTPServer(testDeps)` is used, `/api/v1/<route>` returns the expected handler-level status, `/v1/<route>` is not registered, and `docs.SwaggerInfo.BasePath` matches the new prefix.
- Base: focused helper tests construct `gin.CreateTestContext` for `ParsePageRequest` or `HandleSuccess`, because those helpers are the unit under test.
- Bad: a route-prefix test creates a local `gin.New()` and manually registers the expected group; this is tautological and can pass while production server setup is wrong.

#### 6. Tests Required
- Changed response helper -> test serialized envelope fields and HTTP status through a Gin response recorder.
- Changed pagination helper -> test defaults, boundary normalization, malformed query fallback, `LimitOffset`, and `total=0` metadata.
- Changed route prefix -> test the real server/router registration path accepts the new prefix and rejects the old prefix when no compatibility alias is intended.
- Changed Swagger runtime metadata -> test the runtime `docs.SwaggerInfo.BasePath` value after server setup.

#### 7. Wrong vs Correct

Wrong:
```go
router := gin.New()
api := router.Group("/api/v1")
api.GET("/contract", handler)
// This only proves the test registered /api/v1, not production code.
```

Correct:
```go
server := NewHTTPServer(newTestRouterDeps())
request := httptest.NewRequest(http.MethodPost, "/api/v1/register", nil)
response := httptest.NewRecorder()
server.ServeHTTP(response, request)
require.NotEqual(t, http.StatusNotFound, response.Code)
require.Equal(t, "/api/v1", docs.SwaggerInfo.BasePath)
```

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
