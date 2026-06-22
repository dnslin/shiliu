## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

建立订阅源与内容条目的数据模型、迁移与 repository，作为抓取与内容能力的持久化基座。

端到端范围：
- `feeds` 表迁移：`feed_url`（唯一约束）、`type`（`rss` / `podcast`）、`fetch_status`（`idle`/`fetching`/`success`/`failed`）、`fetch_started_at`、`last_fetched_at`、`last_fetch_error`、`folder_id`（可空）。
- `content_items` 表迁移：`feed_id`、`dedupe_key`、`(feed_id, dedupe_key)` 唯一约束、`type`（`text`/`audio`）、原始 feed 文本字段 `title`/`description`/`content`/`show_notes`、净化后字段 `description_safe`/`content_safe`/`show_notes_safe`、`available_text`、发布时间 / 抓取时间、音频播放进度。
- FeedRepository / ContentItemRepository 基础读写，用**真实 SQLite + 迁移**集成测试验证唯一约束与外键关系。

不在本切片实现抓取 / 净化逻辑（见后续切片）。

## Acceptance criteria

- [ ] `feeds` 表迁移建立，`feed_url` 唯一约束，含抓取状态字段与可空 `folder_id`
- [ ] `content_items` 表迁移建立，`(feed_id, dedupe_key)` 唯一约束，含原始 + 净化后字段、`available_text`、音频进度
- [ ] up/down 双向可执行
- [ ] Feed / ContentItem repository 基础读写完成
- [ ] repository 集成测试基于真实 SQLite + 迁移，断言 `feed_url` 与 `(feed_id,dedupe_key)` 唯一冲突
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #5 golang-migrate 迁移机制
