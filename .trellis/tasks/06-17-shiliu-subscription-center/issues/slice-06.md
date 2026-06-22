## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

把 Nunu 模板的 `User` 模型重构为拾流单人用户账户的**最小鉴权模型**，并建立 users 表迁移与 repository。

端到端范围：
- `internal/model` 的 User 只保留鉴权必需字段：`id`、`username`、`password_hash`、登录失败计数、锁定到期时间、`created_at`、`updated_at`。移除 Nunu 模板的 `Nickname` / `Email` / 软删除（`DeletedAt`）字段。
- users 表 golang-migrate 成对 SQL 迁移（`username` 唯一约束）。
- UserRepository：创建、按 username 查询、更新（含失败计数 / 锁定时间 / password_hash），用**真实 SQLite + 迁移**的集成测试验证（替换原 go-sqlmock + MySQL 方言策略）。

不在本切片实现登录 / 初始化 HTTP 流程（见后续切片）。

## Acceptance criteria

- [ ] User 模型只含 `id`/`username`/`password_hash`/失败计数/锁定到期时间/`created_at`/`updated_at`
- [ ] users 表迁移建立，`username` 唯一约束，up/down 双向可执行
- [ ] UserRepository 覆盖创建 / 按 username 查询 / 更新
- [ ] repository 测试基于真实 SQLite + 迁移运行，断言唯一约束冲突行为
- [ ] 原 `test/server/repository/user_test.go` 的 sqlmock/MySQL 假设已迁移或更新，不再引用已删字段
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #5 golang-migrate 迁移机制
