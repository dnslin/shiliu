## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现文件夹的完整生命周期，并把 `folder_id` 过滤接入统一内容列表查询。文件夹用于组织订阅源。

端到端贯穿 migration + repository + service + handler + 测试：
- 迁移：`folders`（名称唯一约束）；订阅源 `feeds.folder_id` 关系已在 #11 建立。
- CRUD：创建（重名返回冲突）、重命名、删除。
- 删除语义：删除文件夹只移除分组本身，并把原属于该文件夹的订阅源 `folder_id` 置空，**不删除订阅源或其内容条目**；保证无残留指向已删文件夹的 `folder_id`。
- 分配：给单个订阅源选择零个或一个文件夹，分配只接受已存在的 `folder_id` 或置空（不即时新建）。
- 过滤：把 `folder_id` 作为单值过滤维度接入 #17 统一列表查询（按订阅源所属文件夹过滤内容条目）。

## Acceptance criteria

- [ ] `folders`（名称唯一）迁移建立，up/down 双向可执行
- [ ] 文件夹创建 / 重命名 / 删除完成；重名创建返回冲突
- [ ] 删除文件夹只删分组本身，原属订阅源 `folder_id` 置空、订阅源与内容条目保留
- [ ] 删除后无残留指向已删文件夹的 `folder_id`
- [ ] 单个订阅源可分配 / 置空文件夹，分配只接受已存在 `folder_id`
- [ ] `folder_id` 过滤接入统一列表查询并保持 AND 语义
- [ ] repository 集成测试（真实 SQLite）验证名称唯一约束与删除置空语义
- [ ] handler 用 httpexpect 断言 CRUD / 分配 / 过滤
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #11 feeds + content_items 迁移 + repository
- #17 统一内容列表查询
