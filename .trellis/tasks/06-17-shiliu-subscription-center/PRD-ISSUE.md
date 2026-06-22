# 拾流订阅中心 MVP 后端 PRD

> 领域语言以根 `CONTEXT.md` 为准；需求边界以 `.trellis/tasks/06-17-shiliu-subscription-center/prd.md` 为准，技术设计见同目录 `design.md`，执行计划见 `implement.md`。本 issue 是面向实现 agent 的产品需求文档。

## Problem Statement

作为一名开发者 / 信息重度用户，我的文章、博客、新闻、播客和音频节目散落在不同的 RSS 阅读器和播客客户端里。我没有一个统一的地方既能消费文本又能消费音频，更没办法在同一处整理、搜索、用 AI 理解这些内容，并把有价值的部分沉淀进我的 Obsidian 知识库。现有工具要么只做 RSS 阅读，要么只做播客播放，把"订阅 → 消费 → 整理 → 找回 → 理解 → 沉淀"这条链路割裂开了。我也不想把我的订阅数据交给一个公开多租户 SaaS——我要一个能自托管在自己 VPS 上、由单人账户保护的单实例产品。

## Solution

拾流是一个面向开发者与信息重度用户的**订阅中心**：统一管理 RSS 源和播客源，把新内容聚合进**内容收件箱**，让使用者在一个地方阅读文本、收听音频、用**处理状态**和**内容标记**整理、用**基础搜索**找回、用 **AI 摘要**理解，并把单条内容**导出到 Obsidian** 沉淀。

MVP 是一个本地 / VPS / 自托管的**单实例** Web 产品，Go 后端（Nunu 脚手架）+ SQLite 文件数据库，由一个**用户账户**保护访问。后端通过 Docker Compose 用同一镜像启动 `server`（HTTP API）和 `task`（定时抓取 / 自动摘要）两个服务，共享 SQLite volume。本 PRD 只覆盖后端边界；React 前端细节另议。

## User Stories

### 首次初始化与鉴权

1. 作为部署拾流的使用者，我想在第一次访问时通过**首次初始化**创建唯一**用户账户**，以便用我自己的凭据保护这个单实例。
2. 作为使用者，我想在唯一用户账户创建成功后让初始化入口永久关闭，以便没有人能通过 Web 界面再创建第二个账户。
3. 作为使用者，我想用 `username + password` 登录，以便不被强制提供邮箱或个人资料。
4. 作为使用者，我想在密码至少 12 个字符时才被接受，以便我的单实例有基本的密码强度保护。
5. 作为使用者，我想登录成功后拿到一个 30 天有效的 JWT，以便在有效期内用 `Authorization: Bearer <token>` 访问 API 而不必频繁登录。
6. 作为使用者，我想在连续 5 次登录失败后账户被锁定 15 分钟，以便阻挡无限暴力破解。
7. 作为使用者，我想在一次成功登录后失败计数被清零，以便偶尔输错密码不会累积到锁定。
8. 作为已登录的使用者，我想修改我的密码，以便定期轮换凭据。
9. 作为使用者，我想所有业务 API 都要求有效 JWT，以便未鉴权的请求无法读取或修改我的订阅数据。
10. 作为使用者，我想手动退出时由前端删除 token 即可，以便不依赖服务端会话表。

### 订阅源管理

11. 作为使用者，我想通过粘贴 RSS feed URL 添加一个**订阅源**，以便开始追踪它产生的内容条目。
12. 作为使用者，我想添加播客 RSS feed URL 时它被识别为 `podcast` 类型订阅源，以便音频消费入口正确。
13. 作为使用者，我想只有在拾流成功抓取并解析 feed 之后订阅源才被创建，以便我的列表里不出现空的或失败的订阅源草稿。
14. 作为使用者，我想添加一个已存在的订阅源（按规范化 feed URL 判断）时被告知重复，以便不产生重复订阅源。
15. 作为使用者，我想查看我所有订阅源的列表及其当前抓取状态，以便了解哪些源最近抓取成功或失败。
16. 作为使用者，我想删除一个订阅源时它产生的所有内容条目及派生数据（摘要、播放进度、内容标记、标签关联）一并删除，以便不留下无归属的孤儿内容。

### 订阅源抓取

