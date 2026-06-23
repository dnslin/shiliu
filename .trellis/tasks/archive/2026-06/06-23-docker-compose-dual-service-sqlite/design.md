# Design: Docker Compose dual service with shared SQLite volume

## First-Principles Reasoning

### Challenge assumptions

- Assumption: keeping the current `APP_RELATIVE_PATH` Dockerfile is enough because Compose can build multiple services. This is wrong for issue #6: building one binary per service creates separate images or separate build identities, while the requirement says the same image starts `server` and `task`.
- Assumption: SQLite can live inside each container filesystem. This is wrong for durable self-hosted deployment: container filesystems are disposable, and `server`, `task`, and `migration` must see the same database file.
- Assumption: `depends_on` with startup order alone is enough. This is incomplete: schema mutation is a one-shot precondition, so the long-running services need successful migration completion, not merely container creation order.
- Assumption: TLS or backup should be solved inside the app for completeness. This conflicts with the parent MVP boundary: TLS and whole-database backup are deployment-layer responsibilities documented for the operator.
- Assumption: adding MySQL/Redis alongside SQLite is harmless scaffold residue. This is wrong for this slice because the MVP data boundary is SQLite-only and Compose is the primary user deployment path.

### Bedrock truths

- A Docker image is an immutable filesystem plus metadata; different containers can run different commands from the same image only if that image contains all required executables.
- A SQLite database is a file; every process that must share state must read/write the same file path backed by the same persistent filesystem/volume.
- A schema migration changes durable state; long-running services should start after the migration command exits successfully.
- Docker Compose can express a shared named volume, service-specific commands, and dependency conditions for one-shot jobs.
- The existing commands already expose stable process boundaries: `cmd/server`, `cmd/task`, and `cmd/migration`.
- `config/prod.yml` is the runtime config consumed by all three production processes.

### Rebuild from truths

1. Since one image must serve three process roles, build all three Go commands into the runtime image.
2. Since the roles differ only by process command, keep a single image tag/build and set service-specific `command` values in Compose.
3. Since SQLite state is one file, configure production DSN to point under a stable in-container `storage/` path and mount the same named volume there for all three services.
4. Since schema migration is a one-shot durable mutation, model `migration` as a one-shot Compose service and make `server`/`task` depend on successful completion.
5. Since TLS and backup are external responsibilities, document exact operator actions/boundaries without adding app features or extra infrastructure containers.
6. Since this slice is deployment wiring, tests should parse and validate Compose/config/documentation behavior before relying on an expensive Docker build.

### Contrast with convention

A conventional scaffold path would keep one Dockerfile per binary or leave MySQL/Redis Compose services because the template provided them. That optimizes for preserving scaffold shape, not for the MVP's fundamental runtime contract. The essential difference is that Shiliu's deployable unit is a single SQLite-backed app image with three process roles, not a scaffold stack of unrelated infrastructure services.

### Conclusion

Use one multi-binary backend image and one Compose file with three service roles: `migration` mutates the shared SQLite volume once, then `server` and `task` run long-lived commands from the same image against the same production config and mounted storage. Document backup and TLS as deployment-layer responsibilities.

## Architecture and Boundaries

### Image boundary

Update `deploy/build/Dockerfile` so the runtime image includes all entrypoint binaries:

- `/data/app/bin/server` built from `./cmd/server`
- `/data/app/bin/task` built from `./cmd/task`
- `/data/app/bin/migration` built from `./cmd/migration`
- production config and migration SQL files copied into the runtime image

The runtime image should not need Go tooling. The final stage should remain small and run binaries directly.

Recommended shape:

```text
/data/app/
  bin/server
  bin/task
  bin/migration
  config/prod.yml
  migrations/*.sql
  storage/        # mounted named volume in Compose
```

Keep the Dockerfile simple. `APP_RELATIVE_PATH` becomes unnecessary for the unified image and should be removed or made non-essential for this deployment path.

### Compose boundary

Replace the scaffold Compose file with Shiliu services only:

- `migration`
  - uses the same image/build as the other services;
  - command runs `/data/app/bin/migration -conf config/prod.yml -direction up -path migrations` or equivalent relative to `/data/app`;
  - mounts the shared storage volume.
- `server`
  - uses the same image;
  - command runs `/data/app/bin/server -conf config/prod.yml`;
  - publishes `${SHILIU_HTTP_PORT:-8000}:8000` or an equivalent default host mapping;
  - depends on `migration` success.
- `task`
  - uses the same image;
  - command runs `/data/app/bin/task -conf config/prod.yml`;
  - does not publish an HTTP port;
  - depends on `migration` success.

