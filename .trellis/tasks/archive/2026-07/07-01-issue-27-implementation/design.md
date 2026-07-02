# Issue #27 自动摘要技术设计

## Architecture

本切片跨越 migration、model、repository、service、handler、router、task、TaskServer、Wire 和测试。实现保持现有 Nunu 分层：

- `api/v1`: 自动摘要配置请求 / 响应 DTO。
- `internal/model`: 自动摘要配置 singleton model 和内容类型范围枚举。
- `internal/repository`: 自动摘要配置 repository；内容条目候选查询；摘要 claim 原子状态转移。
- `internal/service`: 自动摘要配置 service；自动摘要运行 service；手动/自动摘要共享的摘要生成核心。
- `internal/handler` + `internal/router`: 受鉴权保护的配置 API。
- `internal/task`: 调度编排适配层，只委托 service。
- `internal/server/task.go`: gocron 注册自动摘要 job，保持调度、单例和取消语义。
- `cmd/*/wire`: 注册新 provider 并重新生成。

## Data Model

新增 `auto_summary_configs` singleton 表：

- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `singleton_id INTEGER NOT NULL DEFAULT 1 CHECK (singleton_id = 1)`
- `enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1))`
- `content_type_scope TEXT NOT NULL DEFAULT 'all' CHECK (content_type_scope IN ('text', 'audio', 'all'))`
- `enabled_at DATETIME NULL`
- `created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`
- unique index on `singleton_id`

`enabled_at` 表示当前有效自动摘要配置的开启时间。启用时如果此前为关闭，或开启状态下切换范围，写入当前时间；关闭时清空 `enabled_at`。这样切换范围不会把此前不在当前有效配置下的内容当作候选。

不在 `content_items` 增加 eligibility 标志。本切片用 `created_at >= enabled_at` 作为“配置开启后新抓取入库”的持久化边界，避免把抓取 service 强绑定到自动摘要配置。

## API Contract

新增受 JWT 保护的 API：

- `GET /api/v1/ai/auto-summary-config`
- `PUT /api/v1/ai/auto-summary-config`

请求：

```json
{
  "enabled": true,
  "contentTypeScope": "all"
}
```

响应 `data`：

```json
{
  "enabled": true,
  "contentTypeScope": "all",
  "enabledAt": "2026-07-01T09:00:00Z"
}
```

未保存过配置时 GET 返回 `{enabled:false, contentTypeScope:"all", enabledAt:null}`。启用时如果 AI 服务配置不存在，返回现有 `ErrAIConfigMissing`；不做连通性测试。

## Candidate Selection

新增 repository 查询：

- 输入：`enabledAt`、内容类型集合、batch limit。
- 条件：
  - `created_at >= enabledAt`
  - `ai_summary_status = 'none'`
  - `type IN (...)`
- 排序：`created_at ASC, id ASC`
- limit：固定批次上限，避免单次任务无界消耗 API 调用。

该查询只负责找候选；真正状态变更仍由摘要 service 的 claim 控制，防止候选列表与执行之间发生竞态。

## Summary Generation Reuse

现有 `GenerateAISummary` 是手动行为，允许 `none` / `failed` / `success` 触发。自动摘要需要更窄的触发策略：

- 抽出共享内部核心：读取内容条目、检查文本长度、读取 AI 配置、调用 ChatCompletion、记录 `pending` / `success` / `failed` / `insufficient_text`、同步 FTS。
- 手动入口使用 claim 策略：`none` / `failed` / `success -> pending`。
- 自动入口使用 claim 策略：仅 `none -> pending`。

这样自动摘要复用同一个摘要核心，但不会因为手动摘要竞态而覆盖已经变为 `success` 的条目。

## Scheduled Work

新增 `AutoSummaryTask`，只做 service 委托。

`TaskServer` 注册第二个 gocron job：

- job name: `auto-summary`
- 固定较短轮询间隔，由自动摘要配置的 `enabled` 控制是否实际处理。
- `WaitForSchedule()`：启动后等待首次调度，不立即消耗 API。
- `SingletonMode()`：同一 job 不重叠。
- `Stop()`：取消 in-flight 自动摘要任务，与后台抓取使用同一个 scheduler context。

后台抓取配置关闭时，不影响自动摘要 job；否则手动抓取产生的新内容无法被自动摘要扫描到。

## Error Handling

- 配置请求参数非法：`400 ErrBadRequest`。
- 启用自动摘要但 AI 服务配置缺失：`404 ErrAIConfigMissing`。
- 自动摘要运行时配置关闭：返回 disabled result，无错误。
- 自动摘要运行时 AI 配置缺失：返回错误，由 TaskServer 记录；不改变候选内容条目状态。
- 自动摘要生成失败：复用摘要 service，将条目标记为 `failed` 并记录公开失败原因；任务继续处理后续候选。
- 文本不足：复用摘要 service，将条目标记为 `insufficient_text`；任务继续。
- 竞态下候选不再是 `none`：自动入口返回 skip，不覆盖状态。
- `context.Canceled` / `context.DeadlineExceeded`：停止本轮任务并向上返回。

## Testing Strategy

遵循 TDD 垂直切片，测试公共行为，不测私有实现：

- Migration / repository：真实 SQLite + checked-in migrations。
- Service：优先真实 SQLite seam；ChatCompletion 使用 stub，无外部 API。
- Handler：httpexpect 走真实 Gin handler。
- TaskServer：直接构造 scheduler，断言 job、单例、不立即运行、Stop 取消。
- Wire / build：生成 Wire 后跑 build/test。

## Tradeoffs

- 使用 `created_at >= enabled_at` 而不是新增内容 eligibility 列，减少抓取管线耦合；代价是范围变更语义由 `enabled_at` 重置来表达。
- 自动摘要固定轮询而不是加用户可配 interval，避免超出“全局开关 + 内容类型范围”的 MVP 配置边界。
- 自动入口新增窄 claim 策略而不是直接调用手动入口，是为了满足“不覆盖 success / 不重试 failed 或 insufficient_text”的竞态安全要求。