17. 作为使用者，我想在添加订阅源后立即抓取一次，以便马上看到它的内容。
18. 作为使用者，我想首次抓取每个订阅源最多保存最近 50 条历史内容，以便冷启动不被海量历史淹没、自动摘要成本不失控。
19. 作为使用者，我想手动刷新全部订阅源，以便一键拉取所有源的最新内容。
20. 作为使用者，我想手动刷新单个订阅源，以便只更新我关心的那一个。
21. 作为使用者，我想开启一个全局后台定时抓取，间隔可选关闭 / 30 分钟 / 60 分钟 / 6 小时 / 24 小时（默认 60 分钟），以便订阅源自动保持更新。
22. 作为使用者，我想同一订阅源的抓取互斥，已有抓取进行中时新的同源请求被跳过并返回"已在抓取中 / 已跳过"，以便不重复抓取或互相覆盖状态。
23. 作为使用者，我想后续抓取只新增同源内未见过的内容条目（insert-only），不改写已存在条目，以便我已读 / 已整理的内容不被悄悄变更。
24. 作为使用者，我想进程崩溃后残留的 in-progress 抓取状态能被恢复，以便订阅源不会永久卡在"抓取中"。
25. 作为使用者，我想每个订阅源只保留当前 / 最后一次抓取状态（`fetch_status` / `fetch_started_at` / `last_fetched_at` / `last_fetch_error`），以便排查最近一次抓取结果，而不必维护抓取历史表。

### 内容收件箱与列表视图

26. 作为使用者，我想在**内容收件箱**看到来自所有订阅源的新内容条目，以便集中处理待消费信息流。
27. 作为使用者，我想内容收件箱默认按内容条目发布时间倒序、缺失或异常时回退到抓取时间，以便最新内容在最上面且顺序不被抓取时间污染。
28. 作为使用者，我想内容收件箱默认只显示未处理（`processing_status=unprocessed`）的内容条目，以便聚焦还没处理完的内容。
29. 作为使用者，我想"稍后处理""收藏""已完成"都是同一内容列表的预设过滤视图（分别预设 `mark=later` / `mark=favorite` / `processing_status=completed`），以便它们共享一致的查询、分页、排序和条目操作。
30. 作为使用者，我想订阅源详情页是同一内容列表预设 `feed_id` 过滤的视图，以便用一致的方式浏览单个订阅源的内容。
31. 作为使用者，我想在任意列表视图上追加单值过滤（内容类型 / 处理状态 / 内容标记 / 标签 / 文件夹 / 订阅源），多个条件之间是 AND，以便精确缩小范围。
32. 作为使用者，我想列表用 `page + pageSize` 分页（默认 20、最大 100），以便翻页浏览大量内容条目。

### 文本阅读与音频收听

33. 作为使用者，我想打开文本型内容条目时看到基础渲染的标题、正文 / 摘要、来源和发布时间，以便阅读。
34. 作为使用者，我想内容条目只有摘要而无正文时能打开原文链接，以便补全阅读。
35. 作为使用者，我想阅读的是经过 HTML 净化的安全内容，以便不被恶意 feed 注入脚本。
36. 作为使用者，我想打开音频型内容条目时能播放 / 暂停、拖动进度条、15 秒快进 / 后退和倍速播放，以便舒适收听。
37. 作为使用者，我想音频播放进度被持久化，以便下次继续收听。
38. 作为使用者，我想文本阅读位置不被持久化（MVP 不做），以便我清楚消费进度只覆盖音频。

### 处理状态与内容标记

39. 作为使用者，我想手动把内容条目标记为已完成，也能从已完成改回未处理，以便由我决定处理进度。
40. 作为使用者，我想阅读 / 收听进度不自动改变处理状态，以便听完一集播客不会被误判为已完成。
41. 作为使用者，我想给内容条目加"稍后处理"标记并能叠加在处理状态之上，以便排队待办而不改变主流程位置。
42. 作为使用者，我想给内容条目加"收藏"标记，以便沉淀高价值内容。

### 标签与文件夹

