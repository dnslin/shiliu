## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现标签的完整生命周期，并把 `tag_id` 过滤接入统一内容列表查询。标签用于组织内容条目。

端到端贯穿 migration + repository + service + handler + 测试：
- 迁移：`tags`（名称唯一约束）+ `content_item_tags` 多对多关联表。
- CRUD：创建（重名返回冲突）、重命名、删除。
- 删除语义：删除标签只移除标签本身及其与内容条目的关联记录，**不删除内容条目**；关联表随删除清理。
- 分配：给单个内容条目添加 / 移除多个标签，分配只接受已存在的 `tag_id`（不即时新建）。
- 过滤：把 `tag_id` 作为单值过滤维度接入 #17 统一列表查询。

## Acceptance criteria

- [ ] `tags`（名称唯一）+ `content_item_tags` 迁移建立，up/down 双向可执行
- [ ] 标签创建 / 重命名 / 删除完成；重名创建返回冲突
- [ ] 删除标签只删标签及关联、内容条目保留，关联表无残留
- [ ] 单条内容条目可分配 / 移除多个标签，分配只接受已存在 `tag_id`
- [ ] `tag_id` 过滤接入统一列表查询并保持 AND 语义
- [ ] repository 集成测试（真实 SQLite）验证名称唯一约束与删除语义
- [ ] handler 用 httpexpect 断言 CRUD / 分配 / 过滤
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #11 feeds + content_items 迁移 + repository
- #17 统一内容列表查询
