# Background Tasks

## Purpose

`internal/task` owns scheduled background execution for feed fetching and automatic AI summaries.

## Patterns

- Keep scheduler setup explicit and fail fast when jobs cannot be registered.
- Use singleton behavior for jobs that must not overlap.
- Separate request-scoped actions from background work that must finish state transitions.
- Persist terminal state for claimed work even when downstream processing fails.
- Keep task wiring compatible with `cmd/task` and Wire generation.

## Runtime Boundary

- `cmd/task` is the long-running background entrypoint.
- `cmd/server` should not register scheduled jobs.
- `cmd/migration` owns schema migration and should finish before task startup in deployment.
- Job dependencies are supplied through constructors and Wire, not package globals.
- Configuration defaults must be safe for a single-instance deployment.

## Scheduling

- Global intervals belong in configuration.
- Per-source custom scheduling is outside the MVP unless product scope changes.
- Task code may coordinate services, but services keep business rules.
- Prefer explicit job names in logs and tests so failures are traceable.

## Testing

- Test scheduler registration and job behavior without relying on wall-clock sleeps when possible.
- Test claim/conflict behavior for automatic summaries.
- Verify state transitions for success, failure, and canceled request contexts.
- Cover disabled automatic-summary configuration so the job exits cleanly.

## Anti-Patterns

- Do not let overlapping runs process the same source/item concurrently.
- Do not swallow job registration errors.
- Do not leave claimed summaries stuck in `pending`.
- Do not add fetch history or queue semantics without an explicit product decision.
- Do not make scheduled work depend on HTTP request lifecycle.