43. 作为使用者，我想创建、重命名、删除**标签**，以便用自定义主题词组织内容条目。
44. 作为使用者，我想给单个内容条目分配 / 移除多个标签（只接受已存在的 `tag_id`），以便一条内容能归入多个主题。
45. 作为使用者，我想标签名唯一、重名创建返回冲突，以便标签集合不混乱。
46. 作为使用者，我想按标签过滤内容条目，以便快速找到某主题下的全部内容。
47. 作为使用者，我想删除标签时只移除标签及其关联、内容条目保留，以便整理标签不会误删内容。
48. 作为使用者，我想创建、重命名、删除**文件夹**，以便组织订阅源。
49. 作为使用者，我想给单个订阅源分配零个或一个文件夹（只接受已存在的 `folder_id` 或置空），以便订阅源最多属于一个文件夹。
50. 作为使用者，我想文件夹名唯一、重名创建返回冲突，以便文件夹集合不混乱。
51. 作为使用者，我想按文件夹过滤内容收件箱和搜索结果，以便按来源分组浏览。
52. 作为使用者，我想删除文件夹时只移除分组本身、原属订阅源变为未归类并保留，以便整理分组不会误删订阅源或其内容。

### 基础搜索

53. 作为使用者，我想用关键词搜索内容条目，匹配标题、订阅源名称、**可用文本**和当前 AI 摘要，以便重新找到读过或听过的内容。
54. 作为使用者，我想有关键词时结果按相关性排序、相关性相同时按发布时间倒序，以便最相关且最新的内容靠前。
55. 作为使用者，我想无关键词、只有过滤条件时结果按发布时间倒序，以便搜索与内容收件箱行为一致。
56. 作为使用者，我想搜索能叠加与内容收件箱一致的单值 AND 过滤，以便在关键词基础上进一步缩小范围。
57. 作为使用者，我想搜索结果遵守统一的 `page + pageSize` 分页，以便翻页查看大量命中。

### AI 摘要

58. 作为使用者，我想对单个内容条目手动生成一次 **AI 摘要**，输出固定结构化简体中文 Markdown（TL;DR / 要点 / 对开发者价值 / 原文信息），以便快速理解内容。
59. 作为使用者，我想 AI 摘要只基于可用文本生成，当文本不足时诚实地标记为 `insufficient_text` 而不是编造，以便摘要可信。
60. 作为使用者，我想每个内容条目只保留一个当前摘要及其状态（`none` / `pending` / `success` / `failed` / `insufficient_text`），以便不被多版本摘要干扰。
61. 作为使用者，我想手动摘要能在 `none` / `failed` / `success` 状态触发并覆盖当前摘要或错误，以便重试失败或刷新已有摘要。
62. 作为使用者，我想在摘要 `pending` 时再次触发会被告知"正在生成"，以便不重复发起。
63. 作为使用者，我想 `insufficient_text` 在 MVP 中作为终态不可重试，以便不在注定失败的内容上反复消耗 API 调用。
64. 作为使用者，我想摘要失败时记录失败原因且不自动重试，以便我决定是否手动重试。
65. 作为使用者，我想开启**自动摘要**并选择范围（文本型 / 音频型 / 两者），以便新内容自动获得摘要。
66. 作为使用者，我想自动摘要只处理配置开启后新抓取入库、且状态为 `none` 的内容条目，以便不回填历史、不覆盖已有成功摘要、不自动重试失败或文本不足。
67. 作为使用者，我想手动摘要始终可用，与自动摘要开关无关，以便随时按需理解任意内容条目。

### AI 服务配置

68. 作为使用者，我想配置自己的 AI 服务（API Base URL、API Key、模型名），以便用自带的 OpenAI-compatible 服务生成摘要。
69. 作为使用者，我想保存 AI 配置时只做格式校验、不强制测试成功，以便先保存后验证。
70. 作为使用者，我想有一个可选的"测试配置"动作，以便在需要时主动验证连通性。
71. 作为使用者，我想 API Key 只存在服务端 SQLite、前端只显示"已配置"而不回显完整密钥，且导出 / 搜索 / 错误 / 日志都不泄露它，以便我的密钥安全。

### Obsidian 导出

