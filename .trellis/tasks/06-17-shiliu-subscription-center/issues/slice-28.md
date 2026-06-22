## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现 OPML 一次性批量导入订阅源。

端到端贯穿 handler + service + 测试：
- 上传或粘贴 OPML，解析只读取 feed URL，忽略原 OPML 文件夹 / 分组层级（不自动创建拾流文件夹、不自动分配文件夹）。
- 逐个复用抓取 service：每个订阅源必须先抓取并解析成功才创建；失败项计入失败数，不创建空订阅源。
- 按规范化 feed URL 统计重复（已存在的计入重复数，不重复创建）。
- 返回成功 / 重复 / 失败数量；新增订阅源保存最近 50 条历史内容（复用抓取管线的首次 50 条逻辑）。

## Acceptance criteria

- [ ] 解析 OPML 只读 feed URL，忽略文件夹 / 分组层级
- [ ] 每个订阅源抓取解析成功才创建，失败项计入失败数且不落库
- [ ] 按规范化 feed URL 统计重复，不重复创建
- [ ] 返回成功 / 重复 / 失败数量
- [ ] 新增订阅源复用首次最多 50 条历史内容逻辑
- [ ] service 测试用 Fetcher stub + 样本 OPML 覆盖：全成功、部分失败、含重复、混合统计
- [ ] handler 用 httpexpect 断言导入结果计数
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #13 抓取解析 service 核心管线
