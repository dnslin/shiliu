# Nunu — A CLI tool for building Go applications.

Nunu is a scaffolding tool for building Go applications. Its name comes from a game character in League of Legends, a little boy riding on the shoulders of a Yeti. Just like Nunu, this project stands on the shoulders of giants, as it is built upon a combination of popular libraries from the Go ecosystem. This combination allows you to quickly build efficient and reliable applications.

[简体中文介绍](https://github.com/go-nunu/nunu/blob/main/README_zh.md)

![Nunu](https://github.com/go-nunu/nunu/blob/main/.github/assets/banner.png)

## Documentation
* [User Guide](https://github.com/go-nunu/nunu/blob/main/docs/en/guide.md)
* [Architecture](https://github.com/go-nunu/nunu/blob/main/docs/en/architecture.md)
* [Getting Started Tutorial](https://github.com/go-nunu/nunu/blob/main/docs/en/tutorial.md)
* [Unit Testing](https://github.com/go-nunu/nunu/blob/main/docs/en/unit_testing.md)


## Database migrations

Schema changes are versioned with `golang-migrate` SQL files under `migrations/`.
Each migration must have a paired six-digit filename:

```text
000001_description.up.sql
000001_description.down.sql
```

Run migrations explicitly before starting `cmd/server` or `cmd/task`:

```bash
go run ./cmd/migration -conf config/local.yml -direction up
go run ./cmd/migration -conf config/local.yml -direction down
```

`-direction` defaults to `up`; `down` rolls back one migration version boundary.
`-path` defaults to `migrations`; relative paths are resolved from the command's current working directory and then converted to an absolute file source URL.
The configured `data.db.user.driver` must be empty or `sqlite`; any other value fails before migration starts.
Quote YAML DSNs that contain `#` or other comment-sensitive characters so config parsing preserves the full SQLite filename.
Long-running server and task processes do not run schema migrations implicitly.

## Docker Compose deployment

The primary self-hosted deployment path is Docker Compose:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml up -d --build
```

The Compose stack builds one `shiliu-backend:local` image containing all three Go entrypoints, then runs three process roles from that same image:

- `migration` runs `./bin/migration -conf config/prod.yml -direction up -path migrations` as a one-shot job.
- `server` runs `./bin/server -conf config/prod.yml` and publishes HTTP on `${SHILIU_HTTP_PORT:-8000}`.
- `task` runs `./bin/task -conf config/prod.yml` for scheduled background work and does not publish an HTTP port.

`server` and `task` wait for `migration` to complete successfully before starting. If an older Compose implementation does not support `service_completed_successfully`, run the migration command manually before starting the long-running services.

### SQLite storage and backup

All Compose services mount the named Docker volume `shiliu_storage` at `/data/app/storage`. The production SQLite DSN is `storage/shiliu.db?_busy_timeout=5000`, so the database file lives inside that shared volume.

The application does not provide in-app whole-database backup or restore in the MVP. Back up the SQLite database/volume at the deployment layer. For a simple file/volume backup, stop the long-running services first so SQLite is not actively writing:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml stop server task
docker run --rm -v shiliu_storage:/data -v "$PWD":/backup alpine \
  tar czf /backup/shiliu-sqlite-volume.tgz -C /data .
docker compose -f deploy/docker-compose/docker-compose.yml up -d
```

For stricter online-backup guarantees, use SQLite backup tooling or a platform snapshot mechanism that preserves file consistency.

### TLS boundary

Shiliu serves plain HTTP from the `server` container. TLS/HTTPS termination, certificate issuance, and renewal are deployment responsibilities: put Shiliu behind your own reverse proxy or a platform load balancer/CDN that provides TLS.

## License

Nunu is released under the MIT License. For more information, see the [LICENSE](LICENSE) file.
