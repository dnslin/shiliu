## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

在统一列表查询之上实现预设过滤视图，复用同一套查询 / 分页 / 排序 / 条目操作。

端到端贯穿 handler + service + 测试：
- 内容收件箱：预设 `processing_status=unprocessed`。
- 稍后处理：预设 `mark=later`。
- 收藏：预设 `mark=favorite`。
- 已完成：预设 `processing_status=completed`。
- 订阅源详情：预设 `feed_id=X`。
- 每个视图允许在预设值之上追加其他单值过滤（AND）。视图只是预设过滤，不引入独立列表模型或专属工作流。

## Acceptance criteria

- [ ] 收件箱 / 稍后 / 收藏 / 已完成 / 订阅源详情 五个视图均基于统一列表查询的预设过滤实现
- [ ] 各视图可在预设值之上追加其他单值过滤并保持 AND 语义
- [ ] 各视图复用同一分页与排序行为
- [ ] 无为任一视图分叉的独立列表查询实现
- [ ] handler 用 httpexpect 断言各视图的预设过滤结果
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #17 统一内容列表查询
