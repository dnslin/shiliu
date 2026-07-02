# Services

## Purpose

Services own Shiliu business behavior: subscriptions, content items, tags, folders, AI configuration, AI summaries, and feed fetching.

## Patterns

- Keep business rules close to the service that owns the workflow.
- Use repository interfaces and transaction seams instead of raw DB access.
- Convert untrusted token/user id strings with width-safe parsing before repository calls.
- Preserve domain error identities from repositories when callers need exact status mapping.
- Sanitize feed-provided HTML through shared package utilities before deriving available text.

## Domain Boundaries

- A subscription feed becomes persisted only after fetch/parse succeeds.
- Content item processing status is user-controlled, not reading/listening progress.
- Tags organize content items; folders organize subscription feeds.
- Audio progress is consumption progress and must not mark items completed automatically.
- Obsidian export is plain Markdown output, not synchronization with a vault.

## AI and Fetching

- AI summaries use available text only; do not pretend to understand unavailable full text or audio.
- Automatic summaries must respect the global enabled flag and content-type scope.
- Feed fetching updates only current/last fetch diagnostics; do not create fetch history unless the product scope changes.
- Manual AI summary remains available even when automatic summary is disabled.

## Testing

- Mock repositories for service branching and transaction behavior.
- Use real feed testdata for parsing, dedupe, sanitization, and fetch behavior.
- Assert repository mocks are not called when input validation fails.
- Cover both success and domain-error paths when service behavior feeds handler status mapping.

## Anti-Patterns

- Do not return handler-specific status concepts from services.
- Do not parse repository errors by message text.
- Do not update existing content items during later feed fetches unless the contract changes.
- Do not leave async state as `pending` when post-processing fails or request context is canceled.
- Do not add MVP-excluded capabilities through service side effects.
