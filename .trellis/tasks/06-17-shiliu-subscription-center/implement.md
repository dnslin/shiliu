# 拾流订阅中心后端实现计划

本文是复杂任务的执行计划，承载有序实现清单、验证命令和回滚点。需求边界以 `prd.md` 为准，技术边界以 `design.md` 为准；本文不重复产品决策，只编排落地顺序与验证。

## Preconditions

- 当前仓库是 Nunu 脚手架，Go module 名为 `shuliu`，代码内引用统一为 `shuliu/...`。
- 现状与 design 存在已知落差，必须在功能开发前清理（见 Phase 0）：
  - 响应结构 `{code,message,data}` 按决定沿用 Nunu 模板，不改写（仅扩展错误码注册）。
  - HTTP 路由挂在 `/v1`，不是 `/api/v1`。
  - `cmd/migration` 使用 `db.AutoMigrate`，未引入 `golang-migrate/migrate`。
  - `internal/repository/repository.go` 仍含 MySQL / Postgres / Redis / Mongo 分支与 `NewRedis` / `NewMongo`。
  - `go.mod` 仍依赖 redis、mysql、postgres、mongo 驱动；缺 `golang-migrate/migrate`。
  - `internal/model/user.go` 含 `Nickname` / `Email` / 软删除，与最小鉴权字段冲突。
  - `deploy/docker-compose/docker-compose.yml` 是 MySQL + Redis，不是 server + task 共享 SQLite volume。
  - `config/*.yml` 含 redis / mongo 配置，缺抓取间隔与 AI 服务相关键。
- SQLite 驱动为 `glebarez/sqlite`（基于 `modernc.org/sqlite`，纯 Go）。

## Blocking Spike（必须最先完成）

- [x] **S0 验证 FTS5 可用性**：已在 `glebarez/sqlite v1.11.0`（`modernc.org/sqlite v1.40.1`，纯 Go、go1.24.10）上实测通过。
  - 验证内容：`CREATE VIRTUAL TABLE ... USING fts5(...)` 建表、`INSERT`、`MATCH` 查询、`bm25()` 相关性排序均工作；含中文字段插入正常（FTS5 默认 unicode61 tokenizer）。
  - 结论：搜索按 `design.md` 的 `Search Model` 落地，无需切换 CGO `mattn/go-sqlite3`，无需降级 `LIKE`。
  - 备注：spike 为一次性验证（已删除），Phase 4 落地后由 repository 层真实 SQLite 集成测试长期守护 FTS5 行为。

## Ordered Implementation Checklist

> 每个 checklist 项对应一个已发布的 GitHub issue（切片）；issue 编号 = 切片号 + 1。父 issue：#1。
> 依赖序与 Blocked-by 以各 issue 正文为准；下表括注的依赖供快速参考。

### Phase 0 — 脚手架清理与基础设施对齐
- [ ] **#2 切片01**：重命名 Go module `shuliu` → `shiliu`：改 `go.mod` module 行，全仓替换 import 路径 `shuliu/...` → `shiliu/...`，重生成 `wire_gen.go`，`go build ./...` 验证。作为独立提交，先于其他 Phase 0 改动。（无依赖）
- [ ] **#3 切片02**：清理 `repository.go`：只保留 SQLite 的 `NewDB`，移除 `NewRedis` / `NewMongo` 及 MySQL / Postgres 分支；关闭 GORM `Debug()` 默认开启或改为按 env；同步清理 `go.mod`（移除 redis、mysql、postgres、mongo 驱动）与 `config/*.yml`（移除 redis / mongo），`go mod tidy`。（依赖 #2）
- [ ] **#4 切片03**：保留 Nunu `{code,message,data}` 响应结构（`api/v1/v1.go`）；仅在 `errorCodeMap` 注册拾流业务错误码，并提供分页 `data.items` + `data.page{page,pageSize,total}` 辅助结构（`page` 从 1 开始，handler 内换算为 `limit + offset`）；路由前缀 `/v1` → `/api/v1`（`internal/server/http.go`）。（依赖 #2）
- [ ] **#5 切片04**：引入 `golang-migrate/migrate`，重写 `cmd/migration` / `internal/server/migration.go`，移除 `AutoMigrate`，约定成对 SQL 文件目录与命名，最小初始迁移 up/down 双向可执行。（依赖 #3，详见 Phase 1）
- [ ] **#6 切片05**：重写 `docker-compose.yml`：同一镜像启动 `server`（`cmd/server`）与 `task`（`cmd/task`），共享 SQLite 数据 volume；`migration`（`cmd/migration`）作为一次性前置 job；`config/prod.yml` 新增抓取间隔与 AI 服务配置键；补部署文档（备份 volume、TLS 由反代/平台提供）。（依赖 #3、#5）

