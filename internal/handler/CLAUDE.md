# Handlers

## Purpose

Handlers translate HTTP requests into service calls and service errors into API responses.

## Patterns

- Bind and validate request payloads at the boundary.
- Pull context/user identity from middleware-provided values.
- Call services once the boundary input is normalized.
- Map known `api/v1` errors explicitly with `errors.Is`.
- Log only unexpected server faults at error level.

## Route Contracts

- Route prefix behavior must come from production router/server setup.
- Keep handler methods thin; move branching workflow decisions to services.
- Use shared response helpers for success and error envelopes.
- Keep Swagger comments near the handler method that owns the route.
- Keep authentication assumptions visible in route grouping or middleware setup.

## Status Mapping

- `v1.ErrUsernameAlreadyUse` -> `409 Conflict`.
- `v1.ErrNotFound` -> `404 Not Found`.
- `v1.ErrBadRequest` -> `400 Bad Request`.
- Unknown errors -> `500 Internal Server Error`.

## Testing

- Exercise real handler methods through Gin test contexts or server registration.
- Assert HTTP status plus response envelope fields.
- Include negative tests for each known domain error branch.
- Use production registration for route-prefix and middleware contract tests.

## Anti-Patterns

- Do not collapse all errors into `400`.
- Do not log expected client errors as server errors.
- Do not duplicate pagination or response helpers.
- Do not register routes only inside tests and treat that as production coverage.
- Do not let handler tests pass without asserting the response body contract.
