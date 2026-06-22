## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现"添加订阅源" HTTP 端到端流程：抓取解析成功才落库，按规范化 feed URL 去重。

端到端贯穿 handler + service + 测试：
- 提交 feed URL → 复用抓取 service 先抓取并解析；解析成功才创建订阅源记录并入库首批内容条目；URL 无法抓取或解析失败时不落库，只返回错误。
- 按规范化 feed URL 判断重复：已存在则返回冲突，不创建第二条。
- 订阅源类型自动判定：含播客 RSS 语义 / 音频 enclosure 归 `podcast`，其余结构化 feed 归 `rss`。

## Acceptance criteria

- [ ] 提交有效 feed URL，抓取解析成功后创建订阅源并入库首批内容条目
- [ ] 抓取或解析失败时不创建订阅源，返回明确错误
- [ ] 重复 feed URL（规范化后）返回冲突，不创建第二条
- [ ] 订阅源类型正确判定为 `rss` / `podcast`
- [ ] handler 用 httpexpect、Fetcher 用 stub 覆盖：成功创建、解析失败不落库、重复冲突
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #13 抓取解析 service 核心管线
