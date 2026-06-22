## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现订阅源抓取解析 service 的核心管线，被后续 handler 与 task 复用。本切片只做"抓取一个 feed 并把内容条目入库"的纯能力，不含 HTTP 入口。

端到端范围（service 层）：
- 注入 `Fetcher` / `http.Client` 接口边界，测试用 fixture feed（RSS / podcast / 含恶意 HTML 样本）驱动，**无网络**。
- feed URL 规范化：trim、去 fragment、scheme/host 小写、默认端口归一。
- 抓取解析后、入库前做 HTML 净化（复用净化单例），原始 + 净化后**双存**；从净化后内容生成 `available_text`。
- `dedupe_key` 生成优先级：feed item `guid` → `link` → `title + published_at` 稳定哈希。
- insert-only：发现同 `(feed_id, dedupe_key)` 已存在则跳过，不更新已有条目任何字段。
- 首次抓取每个订阅源最多保存最近 50 条。

## Acceptance criteria

- [ ] 抓取 service 通过注入的 Fetcher 接口工作，测试以 fixture feed 驱动、无真实网络
- [ ] feed URL 规范化按 trim / 去 fragment / scheme-host 小写 / 默认端口归一 实现
- [ ] 入库前净化并双存原始 + 净化后字段，`available_text` 从净化后内容生成
- [ ] `dedupe_key` 按 `guid`→`link`→`title+published_at` 优先级生成
- [ ] 重复抓取同一 feed 时已存在条目被跳过（insert-only），不更新任何字段
- [ ] 首次抓取每源最多入库 50 条
- [ ] service 测试覆盖：首次抓取截断 50、二次抓取仅新增、恶意 HTML 被净化、dedupe 各优先级
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #11 feeds + content_items 迁移 + repository
- #12 HTML 净化单例 + available_text
