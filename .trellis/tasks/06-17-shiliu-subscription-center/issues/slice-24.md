## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现 OpenAI-compatible 摘要客户端与手动 AI 摘要状态机，并在摘要变更时同步 FTS 索引。

端到端贯穿 migration + service + handler + 测试：
- 迁移：内容条目摘要字段（当前摘要 Markdown、状态、生成时间、错误原因）。
- 客户端：OpenAI-compatible 非流式 Chat Completions，注入 `ChatCompletion` 接口边界，测试用 stub（成功 / 超时 / 空响应 / 文本不足）确定性驱动，**无外部 API 调用**。
- 摘要输出：固定结构化简体中文 Markdown（TL;DR / 要点 / 对开发者价值 / 原文信息），基于 `available_text`。
- 状态机：`none` / `pending` / `success` / `failed` / `insufficient_text`。手动摘要可在 `none` / `failed` / `success` 触发，成功覆盖当前摘要或错误；`pending` 返回"正在生成"；`insufficient_text` 为终态不可重试；失败记录原因且不自动重试。
- 摘要成功后同步更新 FTS 索引中的当前 AI 摘要字段（复用 #20 的同步入口）。

## Acceptance criteria

- [ ] 摘要字段迁移建立（当前摘要 / 状态 / 生成时间 / 错误原因），up/down 双向可执行
- [ ] 摘要客户端通过注入接口工作，测试以 stub 驱动、无真实 API 调用
- [ ] 手动摘要在 `none` / `failed` / `success` 可触发并覆盖；`pending` 返回正在生成
- [ ] 文本不足置为 `insufficient_text` 且不可重试；失败记录原因、不自动重试
- [ ] 摘要成功后 FTS 索引的当前 AI 摘要字段同步更新（搜索可命中新摘要）
- [ ] service 测试覆盖状态机全部转移（含 stub 的成功 / 超时 / 空响应 / 文本不足）
- [ ] handler 用 httpexpect 断言触发摘要与各状态响应
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #20 FTS5 虚拟表 + 索引同步
- #24 AI 服务配置