72. 作为使用者，我想把单个内容条目导出为 Markdown（复制或下载），以便沉淀进我的 Obsidian vault。
73. 作为使用者，我想导出内容包含标题、原文链接、订阅源、发布时间、内容类型、标签和订阅源文件夹作为普通 Markdown 元信息，以便保留上下文。
74. 作为使用者，我想导出始终包含一个 `AI 摘要` 区块，按当前摘要状态写入摘要正文或"未生成 / 正在生成 / 生成失败 / 可用文本不足"说明，以便导出不被摘要状态阻塞。
75. 作为使用者，我想导出只包含可用文本前 2000 个字符、被截断时追加"已截断，请打开原文链接查看完整内容"提示，以便导出是摘录而非全文搬运。
76. 作为使用者，我想导出不强制生成 AI 摘要、也不因 AI 服务不可用而被阻止，以便随时导出任意内容条目。

### OPML 导入

77. 作为使用者，我想上传或粘贴 OPML 一次性批量创建订阅源，以便快速从旧阅读器迁移。
78. 作为使用者，我想 OPML 导入只读取 feed URL、忽略原文件夹 / 分组层级，以便导入边界清晰简单。
79. 作为使用者，我想 OPML 中每个订阅源也必须先抓取解析成功才创建、失败项计入失败数，以便不产生空订阅源。
80. 作为使用者，我想导入后看到成功 / 重复 / 失败的数量统计，以便了解导入结果。

### 部署与运维

81. 作为使用者，我想用 Docker Compose 同一镜像启动 `server` 和 `task` 两个服务并共享 SQLite volume，以便单实例部署简单。
82. 作为使用者，我想 `cmd/migration` 在 `server` / `task` 启动前执行显式版本化迁移，以便数据库结构变化可控可回滚。
83. 作为使用者，我想自己通过反向代理或平台提供 TLS，以便理解应用本身不内置证书管理。
84. 作为使用者，我想按部署文档备份 SQLite 数据库 / volume，以便理解产品内不提供整库备份 / 恢复。

## Implementation Decisions

### 脚手架与基础设施（Phase 0）

- 沿用 Nunu 脚手架的 handler / service / repository / middleware / cmd 分层，使用 Gin / Gorm / Wire / Viper / Zap。
- **Go module 重命名** `shuliu` → `shiliu`：改 `go.mod` module 行 + 全仓 import 路径替换 + 重生成 `wire_gen.go`，作为 Phase 0 第一个独立提交（机械但影响面广，便于回滚）。
- 数据层改为 **SQLite-only**：清理 `internal/repository/repository.go`，只保留 `NewDB`（`glebarez/sqlite`），移除 `NewRedis` / `NewMongo` 及 MySQL / Postgres 分支并同步 wire set；`go.mod` 移除 redis / mysql / postgres / mongo 驱动后 `go mod tidy`。
- **响应结构**：沿用 Nunu 模板 `{ "code": <int>, "message": "...", "data": ... }`，不改写。成功 HTTP 200 + `code=0`；失败由 `HandleError` 返回业务 `code` / `message` + HTTP status；错误码集中在 `api/v1` 的 `errorCodeMap` 注册（只追加拾流业务错误码）。
- **分页契约**：列表 API 用 `page + pageSize`（`page` 从 1 开始，默认 `pageSize=20`、最大 100），`data` 内含 `items` 和 `page:{page,pageSize,total}`；handler 内部把 `page + pageSize` 换算为 SQL / FTS5 的 `limit + offset`。
- **路由前缀** `/v1` → `/api/v1`（`internal/server/http.go`）。
- **Docker Compose** 重写为同一镜像启动 `server` + `task` 共享 SQLite volume，`migration` 作为一次性前置 job。
- 清理 `config/*.yml`：移除 redis / mongo；保留 SQLite dsn 和 jwt.key；新增抓取间隔与 AI 服务相关配置键。

### 迁移机制（Phase 1）

- 引入 `golang-migrate/migrate`，用成对 SQL 文件（`000001_xxx.up.sql` / `.down.sql`）。
- 重写 `cmd/migration` / `internal/server/migration.go`：移除 Gorm `AutoMigrate`，改为执行版本化迁移；`server` / `task` 启动前可独立执行，不做隐式迁移。

### 数据模型与 Schema

