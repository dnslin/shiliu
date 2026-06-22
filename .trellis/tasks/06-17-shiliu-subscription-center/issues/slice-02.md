## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

把数据层清理为 **SQLite-only**，移除 Nunu 模板默认的 MySQL / PostgreSQL / Redis / Mongo 运行假设。

端到端范围：
- `internal/repository/repository.go` 只保留基于 `glebarez/sqlite` 的 `NewDB`，移除 `NewRedis`、`NewMongo` 及 MySQL / Postgres 分支；GORM `Debug()` 不再默认开启（改为按环境 / 配置控制）。
- 同步更新 Wire provider set，移除已删除依赖的注入。
- `go.mod` 移除 redis、mysql、postgres、mongo 驱动依赖，`go mod tidy`。
- `config/local.yml`、`config/prod.yml` 移除 redis / mongo 配置段，保留 SQLite dsn 与 jwt.key。

不在本切片引入 golang-migrate（见后续切片）；本切片只做"减法"清理与可编译。

## Acceptance criteria

- [ ] `internal/repository/repository.go` 只剩 SQLite 的 `NewDB`，无 `NewRedis` / `NewMongo` / MySQL / Postgres 分支
- [ ] GORM `Debug()` 不再无条件开启
- [ ] Wire provider set 同步移除已删依赖，三处 `wire_gen.go` 重新生成
- [ ] `go.mod` / `go.sum` 不再包含 redis、mysql、postgres、mongo 驱动；`go mod tidy` 干净
- [ ] `config/local.yml`、`config/prod.yml` 不含 redis / mongo 段，保留 SQLite dsn 与 jwt.key
- [ ] `go build ./...` 与 `go vet ./...` 通过

## Blocked by

- #2 module 重命名 `shuliu` → `shiliu`
