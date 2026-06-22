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

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
