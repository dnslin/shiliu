# Shiliu

@AGENTS.md

Shiliu is a single-instance subscription center for developers and information-heavy users.

## Development

This is a Go backend with Gin, GORM, SQLite, Wire, Swagger, and background tasks.

```bash
go test ./...
go build ./...
go vet ./...
nunu wire all
swag init -g cmd/server/main.go -o ./docs
```

Run Wire after changing providers, constructors, module imports, or generated entrypoints.
Run Swagger generation after changing public routes, request/response structs, or annotations.

## Where to Look

| Task | Location |
|------|----------|
| API contracts and errors | api/v1/ |
| HTTP handlers and route behavior | internal/handler/ |
| Business rules and feed/AI workflows | internal/service/ |
| SQLite persistence | internal/repository/ |
| Domain records | internal/model/ |
| Background scheduling | internal/task/ |
| Versioned schema changes | migrations/ |
| Reusable packages | pkg/ |
| Tests and mocks | test/ |
| Runtime entrypoints | cmd/ |
| Deployment | deploy/ |

## Agent Workflow

Use Trellis guidance from `AGENTS.md` first when task scope is more than a small direct edit.
Before modifying backend code, read the nearest `.trellis/spec/backend/` guideline listed by the relevant index.
Before modifying frontend code, read the nearest `.trellis/spec/frontend/` guideline listed by the relevant index.
Verify reviewer findings against code before prioritizing them.

## Guidance

Context-specific guidance lives in nested `CLAUDE.md` files throughout the repo.
Claude loads the closest file for the directory it reads; closest guidance wins.
Keep root guidance general. Put layer rules beside the layer.

## Quality Gates

For backend code changes, prefer this baseline unless the change is docs-only:

```bash
go build ./...
go vet ./...
go test ./...
git diff --check
```