- **User**（最小鉴权字段）：`id`、`username`、`password_hash`、登录失败计数、锁定到期时间、`created_at`、`updated_at`。去掉 Nunu 模板的 `Nickname` / `Email` / 软删除字段。
- **feeds**：`feed_url`（唯一约束）、`type`（`rss` / `podcast`）、`fetch_status` / `fetch_started_at` / `last_fetched_at` / `last_fetch_error`、`folder_id`（可空）。feed URL 规范化只做 trim / 去 fragment / scheme-host 小写 / 默认端口归一。
- **content_items**：`feed_id`、`dedupe_key`（`(feed_id, dedupe_key)` 唯一约束），`type`（`text` / `audio`）；原始 feed 文本字段 `title` / `description` / `content` / `show_notes`（按 feed 原样保真存储）+ 净化后字段 `description_safe` / `content_safe` / `show_notes_safe`；去标签纯文本 `available_text`；发布时间 / 抓取时间；音频播放进度。`dedupe_key` 生成优先级：feed item `guid` → `link` → `title + published_at` 稳定哈希。
- **tags**（名称唯一）+ **content_item_tags** 多对多关联表。
- **folders**（名称唯一）+ `feeds.folder_id` 外键关系。
- **AI 摘要字段**：当前摘要 Markdown、状态（`none` / `pending` / `success` / `failed` / `insufficient_text`）、生成时间、错误原因。
- **AI 服务配置**：API Base URL、模型名、server-only API Key。
- **FTS5 虚拟表**：索引内容条目标题、订阅源名称、`available_text`、当前 AI 摘要 Markdown；通过显式 migration 创建，并在相关字段变更时同步。

### 抓取 / 净化 / 可用文本管线（service 层，handler 与 task 共享）

- **抓取能力放在 service 层**，被 HTTP handler（手动刷新 / 添加订阅源 / OPML 导入）和 `cmd/task`（后台定时抓取）共享，禁止分叉实现；`internal/task` 只做调度编排。
- **HTML 净化在信任边界**（抓取解析后、入库前，与 insert-only 一致，每条只净化一次）。使用单一全局 `bluemonday` 白名单策略（`UGCPolicy` 适度收紧）作为 `pkg` 级单例，**禁止按来源 / 按站点适配净化规则**——白名单模型只声明允许输出的标签 / 属性，输入多样性不增加规则数量。
- **双存**：原始 HTML 入库供保真与未来对存量重新净化（insert-only 不重抓，只能靠 migration 重净化）；净化后 HTML 供渲染；`available_text` 从净化后内容去标签 + 规范化空白生成，优先级 `content` → `show_notes` → `description/summary` → `title`。
- **同源互斥**：同一订阅源抓取必须互斥，可由数据库字段或事务级状态表达，必须能恢复进程崩溃后的过期 in-progress 状态；不同订阅源是否并发由实现选择。
- **删除订阅源级联**删除其内容条目及派生数据（摘要 / 播放进度 / 内容标记 / 标签关联）。

### 统一内容列表查询（Phase 4，依赖已通过的 S0）

- **单一查询入口**承载所有列表视图：单值过滤 `content_type` / `processing_status` / `mark` / `tag_id` / `folder_id` / `feed_id`，条件间 AND；预设视图（内容收件箱 / 稍后处理 / 收藏 / 已完成 / 订阅源详情）只是在此之上预设某个过滤值。禁止为各视图分叉列表查询。
- **基础搜索**用 FTS5 `MATCH` + `bm25()` 相关性排序：有关键词→相关性，相同→发布时间倒序；无关键词→发布时间倒序。结构化过滤仍走普通表字段 / 关联表。
- 内容收件箱与搜索页复用同一套查询 / 过滤语义，区别只在是否传关键词。

### AI 摘要（Phase 6）

- OpenAI-compatible 非流式 Chat Completions 客户端（注入式，便于测试替换）。
- **手动摘要（handler）+ 自动摘要（task）复用同一 service**；状态机 `none` / `pending` / `success` / `failed` / `insufficient_text` 与触发覆盖规则集中在 service 层。
- 保存配置只做格式校验 + 可选"测试配置"动作；API Key 不回显、不入日志 / 导出 / 搜索 / 错误。

### Obsidian 导出（Phase 7）

- 单条内容条目 → Markdown：标题、链接、订阅源、发布时间、内容类型、标签、订阅源文件夹、`AI 摘要` 区块（按状态填充）、`available_text` 前 2000 字符摘录 + 截断提示。纯函数式映射，不依赖 AI 服务可用性。

