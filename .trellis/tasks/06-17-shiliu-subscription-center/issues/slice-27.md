## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现单条内容条目的 Obsidian Markdown 导出（复制或下载）。

端到端贯穿 handler + service + 测试：
- 把单个内容条目整理为 Markdown，字段包含：标题、原文 / 原节目链接、订阅源名称、发布时间、内容类型、标签、订阅源文件夹（均作为普通 Markdown 元信息，不做 frontmatter）。
- 始终包含 `AI 摘要` 区块：`success` 写当前摘要 Markdown；`none` / `pending` / `failed` / `insufficient_text` 分别写明未生成 / 正在生成 / 生成失败 / 可用文本不足。
- 可用文本摘录：`available_text` 前 2000 字符；被截断时追加"已截断，请打开原文链接查看完整内容"提示。
- 导出不强制生成 AI 摘要，也不得因 AI 服务不可用 / 摘要失败 / 文本不足而被阻止（纯映射，不依赖 AI 服务）。

## Acceptance criteria

- [ ] 导出 Markdown 含 标题 / 链接 / 订阅源 / 发布时间 / 内容类型 / 标签 / 订阅源文件夹 元信息
- [ ] 始终含 `AI 摘要` 区块，按当前状态写摘要正文或对应状态说明
- [ ] 可用文本摘录取前 2000 字符，截断时追加截断提示
- [ ] 导出不依赖 AI 服务可用性，任意摘要状态都能成功导出
- [ ] service 测试覆盖：各摘要状态区块、截断 / 不截断、标签与文件夹元信息
- [ ] handler 用 httpexpect 断言导出响应
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #22 标签 CRUD + 分配 + 删除 + 过滤
- #23 文件夹 CRUD + 分配 + 删除置空 + 过滤
- #25 OpenAI-compatible 客户端 + 手动摘要状态机
