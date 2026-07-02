# Tests

## Framework

Tests use Go's `testing` package with `testify`, `gomock`, Gin test helpers, and real SQLite where persistence matters.

## Layout

- `test/server/handler`: HTTP handler behavior.
- `test/server/service`: service behavior with mocks.
- `test/server/repository`: repository behavior over SQLite.
- `test/deploy`: static deployment contract tests.
- `test/mocks`: generated mocks.

## Fixtures

- Feed XML and HTML edge cases live beside service tests when they model parser input.
- Temporary SQLite files should be isolated per test.
- Static contract tests should read checked-in files, not duplicated expected snippets.
- Generated mocks belong under `test/mocks` and should change only through mock generation.

## Running Tests

```bash
go test ./...
```

Use focused packages while developing, then run the full suite before completion.

## Patterns

- Prefer behavior assertions through public seams.
- Repository tests should use migrations or realistic SQLite setup.
- Handler tests assert status code plus response envelope.
- Service tests assert validation short-circuits before repository calls.
- Regenerate mocks when interfaces change.
- Keep table tests readable: name each case by the behavior being protected.
- Use `require` for setup failures and `assert` for independent outcome checks.

## Anti-Patterns

- Do not write tautological tests that recreate the production route/config under test.
- Do not assert only SQL text when real SQLite behavior is cheap to test.
- Do not share mutable state between tests.
- Do not hand-edit generated mocks.
- Do not leave tests dependent on wall-clock timing unless the behavior is explicitly temporal.