### Phase 1 — 迁移机制（= #5 切片04，与 Phase 0 重叠展开）
- [ ] **#5 切片04**：引入 `golang-migrate/migrate`，`go mod tidy`；重写 `cmd/migration` / `internal/server/migration.go` 执行成对 SQL 文件（`migrations/000001_xxx.up.sql` / `.down.sql`），移除 `AutoMigrate`；约定迁移目录与命名，确保 `server` / `task` 启动前可独立执行。

### Phase 2 — 单人用户账户与鉴权
- [ ] **#7 切片06**：重构 `User` 模型为最小鉴权字段：`id`、`username`、`password_hash`、登录失败计数、锁定到期时间、`created_at`、`updated_at`（去掉 `Nickname` / `Email` / 软删除）；迁移 users 表；repository 用真实 SQLite 测试。（依赖 #5）
- [ ] **#8 切片07**：首次初始化：创建唯一用户账户后永久关闭入口。（依赖 #7、#4）
- [ ] **#9 切片08**：登录（bcrypt cost 12，密码最少 12 字符；签发 30 天 JWT；`Authorization: Bearer`）+ 登录保护（连续 5 次失败锁定 15 分钟，成功清零）+ JWT 中间件保护业务路由。（依赖 #7、#4）
- [ ] **#10 切片09**：已登录改密码。（依赖 #9）

### Phase 3 — 订阅源与内容模型 + 抓取 service
- [ ] **#11 切片10**：迁移 `feeds` 表（`feed_url` 唯一、`type` rss/podcast、`fetch_status` / `fetch_started_at` / `last_fetched_at` / `last_fetch_error`、`folder_id`）+ `content_items` 表（`feed_id`、`dedupe_key`，`(feed_id,dedupe_key)` 唯一，`type` text/audio，原始 + 净化后字段 `description_safe`/`content_safe`/`show_notes_safe`，`available_text`，发布/抓取时间，音频进度）；repository 真实 SQLite 测试唯一约束。（依赖 #5）
- [ ] **#12 切片11**：引入 `bluemonday`；建 `pkg` 级单例白名单策略（`UGCPolicy` 适度收紧），禁止按来源适配；`available_text` 去标签生成（纯函数单测）。（依赖 #2）
- [ ] **#13 切片12**：service 层抓取能力核心管线（被 handler 与 task 共享，Fetcher 注入）：feed URL 规范化、抓取解析、入库前 HTML 净化（双存原始 + 净化后）、`available_text` 生成、`dedupe_key` 生成、insert-only、首次最多 50 条。（依赖 #11、#12）
- [ ] **#14 切片13**：添加订阅源（抓取解析成功才落库 + feed URL 去重 + 类型判定）。（依赖 #13）
- [ ] **#15 切片14**：手动刷新全部 / 单个 + 同源互斥 + 崩溃恢复。（依赖 #13）
- [ ] **#16 切片15**：删除订阅源级联删派生数据 + 订阅源列表（含抓取状态）。（依赖 #13）

### Phase 4 — 内容列表 / 过滤 / 搜索（依赖 S0）
- [ ] **#17 切片16**：统一内容列表查询：单值过滤 `content_type` / `processing_status` / `mark` / `feed_id`（tag_id/folder_id 由 #22/#23 接入），条件间 AND，`page + pageSize` 分页（handler 换算为 `limit+offset`）+ 条目详情。（依赖 #11）
- [ ] **#18 切片17**：预设视图：内容收件箱（`unprocessed`）、稍后处理（`later`）、收藏（`favorite`）、已完成（`completed`）、订阅源详情（`feed_id`）。（依赖 #17）
- [ ] **#19 切片18**：处理状态切换、稍后处理 / 收藏标记、音频播放进度持久化。（依赖 #17）
- [ ] **#20 切片19**：FTS5 虚拟表迁移 + 入库索引同步（索引标题、订阅源名称、`available_text`、当前 AI 摘要 Markdown）。（依赖 #11、#13）
- [ ] **#21 切片20**：基础搜索（`MATCH` + `bm25` 相关性排序：有关键词按相关性、相同按发布时间倒序，无关键词按发布时间倒序；复用统一列表过滤与分页）。（依赖 #17、#20）

### Phase 5 — 标签与文件夹
- [ ] **#22 切片21**：迁移 `tags`（名称唯一）+ `content_item_tags` 关联；CRUD + 单条分配（只接受已存在 id）+ 删除语义（只删标签及关联，不删内容条目）+ `tag_id` 过滤接入统一列表。（依赖 #11、#17）
- [ ] **#23 切片22**：迁移 `folders`（名称唯一）+ `feeds.folder_id`；CRUD + 单条分配/置空（只接受已存在 id）+ 删除语义（只删分组并置空订阅源 `folder_id`，不删订阅源/内容）+ `folder_id` 过滤接入统一列表。（依赖 #11、#17）

