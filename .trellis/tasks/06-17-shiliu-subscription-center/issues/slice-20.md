## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现基础搜索：在统一列表查询之上接入 FTS5 关键词检索与排序，复用同一套结构化过滤与分页。

端到端贯穿 handler + service + repository + 测试：
- 有关键词：用 FTS5 `MATCH` 检索标题 / 订阅源名称 / `available_text` / 当前 AI 摘要；按 `bm25()` 相关性排序，相关性相同按发布时间倒序。
- 无关键词：退回发布时间倒序（与内容收件箱一致）。
- 复用与内容收件箱一致的单值 AND 过滤（含 #17 已接入的过滤维度）与 `page + pageSize` 分页。

## Acceptance criteria

- [ ] 有关键词时按 `bm25()` 相关性排序，相同相关性按发布时间倒序
- [ ] 无关键词时按发布时间倒序
- [ ] 关键词匹配覆盖 标题 / 订阅源名称 / `available_text` / 当前 AI 摘要
- [ ] 搜索复用统一列表的单值 AND 过滤与 `page+pageSize` 分页
- [ ] repository 集成测试（真实 SQLite）验证相关性排序与过滤叠加
- [ ] handler 用 httpexpect 断言有 / 无关键词两种排序与分页
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #17 统一内容列表查询
- #20 FTS5 虚拟表 + 索引同步
