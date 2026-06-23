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

## License

Nunu is released under the MIT License. For more information, see the [LICENSE](LICENSE) file.