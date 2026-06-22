## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

建立 FTS5 全文索引虚拟表与入库 / 变更时的索引同步，作为基础搜索的持久化基座。

端到端范围（migration + repository + 测试）：
- FTS5 虚拟表迁移：索引内容条目标题、订阅源名称、`available_text`、当前 AI 摘要 Markdown。
- 索引同步：内容条目入库时写入索引；标题 / `available_text`、订阅源名称、当前 AI 摘要变更时保持索引一致（摘要写入的同步在 AI 摘要切片接入，本切片预留同步入口）。
- 基于已通过的 S0（`glebarez/sqlite` / `modernc.org/sqlite` 支持 FTS5 + bm25），用**真实 SQLite + 迁移**集成测试验证 `MATCH` 命中与中文字段。

## Acceptance criteria

- [ ] FTS5 虚拟表迁移建立，索引 标题 / 订阅源名称 / `available_text` / 当前 AI 摘要
- [ ] up/down 双向可执行
- [ ] 内容条目入库时写入 FTS 索引；提供相关字段变更时的索引同步入口
- [ ] 集成测试（真实 SQLite）验证 `MATCH` 命中、中文字段可检索
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #11 feeds + content_items 迁移 + repository
- #13 抓取解析 service 核心管线