The Compose file should define one named volume, e.g. `shiliu_storage`, mounted at `/data/app/storage` for all three services.

### Config boundary

Update `config/prod.yml` only with deploy-time keys required by the issue:

```yaml
task:
  fetch_interval_minutes: 60
ai:
  api_base_url: ""
  api_key: ""
  model: ""
```

Allowed fetch interval values should be documented as `0` (off), `30`, `60`, `360`, and `1440`. This slice does not have to wire scheduler behavior if the fetch scheduler is not implemented yet.

The SQLite DSN should remain under `data.db.user.dsn`, but it should be production-container friendly, e.g. `storage/shiliu.db?_busy_timeout=5000`, so it resolves inside the mounted storage directory.

### Documentation boundary

The repository currently has `README.md` with migration commands. Add deployment-facing documentation in the lowest-friction place:

- either extend `README.md` with a `Docker Compose deployment` section;
- or add `deploy/docker-compose/README.md` and link it from `README.md` if the content becomes too specific.

Documentation must include:

- build/start command using `docker compose -f deploy/docker-compose/docker-compose.yml up -d --build`;
- service roles (`migration`, `server`, `task`);
- SQLite persistence path and volume name;
- backup example using a temporary container or Docker volume copy/tar pattern;
- TLS statement: terminate HTTPS in reverse proxy/platform; app only serves HTTP.

### Startup and migration boundary

Do not add migration calls to `cmd/server` or `cmd/task`. Deployment sequencing belongs in Compose. The existing migration contract remains:

```bash
go run ./cmd/migration -conf config/prod.yml -direction up
```

In Compose, the same command is run from the built binary.

## Data Flow

```text
Docker build
  -> compile cmd/server, cmd/task, cmd/migration
  -> runtime image contains all three binaries + config + migrations

Compose up
  -> create named volume shiliu_storage
  -> migration container mounts /data/app/storage
  -> migration reads config/prod.yml data.db.user.dsn
  -> migration applies SQL files to /data/app/storage/shiliu.db
  -> server and task start after migration success
  -> server/task mount the same /data/app/storage and read/write the same SQLite file
```

## TDD Strategy

Prefer behavior tests at public file seams rather than implementation detail checks.

1. Add a deployment test that parses `deploy/docker-compose/docker-compose.yml` and asserts:
   - service set contains `server`, `task`, `migration`;
   - service set does not contain MySQL/Redis scaffold services;
   - all three services reference the same image/build identity;
   - all three mount the same named volume to the same storage target;
   - `server` and `task` depend on `migration` with successful-completion semantics;
   - `server` publishes a port and `task` does not.
2. Add a config test that parses `config/prod.yml` and asserts:
   - `task.fetch_interval_minutes` exists and defaults to `60`;
   - AI placeholder keys exist;
   - the configured SQLite DSN points under `storage/` and keeps `_busy_timeout=5000`.
3. Add a documentation test or targeted script assertion that deployment docs mention SQLite backup and TLS/reverse proxy responsibility.
4. Run the real Docker build command as final integration verification.

The tests can be implemented in Go if YAML support is already available through indirect dependencies, or as a small repository test using existing tooling. The key is requirement-driven assertions, not snapshotting the entire YAML file.

## Compatibility Notes

- Existing local development config remains unchanged unless implementation evidence shows a shared key should be added to both local and prod for consistency.
- Existing `go run ./cmd/server` and `go run ./cmd/task` workflows remain available.
- `Makefile docker` may need updating because it currently builds only `cmd/task` through `APP_RELATIVE_PATH`; preserve or replace it only if it would otherwise point users at the obsolete one-binary image contract.
- Compose `depends_on.condition: service_completed_successfully` requires a Docker Compose implementation that supports the Compose specification condition syntax. If unavailable in a user's old Compose binary, the documented fallback is to run migration manually first.

## Operational and Rollback Considerations

- Rollback point 1: Compose file rewrite. If service wiring is wrong, restore the previous Compose file, but note that previous MySQL/Redis scaffold did not satisfy MVP deployment.
- Rollback point 2: Dockerfile multi-binary build. If build fails because of Alpine/Go version mismatch, fix the build image version rather than splitting into per-service images.
- Rollback point 3: production DSN. Changing `config/prod.yml` from `storage/nunu-test.db` to a production filename affects where new deployments write data; existing deployments can keep their mounted file by overriding config or renaming the file.
- Backup documentation must not imply hot backup consistency guarantees beyond SQLite file/volume copy basics. Operators should stop services or use SQLite backup tooling for strict consistency once available.