### OPML 导入（Phase 8）

- 解析 OPML 只读 feed URL、忽略文件夹层级；逐个复用 service 层抓取能力，成功才创建；按规范化 URL 统计重复；返回成功 / 重复 / 失败数量。

### 定时任务（Phase 9）

- `TaskServer` 移除示例 `CheckUser`；接入全局后台抓取（间隔可配：关闭 / 30 / 60 / 360 / 1440 分钟，默认 60）与自动摘要，复用 service。

## Testing Decisions

测试遵循"只测外部行为，不测实现细节"：断言 API 契约、业务规则结果和持久化引擎行为，而非内部函数签名或 SQL 文本。需求驱动——每条需求至少映射一个测试场景（happy path + 边界 + 错误 + 状态转移）。

### 复用现有 seam（优先）

仓库已有三层测试 seam（`test/server/{handler,service,repository}` + `test/mocks`），优先复用：

- **Handler seam**（`test/server/handler`，最高 seam）：`httpexpect` 驱动真实 Gin engine、service 用 `gomock` mock。用于断言 HTTP 契约——`{code,message,data}` envelope、HTTP status、`data.items` + `data.page:{page,pageSize,total}` 形状、JWT 中间件保护、错误码映射。**Prior art**：`test/server/handler/user_test.go`（Register / Login / GetProfile）。新 handler 测试照此模式：mock service 返回 → 断言 envelope 与 data 形状。
- **Service seam**（`test/server/service`）：repository / transaction 用 `gomock` mock，无 SQL 依赖。用于断言业务规则——首次初始化后关闭入口、登录失败计数 / 锁定 / 清零、bcrypt cost 12、密码 ≥12 字符校验、AI 摘要状态机（`none`/`failed`/`success`→可手动触发，`pending`→正在生成，`insufficient_text`→终态）、自动摘要只处理 `none`、抓取同源互斥的跳过语义、dedupe insert-only。**Prior art**：`test/server/service/user_test.go`（gomock 模式）。

### 持久层 seam（替换现有 go-sqlmock 策略）

现有 `test/server/repository/user_test.go` 用 `go-sqlmock` + **MySQL** GORM 方言断言 SQL 字符串。本设计的核心持久化行为——FTS5 `MATCH` / `bm25` 排序、`(feed_id,dedupe_key)` / `feed_url` / tag-folder 名唯一约束、删除订阅源级联、删除文件夹置空 `folder_id`——**无法用 SQL 字符串断言验证**。

决策（已与使用者确认）：repository seam 改用**临时 / 内存 SQLite + 真实 golang-migrate 迁移**跑集成测试，针对真实引擎验证：

- FTS5 建表 + `MATCH` 命中 + `bm25()` 相关性排序 + 中文字段（**S0 已实测通过**：`glebarez/sqlite v1.11.0` / `modernc.org/sqlite v1.40.1` 支持 FTS5 + bm25，无需 CGO）。
- `(feed_id, dedupe_key)` 唯一 → insert-only 幂等；重复 insert 被正确跳过。
- `feeds.feed_url`、`tags.name`、`folders.name` 唯一约束 → 重复创建返回冲突。
- 删除订阅源 → 内容条目及派生数据级联删除，无孤儿。
- 删除文件夹 → 原属订阅源 `folder_id` 置空、订阅源与内容条目保留；删除标签 → 关联清理、内容条目保留。
- 统一列表查询的单值 AND 过滤组合与发布时间倒序排序。

现有 user repository 的 sqlmock 测试随 User 模型重构（去 Email/Nickname）一并更新或迁移到真实 SQLite。

### 新增 seam（提在最高点）

- **Fetcher 边界**：抓取 service 注入一个 `http.Client` / `Fetcher` 接口，测试用 fixture feed（RSS / podcast / 含恶意 HTML 的样本）驱动整条 fetch → sanitize → available_text → dedupe 管线，**无网络**。验证净化白名单剥除脚本、`available_text` 去标签与优先级、`dedupe_key` 生成、首次最多 50 条、双存原始 + 净化后。
- **ChatCompletion 边界**：AI 摘要 service 注入一个 OpenAI-compatible 客户端接口，测试用 stub 返回（成功 / 超时 / 空响应 / 文本不足）确定性驱动摘要状态机，**无外部 API 调用**。

