## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现统一内容列表查询入口，承载后续所有列表视图（收件箱 / 稍后 / 收藏 / 已完成 / 订阅源详情 / 搜索）。本切片只做查询骨架与基础单值过滤，不含预设视图与搜索。

端到端贯穿 handler + service + repository + 测试：
- 单一查询入口：单值过滤 `content_type` / `processing_status` / `mark` / `feed_id`（标签 / 文件夹过滤在后续切片接入），未传表示不过滤，多个条件之间 AND。
- 分页 `page + pageSize`（复用 #4 的辅助），换算为 SQL `limit + offset`。
- 默认排序：内容条目发布时间倒序，缺失或异常时回退抓取时间。
- 内容条目详情查询（单条）。

**禁止为后续各视图分叉列表查询**——它们都复用此入口。

## Acceptance criteria

- [ ] 单一列表查询支持 `content_type` / `processing_status` / `mark` / `feed_id` 单值 AND 过滤
- [ ] 分页 `page+pageSize`，响应含 `items` 与 `page:{page,pageSize,total}`
- [ ] 默认按发布时间倒序，缺失 / 异常回退抓取时间
- [ ] 提供内容条目详情查询
- [ ] repository 集成测试（真实 SQLite）覆盖过滤组合与排序回退
- [ ] handler 用 httpexpect 断言分页与过滤响应
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #11 feeds + content_items 迁移 + repository
