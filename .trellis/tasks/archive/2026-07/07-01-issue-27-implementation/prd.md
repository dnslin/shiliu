# Issue #27 自动摘要

## Goal

实现自动摘要能力：使用者开启配置并选择内容类型范围后，拾流由后台任务为符合条件的新抓取入库内容条目自动生成 AI 摘要，复用现有手动 AI 摘要 service 和状态机。

## Source Requirements

- GitHub Issue #27: `[切片26] 自动摘要（仅 none / 仅新内容 / 类型范围）`
- Parent PRD: GitHub Issue #1 `拾流订阅中心 MVP 后端实现 PRD`
- Project background: root `CONTEXT.md`
- Product planning: `.trellis/tasks/06-17-shiliu-subscription-center/prd.md`
- Technical plan: `.trellis/tasks/06-17-shiliu-subscription-center/design.md` and `implement.md`

## Confirmed Facts

- 依赖 Issue #13、#25、#26 均已关闭，`main` 已包含抓取 service、手动 AI 摘要 service、AI 服务配置、FTS 同步和 TaskServer 后台抓取。
- 当前手动摘要入口 `ContentItemService.GenerateAISummary` 支持 `none` / `failed` / `success` 手动触发，`pending` 返回正在生成，`insufficient_text` 终态不可重试。
- 当前 TaskServer 使用 `gocron`，已有后台抓取任务、配置解析、单例不重叠、Stop 取消 in-flight 任务的测试。
- 自动摘要配置必须只包含全局开关和内容类型范围，不引入按订阅源、文件夹、标签、关键词、费用配额或复杂规则。
- 自动摘要必须只处理配置开启后新抓取入库的内容条目，不批量回填历史内容。
- 自动摘要必须只处理摘要状态为 `none` 的内容条目，不覆盖 `success`，不自动重试 `failed` 或 `insufficient_text`。
- 手动 AI 摘要必须始终可用，且不受自动摘要开关影响。

## Requirements

- 提供自动摘要配置持久化能力，配置包含：
  - `enabled`: 全局开关。
  - `contentTypeScope`: `text` / `audio` / `all`。
  - `enabledAt`: 当前有效配置开启时间，用于限定只处理开启后的新内容。
- 提供受鉴权保护的后端 API 读写自动摘要配置，遵守现有 `/api/v1`、`{code,message,data}` 和错误映射约定。
- 启用自动摘要前必须存在 AI 服务配置字段；不要求最近一次测试配置成功。
- 禁用自动摘要时后台任务必须无副作用，不生成摘要、不改变内容条目摘要状态。
- 自动摘要候选必须同时满足：
  - `content_items.created_at >= enabledAt`。
  - `content_items.ai_summary_status = 'none'`。
  - `content_items.type` 落在配置范围内。
- 自动摘要必须通过 TaskServer / gocron 调度运行，且运行之间不重叠。
- 自动摘要必须复用现有摘要生成核心逻辑、OpenAI-compatible 客户端、AI 服务配置、文本不足判断、失败记录和 FTS 同步。
- 自动摘要必须使用只允许 `none -> pending` 的原子 claim，避免与手动摘要竞态时覆盖已经变为 `success` / `failed` / `insufficient_text` 的内容条目。
- 自动摘要运行应有批次上限，避免单次任务无界消耗 API 调用。
- 测试必须按 TDD 垂直切片推进，不允许只做最小路径；每个行为通过公共接口或真实 SQLite seam 验证。

## Out of Scope

- 历史内容批量摘要 / 回填。
- 按订阅源、文件夹、标签、关键词或复杂规则选择自动摘要范围。
- 自动重试 `failed` / `insufficient_text`。
- 自动覆盖已有 `success` 摘要。
- 摘要任务队列、进度页面、费用配额、并发池或多节点调度。
- 前端 UI 实现。

## Acceptance Criteria

- [ ] 自动摘要配置支持全局开关和内容类型范围 `text` / `audio` / `all`。
- [ ] 配置 API 受 JWT 鉴权保护，读写响应不泄露 AI API Key，错误映射符合现有 handler 约定。
- [ ] 启用配置记录当前有效开启时间；禁用后重新启用或开启时切换范围不会回填此前不在当前有效配置下的新内容。
- [ ] 自动摘要只选择配置开启后新入库、状态为 `none`、类型在范围内的内容条目。
- [ ] 自动摘要不会覆盖 `success`，不会自动重试 `failed` 或 `insufficient_text`。
- [ ] 自动摘要复用现有手动摘要核心逻辑和 AI service，不分叉摘要实现。
- [ ] TaskServer / gocron 调度自动摘要任务，任务运行不重叠，Stop 会取消 in-flight 自动摘要。
- [ ] 测试覆盖：关闭不处理、范围过滤、只取 `none`、不覆盖 `success`、不重试 `failed` / `insufficient_text`、竞态下只允许 `none` claim、配置 API、TaskServer 调度。
- [ ] 迁移 up/down 可执行，repository 测试使用真实 SQLite。
- [ ] `go build ./...`、`go test ./...` 通过。
