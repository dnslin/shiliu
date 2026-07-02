# Backend Internals

## Layers

- `handler`: HTTP boundary, binding, status mapping.
- `service`: business rules, orchestration, external fetch/AI calls.
- `repository`: SQLite persistence through GORM.
- `model`: persisted records and domain constants.
- `middleware`: request cross-cutting concerns.
- `task`: scheduled/background work.

## Contracts

- Keep HTTP concerns in handlers.
- Keep database details in repositories.
- Services own business decisions and transaction boundaries.
- Repositories expose public seams through interfaces and constructors.
- Background work must not depend on request cancellation when final state must be persisted.

## Imports

- Internal imports use `shiliu/...`.
- Generated Wire files are regenerated, not hand-edited.
- Avoid reintroducing scaffold datastore dependencies.

## Validation

```bash
nunu wire all
go build ./...
go vet ./...
go test ./...
```

## Anti-Patterns

- Do not parse domain errors by text.
- Do not mix HTTP status selection into services.
- Do not let repositories decide API response shape.
- Do not keep MySQL, PostgreSQL, Redis, or MongoDB assumptions in live backend code.
