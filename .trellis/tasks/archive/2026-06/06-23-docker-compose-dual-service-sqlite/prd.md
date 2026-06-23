# [切片05] Docker Compose 双服务共享 SQLite volume

## Goal

Deliver the self-hosted deployment slice for Shiliu MVP: Docker Compose must run the HTTP API (`cmd/server`) and background scheduler (`cmd/task`) from the same backend image, with an explicit one-shot migration job (`cmd/migration`) completing before either long-running service starts, and all three processes sharing the same SQLite-backed deployment data boundary.

## Source Requirement

- GitHub issue: #6 `[切片05] Docker Compose 双服务共享 SQLite volume`
- Parent issue: #1 `拾流订阅中心 MVP 后端实现 PRD`
- Parent deployment user stories covered by this slice:
  - 81. Use Docker Compose to start `server` and `task` from the same image and share a SQLite volume.
  - 82. Run `cmd/migration` before `server` / `task` through explicit versioned migrations.
  - 83. TLS is provided by a reverse proxy or hosting platform, not by the application.
  - 84. SQLite database / volume backup is handled by deployment documentation, not an in-app backup feature.

## Confirmed Facts

- Current `deploy/docker-compose/docker-compose.yml` still starts MySQL and Redis scaffold services; it does not run Shiliu `server`, `task`, or `migration`.
- There is no repository-root `Dockerfile`; the active Docker build file is `deploy/build/Dockerfile`.
- Current `deploy/build/Dockerfile` uses `APP_RELATIVE_PATH` to build exactly one Go entrypoint into `./server`, which cannot satisfy the same-image requirement for `server`, `task`, and `migration` simultaneously.
- `cmd/server`, `cmd/task`, and `cmd/migration` all accept `-conf`; `cmd/migration` also accepts `-direction` and `-path`.
- `cmd/migration` already owns explicit `golang-migrate` execution; `cmd/server` and `cmd/task` must not run migrations implicitly.
- `config/prod.yml` currently has SQLite keys under `data.db.user` but lacks the global fetch interval and AI service placeholder keys required by issue #6.
- Runtime config can be overridden by `APP_CONF`; otherwise each command uses the `-conf` flag.
- Current SQLite DSN is `storage/nunu-test.db?_busy_timeout=5000`; Compose needs a stable in-container storage path backed by a named volume.
- `README.md` documents migration commands but not Compose deployment, SQLite volume backup, or TLS boundary.

## Requirements

1. Replace scaffold Docker Compose services with Shiliu deployment services:
   - `server`: long-running HTTP API service backed by `cmd/server`.
   - `task`: long-running scheduled task service backed by `cmd/task`.
   - `migration`: one-shot job backed by `cmd/migration`.
2. Use the same backend image for `server`, `task`, and `migration`.
3. The image must contain executable artifacts for all three command entrypoints, not rely on separate per-service images.
4. `server` and `task` must share one named SQLite data volume at the same in-container path.
5. `migration` must use the same SQLite data volume and the same production config file as `server` and `task`.
6. `migration` must run before `server` and `task` start normally.
7. `server` and `task` must not run schema migrations implicitly.
8. `config/prod.yml` must include a global fetch interval setting with allowed values `0`, `30`, `60`, `360`, and `1440` minutes; default value is `60`.
9. `config/prod.yml` must include AI service placeholder keys for OpenAI-compatible summary generation: API base URL, API key, and model name.
10. Deployment documentation must explain:
    - how the Compose services map to `server`, `task`, and `migration`;
    - where SQLite data is persisted;
    - how to back up the SQLite volume/database from the deployment layer;
    - that TLS/HTTPS is provided by a reverse proxy or platform, not by the app.
11. Remove MySQL/Redis scaffold assumptions from the Compose deployment path.
12. Preserve local source-run behavior outside Compose unless directly required by this slice.

## Acceptance Criteria

- [ ] `deploy/docker-compose/docker-compose.yml` builds or references one Shiliu backend image used by `server`, `task`, and `migration`.
- [ ] `server` runs the HTTP API command and publishes the configured HTTP port.
- [ ] `task` runs the scheduled task command without publishing an HTTP port.
- [ ] `migration` runs `cmd/migration` as a one-shot job using production config and the shared SQLite volume.
- [ ] Compose ordering makes successful migration completion a prerequisite for normal `server` and `task` startup.
- [ ] The Compose file defines a named SQLite data volume shared by all three services.
- [ ] The Compose deployment no longer defines MySQL or Redis services.
- [ ] `config/prod.yml` contains `task.fetch_interval_minutes: 60` or an equivalent single global fetch interval key whose documented allowed values are off/30/60/360/1440.
- [ ] `config/prod.yml` contains AI placeholder keys for API base URL, API key, and model name without hardcoding a real secret.
- [ ] Deployment docs explain SQLite volume/database backup and TLS responsibility boundaries.
- [ ] `docker compose -f deploy/docker-compose/docker-compose.yml build` succeeds.
- [ ] User stories 81-84 are verifiable from Compose wiring, config, and docs.

## TDD Behavior Scenarios

1. Compose model uses one image for all runtime services.
   - Given the Compose file is parsed, `server`, `task`, and `migration` refer to the same image/build output.
2. Compose model shares SQLite persistence.
   - Given the Compose file is parsed, all three services mount the same named volume to the same app storage path.
3. Compose model runs migration first.
   - Given the Compose file is parsed, `server` and `task` depend on `migration` with successful-completion semantics.
4. Compose model replaces scaffold services.
   - Given the Compose file is parsed, no MySQL or Redis services remain.
5. Production config exposes deployment knobs.
   - Given `config/prod.yml` is parsed, fetch interval defaults to `60` and AI placeholder keys exist without real secrets.
6. Documentation covers operational boundaries.
   - Given deployment docs are searched, they mention SQLite backup/volume backup and TLS via reverse proxy/platform.
7. Image build supports all three commands.
   - Given `docker compose ... build` is run, the build succeeds for the unified image.

## Out of Scope

- Implementing feed fetching, AI summary generation, OPML import, or user-facing APIs.
- Implementing in-app database backup/restore or all-data export.
- Adding TLS termination, certificate issuance, or reverse-proxy container configuration.
- Adding PostgreSQL/MySQL/Redis/Mongo services or multi-database support.
- Changing `cmd/server`, `cmd/task`, or `cmd/migration` business behavior except as needed for container command execution.
- Solving task scheduling interval consumption in code if no fetch scheduler exists yet; this slice only adds the deploy-time config key required by issue #6.

## Open Questions

None blocking. The implementation should choose the simplest contract that satisfies the bedrock requirement: one image with all three command binaries and service-specific Compose commands.
