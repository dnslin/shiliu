## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

把数据库迁移机制从裸 Gorm `AutoMigrate` 切换为 `golang-migrate/migrate` 的显式版本化迁移。

端到端范围：
- 引入 `golang-migrate/migrate` 依赖，`go mod tidy`。
- 约定迁移目录与命名（成对 SQL 文件 `000001_xxx.up.sql` / `000001_xxx.down.sql`）。
- 重写 `cmd/migration` / `internal/server/migration.go`：移除 `AutoMigrate`，改为执行版本化迁移；可被 `server` / `task` 启动前独立执行，不在 `server` / `task` 启动时做隐式迁移。
- 提供一个最小的初始迁移（可仅含迁移元数据表 / 占位），用于验证 up / down 双向可执行；具体业务表在后续切片各自的迁移中创建。

## Acceptance criteria

- [ ] `golang-migrate/migrate` 已加入依赖，`go mod tidy` 干净
- [ ] `cmd/migration` 执行版本化迁移，不再调用 `AutoMigrate`
- [ ] `server` / `task` 启动路径不触发隐式迁移
- [ ] 迁移目录与成对 SQL 命名约定已落地并在 README/部署文档或代码注释中说明
- [ ] `migrate up` 后再 `migrate down` 可双向干净执行（基于 SQLite 文件库验证）
- [ ] `go build ./...`、`go vet ./...` 通过

## Blocked by

- #3 数据层 SQLite-only 清理
