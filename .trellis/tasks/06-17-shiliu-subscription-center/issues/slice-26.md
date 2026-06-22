## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现自动摘要：由使用者配置开启后，后台为符合范围的新抓取入库内容条目自动生成 AI 摘要，复用手动摘要 service。

端到端贯穿 task + service + 配置 + 测试：
- 自动摘要配置：全局开关 + 内容类型范围（文本型 / 音频型 / 两者）。
- 生效范围：只处理配置开启**之后**新抓取入库、且摘要状态为 `none` 的内容条目；不批量回填已有历史内容。
- 触发约束：不自动覆盖 `success`，不自动重试 `failed` 或 `insufficient_text`；文本不足或失败时跳过并记录原因。
- 复用 #24 的摘要 service，不分叉摘要逻辑；由 `TaskServer` / `gocron` 调度。

## Acceptance criteria

- [ ] 自动摘要支持全局开关 + 内容类型范围（text / audio / 两者）配置
- [ ] 只处理开启后新入库且状态 `none` 的内容条目，不回填历史
- [ ] 不覆盖 `success`，不自动重试 `failed` / `insufficient_text`
- [ ] 复用手动摘要 service，无分叉摘要实现
- [ ] 测试覆盖：开关关闭不处理、范围过滤、只取 `none`、不覆盖 `success`、不重试失败 / 文本不足
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #13 抓取解析 service 核心管线
- #25 OpenAI-compatible 客户端 + 手动摘要状态机
