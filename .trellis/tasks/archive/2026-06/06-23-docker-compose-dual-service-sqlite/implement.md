# Implementation Plan: Docker Compose dual service shared SQLite volume

## Pre-Implementation Checklist

- [x] Issue #6 read through `gh issue view 6`.
- [x] Parent issue #1 deployment stories 81-84 reviewed.
- [x] Current Compose/Docker/config/entrypoints inspected.
- [x] Relevant backend specs reviewed: database guidelines, migration guidelines, quality guidelines, cross-layer and reuse thinking guides.
- [ ] User reviews/approves planning artifacts.
- [ ] Task is started with `python ./.trellis/scripts/task.py start .trellis/tasks/06-23-docker-compose-dual-service-sqlite` before code edits.
- [ ] New git branch is created before implementation.

## Branch

Recommended branch name:

```bash
git switch -c issue-6-docker-compose-dual-service-sqlite
```

## TDD Scenario Matrix

Each scenario should be implemented as RED -> GREEN -> REFACTOR, one vertical slice at a time.

1. **Compose service roles**
   - RED: test parses Compose and expects `server`, `task`, `migration`; rejects `user-db`, `cache-redis`, `mysql`, `redis` scaffold remnants.
   - GREEN: rewrite `deploy/docker-compose/docker-compose.yml` service set.
2. **Same image contract**
   - RED: test expects `server`, `task`, and `migration` to use the same image/build identity.
   - GREEN: define a single image/build anchor or identical image field for all services.
3. **Shared SQLite volume**
   - RED: test expects all three services to mount one named volume at the same storage target.
   - GREEN: add named volume and mounts.
4. **Migration prerequisite**
   - RED: test expects `server` and `task` to depend on `migration` success, not just ordering.
   - GREEN: add Compose `depends_on` condition with `service_completed_successfully`.
5. **Correct service commands and ports**
   - RED: test expects commands to invoke `server`, `task`, and `migration`; `server` publishes HTTP port, `task` does not.
   - GREEN: add service-specific commands/ports.
6. **Production config keys**
   - RED: test parses `config/prod.yml` and expects fetch interval default `60`, AI placeholder keys, and deployment SQLite DSN under `storage/`.
   - GREEN: update `config/prod.yml`.
7. **Deployment docs**
   - RED: test or script assertion expects docs mention Compose services, SQLite volume backup, and TLS/reverse proxy/platform responsibility.
   - GREEN: update README or add `deploy/docker-compose/README.md` and link it.
8. **Unified image build**
   - RED: if feasible, test/static check expects Dockerfile build commands for `cmd/server`, `cmd/task`, `cmd/migration`.
   - GREEN: update `deploy/build/Dockerfile` to compile all three binaries and copy config/migrations.
   - Final integration: run `docker compose -f deploy/docker-compose/docker-compose.yml build`.

## Implementation Steps

1. Load pre-development Trellis guidance via `trellis-before-dev` once the task is in progress.
2. Create the implementation branch.
3. Add requirement-driven tests for Compose/config/docs.
   - Prefer a focused Go test file under a deployment-appropriate package if practical.
   - Avoid asserting whole-file snapshots; assert the public deployment contract.
4. Run the new tests and confirm RED failure is caused by current scaffold deployment state.
5. Update `deploy/docker-compose/docker-compose.yml`:
   - remove MySQL/Redis services;
   - add `migration`, `server`, `task`;
   - use one image/build contract;
   - add shared named storage volume;
   - add migration-success dependencies;
   - expose only server HTTP port.
6. Update `deploy/build/Dockerfile`:
   - use a Go version compatible with `go.mod` (`go 1.24.10`) or otherwise confirmed buildable;
   - build `cmd/server`, `cmd/task`, `cmd/migration` into separate binaries;
   - copy `config/` and `migrations/` into runtime image;
   - set a neutral default command or keep Compose commands explicit.
7. Update `config/prod.yml`:
   - set production SQLite DSN to a stable volume-backed file under `storage/`;
   - add global fetch interval key defaulting to `60`;
   - add AI service placeholder keys with empty API key.
8. Update deployment docs:
   - Compose build/up commands;
   - service role explanation;
   - SQLite volume path/name and backup example;
   - TLS boundary statement.
9. Run the new tests until GREEN.
10. Refactor only after GREEN:
    - remove duplicate YAML parsing helpers if tests introduced them;
    - keep Dockerfile/Compose readable and minimal.
11. Run full validation commands.
12. Dispatch `trellis-check` for quality verification if code/docs changed significantly.
13. Address findings, rerun affected checks.
14. Proceed to finish phase: update specs if a durable convention was learned, commit if requested by Trellis finish workflow/user.

## Validation Commands

Run from repository root unless noted.

```bash
go test ./...
go build ./...
go vet ./...
docker compose -f deploy/docker-compose/docker-compose.yml config
docker compose -f deploy/docker-compose/docker-compose.yml build
```

If Docker is unavailable or daemon access fails, capture the exact error and still run all static/config tests. Do not report Docker build as verified unless the command succeeds.

## Static Checks

Use targeted searches before completion:

```bash
# Compose deployment should not retain scaffold services
rg -n "mysql|redis|user-db|cache-redis" deploy/docker-compose config/prod.yml

# Long-running services must not import/call migration runner
rg -n "internal/migration|migration\.Run|AutoMigrate" cmd/server cmd/task internal/server internal/task

# Deployment docs should mention boundaries
rg -n "backup|volume|SQLite|TLS|reverse proxy|platform" README.md deploy/docker-compose
```

## Risky Files and Rollback Points

- `deploy/build/Dockerfile`
  - Risk: broken image build.
  - Rollback/fix: keep the multi-binary requirement; adjust Go base image/build commands rather than returning to per-binary image builds.
- `deploy/docker-compose/docker-compose.yml`
  - Risk: Compose syntax/version compatibility.
  - Rollback/fix: validate with `docker compose ... config`; keep migration-success semantics or document manual fallback.
- `config/prod.yml`
  - Risk: production filename change from scaffold `nunu-test.db` to Shiliu-specific DB.
  - Rollback/fix: make the file path clear in docs and allow config override; do not hide the SQLite file outside the mounted volume.
- Deployment docs
  - Risk: backup example suggests unsafe consistency guarantees.
  - Rollback/fix: phrase as deployment-layer backup guidance and recommend stopping services for simple file/volume backup.

## Review Gate Before `task.py start`

Planning is ready when:

- `prd.md`, `design.md`, and `implement.md` exist.
- PRD acceptance criteria are testable.
- Design identifies image, Compose, volume, config, migration, and docs boundaries.
- Implementation plan contains TDD scenarios and validation commands.
- User approves proceeding into implementation.
