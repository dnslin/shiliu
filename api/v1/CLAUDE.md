# API Contracts

## Purpose

`api/v1` owns public request/response shapes, error identities, pagination helpers, and Swagger-facing comments.

## Patterns

- Keep response envelopes stable: `code`, `message`, `data`.
- Define reusable domain errors here; handlers and services compare them with `errors.Is`.
- Keep API structs explicit. Do not leak GORM models as public contracts.
- Pagination defaults and limits belong in shared helpers, not in each handler.
- Swagger annotations must match runtime routes and JSON field names.

## Version Boundary

- Treat this directory as a public contract even when only the web UI consumes it.
- Prefer additive fields when compatibility is expected.
- Keep enum strings aligned with model/service constants.
- Name request and response structs by operation when reuse would blur meaning.

## Error Handling

- Duplicate/conflict domain errors map to `409`.
- Missing records map to `404`.
- Bad request or invalid token subject maps to `400`.
- Unknown server faults map to `500`.

## Testing

- Contract changes need focused tests for JSON fields and status codes.
- Route prefix changes must test the real server/router path, not a local test-only router.
- Pagination changes need boundary tests, malformed query tests, and overflow regressions.
- Error additions need at least one handler or helper test that proves the mapped envelope.

## Anti-Patterns

- Do not compare error message strings.
- Do not add handler-specific response envelopes.
- Do not expose internal persistence fields by accident.
- Do not update Swagger without checking runtime route registration.
- Do not rename JSON fields without updating tests, docs, and frontend callers.
