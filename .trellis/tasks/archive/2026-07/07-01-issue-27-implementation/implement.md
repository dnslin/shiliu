# Issue #27 自动摘要实施计划

## Branch

- 从 `main` 新增开发分支：`issue-27-auto-summary`
- 分支创建后再进入 Phase 2 实施。

## TDD Rules

- 使用垂直切片：一个行为测试失败，再实现该行为，再继续下一项。
- 测试通过公共 seam：API、service interface、repository interface、TaskServer scheduler。
- 不做只满足 happy path 的最小实现；每个切片同时补边界、错误和竞态保护。
- 不引入真实 AI API 调用；ChatCompletion 通过 stub 驱动。

## Ordered Checklist

1. Migration + repository baseline
   - RED: migration test 断言 `auto_summary_configs` up/down。
   - GREEN: 新增 `000011_auto_summary_configs.up.sql` / `.down.sql`。
   - RED: repository 测试保存、读取、upsert、默认空配置。
   - GREEN: 新增 model + `AutoSummaryConfigRepository`。

2. 自动摘要配置 service
   - RED: service 测试启用时校验 scope、记录 `enabledAt`、缺 AI 服务配置返回 `ErrAIConfigMissing`。
   - GREEN: 新增 `AutoSummaryConfigService`。
   - RED: service 测试禁用、重新启用、开启时切换 scope 会更新有效时间。
   - GREEN: 完成时间语义。

3. 配置 API
   - RED: handler 测试 GET/PUT 配置，断言 envelope、JSON 字段、错误映射。
   - GREEN: 新增 DTO、handler、router。
   - RED: server route/auth 测试保护 `/api/v1/ai/auto-summary-config`。
   - GREEN: 接入 `NewHTTPServer` 和 router deps。

4. 候选查询
   - RED: repository 测试只返回 `created_at >= enabledAt`、`status=none`、scope 内类型，排除 `success` / `failed` / `insufficient_text` / `pending`。
   - GREEN: 新增 `ListAutoSummaryCandidates`。

5. 摘要 claim 策略重构
   - RED: service 测试自动入口只允许 `none`，不会覆盖 `success`，不会重试 `failed` / `insufficient_text`。
   - GREEN: 抽出共享摘要核心，手动入口保留原行为，自动入口用 `none` claim。
   - RED: 竞态测试：候选从 `none` 变为 `success` 后自动入口返回 skipped 且不调用 ChatCompletion。
   - GREEN: repository 增加原子 claim 方法或等价实现。

6. 自动摘要运行 service
   - RED: service 集成测试关闭时不处理；启用 text 只处理 text；启用 audio 只处理 audio；all 处理两者；每条结果计数。
   - GREEN: 新增 `AutoSummaryService.RunAutoSummary` 和批次上限。
   - RED: service 测试生成失败 / 文本不足会记录状态并继续后续候选。
   - GREEN: 完成错误与继续策略。

7. Task adapter + TaskServer
   - RED: `internal/task` 测试 `AutoSummaryTask` 委托 service。
   - GREEN: 新增 task adapter。
   - RED: TaskServer 测试自动摘要 job 注册、等待首次调度、RunAll 调用、SingletonMode 不重叠、Stop 取消 in-flight。
   - GREEN: 接入 gocron job。

8. Wire and docs
   - 更新 `cmd/server/wire/wire.go`、`cmd/task/wire/wire.go` provider sets。
   - 运行 `nunu wire all` 生成 `wire_gen.go`。
   - 如项目要求 Swagger 文档更新，则运行现有 swagger 生成命令或手工保持 docs 测试通过。

9. Final validation
   - `go test ./internal/service ./internal/server ./internal/task`
   - `go test ./test/server/repository ./test/server/service ./test/server/handler`
   - `go test ./...`
   - `go build ./...`
   - `git diff --check`

## Risky Files / Rollback Points

- `internal/service/content_item.go`: 手动摘要行为不能回退；重构后原有手动摘要测试必须仍覆盖 `none` / `failed` / `success` 覆盖、`pending`、`insufficient_text`。
- `internal/repository/content_item.go`: claim 必须原子，不能读后写。
- `internal/server/task.go`: 不能破坏后台抓取已有调度、Stop 取消和不重叠行为。
- `cmd/*/wire/wire_gen.go`: 生成产物不可手改，必须用 Wire 生成。
- `migrations/`: up/down 成对，down 只回滚本切片表。

## Review Gate Before Start

- [ ] 使用者已审阅 `prd.md` / `design.md` / `implement.md` 或明确批准继续。
- [ ] 创建开发分支 `issue-27-auto-summary`。
- [ ] 加载 `trellis-before-dev` 后再改代码。