### Phase 6 — AI 摘要与服务配置
- [ ] **#24 切片23**：迁移 AI 服务配置（base url、model、server-only api key）；保存只格式校验 + 可选“测试配置”动作；api key 不回显、不入日志 / 导出 / 搜索 / 错误。（依赖 #5、#4）
- [ ] **#25 切片24**：迁移摘要字段（当前摘要 Markdown、状态、生成时间、错误原因）；OpenAI-compatible 非流式 Chat Completions 客户端（ChatCompletion 注入）；手动摘要状态机 `none`/`pending`/`success`/`failed`/`insufficient_text` 与触发覆盖规则；摘要成功后同步 FTS 索引。（依赖 #20、#24）

### Phase 7 — Obsidian 导出
- [ ] **#28 切片27**：单条内容条目 Markdown：标题、链接、订阅源、发布时间、内容类型、标签、订阅源文件夹、`AI 摘要` 区块（按状态）、`available_text` 前 2000 字符摘录 + 截断提示；不依赖 AI 服务可用性。（依赖 #22、#23、#25）

### Phase 8 — OPML 导入
- [ ] **#29 切片28**：解析 OPML 只读 feed URL，忽略文件夹层级；逐个复用抓取 service 成功才创建；按规范化 URL 统计重复；返回成功 / 重复 / 失败数量。（依赖 #13）

### Phase 9 — 定时任务
- [ ] **#26 切片25**：`TaskServer` 移除示例 `CheckUser`；接入全局后台抓取（间隔可配：关闭 / 30 / 60 / 360 / 1440 分钟，默认 60），复用 service。（依赖 #15）
- [ ] **#27 切片26**：自动摘要（仅处理 `none`、仅开启后新抓取入库、内容类型范围；不覆盖 `success`、不自动重试 `failed`/`insufficient_text`），复用 #25 摘要 service。（依赖 #13、#25）

### Phase 10 — 测试与交付
- [ ] 按需求驱动补测试：鉴权 / 首次初始化 / 抓取去重 insert-only / 删除级联 / 过滤与搜索排序 / 摘要状态机 / 导出截断 / OPML 计数。（贯穿各切片验收，无独立 issue）
- [ ] 部署文档：备份 SQLite volume、TLS 由反代或平台提供（并入 #6 切片05）。

## Validation Commands

- 构建：`go build ./...`
- 静态检查：`go vet ./...`
- 依赖整理：`go mod tidy`（确认 redis/mysql/postgres/mongo 移除、golang-migrate 加入）
- Wire 生成：在改动 DI 后重新生成 `wire_gen.go`
- 迁移：`golang-migrate` up / down 双向可执行
- 测试：`go test ./...`
- 部署构建：`docker compose -f deploy/docker-compose/docker-compose.yml build`

## Risky Files / Rollback Points

- `api/v1/v1.go`：保留 Nunu envelope，仅追加错误码，避免结构改动波及所有 handler。
- `internal/repository/repository.go`：`NewDB` 与 DI 强耦合，删 Redis/Mongo 需同步 wire set。
- `go.mod` / `wire_gen.go`：依赖与生成代码，改后必须重生成并 `go build` 验证。
- `internal/server/http.go`：路由前缀变更影响前端联调。
- `internal/server/migration.go` + `cmd/migration`：迁移机制切换是结构性改动，保留 git 提交粒度以便回滚。
- module 名重命名 `shuliu` → `shiliu`：全仓 import 路径替换 + `wire_gen.go` 重生成，机械但影响面广，作为 Phase 0 第一步独立提交，便于回滚。

## Follow-up Before task.py start

- [x] 使用者审阅 `prd.md` / `design.md` / `implement.md`。
- [x] curate `implement.jsonl` 与 `check.jsonl`（spec / research 清单）。
- [x] S0（FTS5 验证）已实测通过，搜索按 `design.md` Search Model 落地，无需替代方案。
- [x] PRD 已发布为父 issue #1；28 个纵向切片已发布为 issue #2–#29（切片号 + 1），均打 `ready-for-agent` 标签并引用 #1；中文 body 副本在 `issues/slice-*.md`。

## Issue 发布对照（切片号 → issue）

- 地基：#2 切片01 · #3 切片02 · #4 切片03 · #5 切片04 · #6 切片05
- 鉴权：#7 切片06 · #8 切片07 · #9 切片08 · #10 切片09
- 订阅源/抓取：#11 切片10 · #12 切片11 · #13 切片12 · #14 切片13 · #15 切片14 · #16 切片15
- 内容列表/搜索：#17 切片16 · #18 切片17 · #19 切片18 · #20 切片19 · #21 切片20
- 标签/文件夹：#22 切片21 · #23 切片22
- AI 摘要：#24 切片23 · #25 切片24
- 自动化：#26 切片25 · #27 切片26
- 导出/导入：#28 切片27 · #29 切片28
- 起手（无 Blocker）：#2（module 重命名）、#12（净化单例，仅依赖 #2）