### 交付测试范围

按需求驱动补测：鉴权 / 首次初始化、抓取去重 insert-only、删除级联、过滤与搜索排序、摘要状态机、导出截断、OPML 计数。

## Out of Scope

以下明确不进入 MVP（详见 `prd.md` Later Iterations / Out of Scope）：

- 多用户注册、多用户数据隔离、团队 / 角色 / 多租户、云端同步。
- 产品内密码恢复 / 恢复密钥 / CLI 重置；邮箱、昵称、profile、个人资料编辑。
- refresh token、服务端 session 表、"记住我"、多设备会话、JWT 密钥轮换、登录历史 / 安全审计 / 设备列表、IP 维度限速、2FA / Passkey、可配置 hash cost / Argon2 / scrypt。
- 应用内置 HTTPS / TLS / 证书管理；产品内整库备份 / 恢复 / 全量导出。
- PostgreSQL / MySQL / Redis / Mongo / 多数据库后端 / 横向扩展；裸 Gorm AutoMigrate 作为生产迁移；Goose / 无版本 migration runner。
- Next.js 全栈单体；React 前端组件 / 状态 / 路由 / UI 细节；GraphQL / tRPC / 多版本 API / 把 Swagger 当产品功能。
- 普通网页 URL / Newsletter / YouTube / Bilibili / Twitter 等非 RSS 来源；原网页全文抓取 / 正文抽取 / 阅读模式 / 高亮 / 批注 / 主题 / 字体 / 文本阅读位置持久化。
- 取消订阅但保留历史内容、归档 / 隐藏订阅源；抓取历史表 / 趋势 / 审计；每源自定义抓取频率 / 智能退避 / WebSub / 复杂队列 / 多节点调度。
- 文章 / 博客 / 新闻细分内容类型；跨源 canonical 探测 / feed autodiscovery / 相似源合并；OPML 自动同步 / 文件夹映射。
- 发布时间范围 / 多选 / OR 条件组 / 保存搜索条件；中文分词插件 / 拼音 / 模糊纠错 / 向量语义搜索 / 问答检索。
- 标签 / 文件夹高级能力：批量编辑 / 自动打标 / 嵌套文件夹 / 多文件夹归属 / 导出到 frontmatter。
- 批量 AI 摘要 / 历史回填 / 摘要版本历史 / 多模型对比；官方托管 AI 服务 / 平台密钥 / 配额计费；原生多供应商适配 / provider 类型 / 流式响应；强制配置测试门禁 / 健康检查；API Key 字段加密 / 外部 secret manager / 密钥轮换。
- 音频转写 / 播放队列 / 章节 / 离线下载；Obsidian 插件 / frontmatter / 模板 / 双链 / vault 自动同步 / 全文导出。

## Further Notes

- **领域语言权威**：实现时命名、边界、术语必须与根 `CONTEXT.md` 一致（使用者 vs 用户账户、订阅源仅 rss/podcast、内容条目仅 text/audio、可用文本=净化去标签后文本、处理状态 vs 内容标记 vs 标签）。
- **跨层 / 复用守则**：见 `.trellis/spec/guides/cross-layer-thinking-guide.md` 与 `code-reuse-thinking-guide.md`——统一列表查询、bluemonday 单例、handler 与 task 共享抓取 / 摘要 service 都要求复用而非分叉。
- **执行顺序与回滚点**：见 `implement.md`（Phase 0 → 10、Validation Commands、Risky Files）。Phase 0 的 module 重命名、repository.go 清理、go.mod 清理建议各自独立提交。
- **S0 已闭环**：FTS5 可用性已实测通过，搜索按 Search Model 落地，无需 CGO `mattn/go-sqlite3` 或 `LIKE` 降级。
- **安全红线**：feed HTML 不可信，必须在入库边界净化；AI API Key 仅服务端、永不回显 / 入日志 / 导出 / 搜索 / 错误；所有业务路由由 JWT 中间件保护。
- 验证命令：`go build ./...`、`go vet ./...`、`go mod tidy`、wire 重生成、golang-migrate up/down、`go test ./...`、`docker compose build`。
