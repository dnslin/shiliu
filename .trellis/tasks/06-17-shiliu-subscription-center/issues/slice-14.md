## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现手动刷新与同源抓取互斥 + 崩溃恢复。

端到端贯穿 handler + service + 测试：
- 手动刷新全部订阅源、手动刷新单个订阅源，复用抓取 service。
- 同一订阅源抓取互斥：已有抓取进行中（`fetch_status=fetching`）时，新的同源请求被跳过并返回"已在抓取中 / 已跳过"语义。
- 进程崩溃恢复：基于 `fetch_started_at` 识别过期 in-progress 状态，使卡住的订阅源能被后续抓取重新接管，不会永久卡在 `fetching`。
- 抓取过程更新订阅源当前 / 最后状态字段（`fetch_status` / `fetch_started_at` / `last_fetched_at` / `last_fetch_error`），不建抓取历史表。

## Acceptance criteria

- [ ] 支持手动刷新全部 与 手动刷新单个，均复用抓取 service
- [ ] 同源抓取进行中时新请求被跳过并返回"已在抓取中 / 已跳过"
- [ ] 过期 in-progress 状态（基于 `fetch_started_at`）可被恢复并重新抓取
- [ ] 抓取后正确更新当前 / 最后状态字段，无抓取历史表
- [ ] service 测试覆盖：正常刷新、同源并发跳过、过期状态恢复
- [ ] handler 用 httpexpect 断言刷新全部 / 单个的响应语义
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #13 抓取解析 service 核心管线
