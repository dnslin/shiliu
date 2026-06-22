## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现全局后台定时抓取，接入 Nunu `TaskServer`。

端到端贯穿 task + 配置 + 测试：
- 移除 `internal/server/task.go` 中的示例 `CheckUser` 任务。
- 在 `TaskServer` / `gocron` 中接入全局后台抓取任务，遍历所有订阅源并复用抓取 service（含同源互斥）；`internal/task` 只做调度编排，不承载抓取业务逻辑。
- 抓取间隔可配：关闭 / 30 / 60 / 360 / 1440 分钟，默认 60；"关闭"时不调度后台抓取。

## Acceptance criteria

- [ ] 示例 `CheckUser` 任务已移除
- [ ] 后台抓取任务接入 `TaskServer` / `gocron`，复用抓取 service 与同源互斥
- [ ] `internal/task` 只做调度编排，不重复实现抓取逻辑
- [ ] 抓取间隔按配置生效（关闭 / 30 / 60 / 360 / 1440 分钟，默认 60）；"关闭"时不调度
- [ ] 测试覆盖：间隔配置解析、关闭时不调度、调度触发复用 service
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #15 手动刷新 + 同源互斥 + 崩溃恢复
