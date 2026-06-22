## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现订阅源列表查询与删除订阅源级联。

端到端贯穿 handler + service + repository + 测试：
- 订阅源列表：返回所有订阅源及其当前 / 最后抓取状态（`fetch_status` / `last_fetched_at` / `last_fetch_error`）、类型、所属文件夹。
- 删除订阅源：级联删除该订阅源产生的内容条目及全部派生数据（AI 摘要、播放进度、内容标记、标签关联），不保留孤儿内容条目。
- 级联行为用**真实 SQLite + 迁移**集成测试验证（外键 / 显式删除均可，但必须保证无孤儿残留）。

## Acceptance criteria

- [ ] 订阅源列表返回当前 / 最后抓取状态、类型、所属文件夹
- [ ] 删除订阅源级联删除其内容条目、AI 摘要、播放进度、内容标记、标签关联
- [ ] 删除后查询无任何孤儿内容条目或派生数据
- [ ] repository 级联删除用真实 SQLite + 迁移集成测试验证无残留
- [ ] handler 用 httpexpect 断言列表与删除响应
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #13 抓取解析 service 核心管线
