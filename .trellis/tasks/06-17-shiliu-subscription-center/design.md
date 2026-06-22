# 拾流订阅中心后端技术设计

## Scope

本设计只覆盖拾流 MVP 后端边界。前端方向为 React，但 React 组件、状态管理、路由和 UI 技术细节不在本轮讨论范围内。

## Architecture

- 后端使用 Go。
- Go 后端采用 Nunu 作为脚手架。
- 保留 Nunu 的 handler / service / repository / middleware / cmd 分层思路。
- 使用 Nunu 模板中的 Gin、Gorm、Wire、Viper、Zap。
- 后端 API 采用 REST JSON。
- API 统一挂载在 `/api/v1` 前缀下。
- 沿用 Nunu 模板统一响应结构 `{ "code": <int>, "message": "...", "data": ... }`。
- 成功响应 HTTP 200，`code` 为 0，业务数据放在 `data`。
- 失败响应由 `HandleError` 返回业务 `code`、`message` 和可选 `data`，HTTP status 表达错误类别。
- 错误码集中在 `api/v1` 的 `errorCodeMap` 注册。
- 分页列表在 `data` 内包含 `items` 和 `page`。
- 列表 API 使用 `page + pageSize` 页码分页（`page` 从 1 开始）。
- 默认 `pageSize=20`，最大 `pageSize=100`。
- `data.page` 返回 `page`、`pageSize`、`total`。
- handler 内部把 `page + pageSize` 换算为 SQL / FTS5 的 `limit + offset`。
- Cursor pagination 后置。
- OpenAPI / Swagger 只作为开发辅助，不作为产品功能。
- 不做 GraphQL、tRPC 或多版本 API。
- 不采用 Next.js 全栈单体。
- 不沿用 Nunu 示例中的 MySQL / Redis 运行假设。

## Runtime Entrypoints

- `cmd/server`：启动 HTTP API 服务。
- `cmd/task`：启动 Nunu `TaskServer`，使用 `gocron` 执行定时任务。
- `cmd/migration`：执行数据库显式迁移流程。

Docker Compose 使用同一后端镜像启动两个长期运行服务：

- `server`：运行 `cmd/server`。
- `task`：运行 `cmd/task`。

两个服务共享同一个 SQLite 数据 volume。`cmd/migration` 必须在 `server` / `task` 正常运行前执行。

## Data Store

- MVP 使用 SQLite 文件数据库。
- SQLite 文件持久化在部署 volume 中。
- 不引入 PostgreSQL、MySQL、Redis、Mongo、独立缓存服务、独立任务队列服务或数据库多后端适配。
- 数据库结构变化必须通过 `golang-migrate/migrate` 执行。
- 迁移文件使用成对 SQL 文件，例如 `000001_init.up.sql` / `000001_init.down.sql`。
- `cmd/migration` 保留为 Nunu 迁移入口，并负责调用 `golang-migrate/migrate`。
- `cmd/migration` 必须在 `server` / `task` 正常运行前执行。
- 不依赖裸 Gorm `AutoMigrate` 作为生产迁移机制。
- 不在 `server` / `task` 启动时做隐式迁移。
- Goose、自写无版本 migration runner 和非成对 SQL 文件迁移方案不进入 MVP。

## Backend Layering

- HTTP handler 负责请求解析、鉴权上下文和响应映射。
- service 层承载业务逻辑。
- repository 层封装 SQLite / Gorm 数据访问。
- `internal/task` 只负责定时调度编排，不承载核心业务逻辑。
- 订阅源抓取、OPML 导入后抓取、手动刷新、自动摘要和手动摘要必须复用 service 层能力。
- HTTP handler 与 `cmd/task` 共同调用同一套 service，避免手动流程和后台流程分叉。

## Content Item Identity

- 内容条目必须归属于一个订阅源。
- 删除订阅源时必须同时删除该订阅源产生的内容条目及派生数据，包括摘要、播放进度、内容标记和标签关联。
- MVP 不保留无订阅源归属的孤儿内容条目。
- 取消订阅但保留历史内容、归档订阅源或隐藏订阅源后置。
- 内容条目只在同一订阅源内去重。
- 每个内容条目保存 `feed_id` 和 `dedupe_key`。
- `dedupe_key` 生成优先级为 feed item `guid` → `link` → `title + published_at` 稳定哈希。
- 数据库必须对 `(feed_id, dedupe_key)` 建唯一约束。
- 后续抓取采用 insert-only：发现同一 `(feed_id, dedupe_key)` 已存在时跳过该条目，不更新标题、描述、正文、show notes 或 `available_text`。
- MVP 不做已有内容条目修订同步、摘要过期标记或重新导出提示。
- 音频 enclosure URL 是音频字段，不作为主去重键。
- 跨订阅源重复、相似内容合并和 canonical URL 归一化后置。

## Feed Creation Boundary

- 手动添加订阅源必须先成功抓取并解析 feed，之后才创建订阅源记录。
- URL 无法抓取或解析失败时不保存订阅源，只返回错误。
- OPML 导入中的单个订阅源也必须先成功抓取并解析 feed 才创建。
- OPML 导入失败项计入失败数量，不创建空订阅源。
- OPML 导入只读取 feed URL，忽略原 OPML 文件夹 / 分组层级。
- OPML 导入不自动创建拾流文件夹，也不自动把订阅源分配到文件夹。
- MVP 不保存首次抓取 / 解析失败的空订阅源、失败订阅源草稿或稍后自动补抓。
- OPML 自动同步、复杂文件夹层级管理、文件夹自动映射和按 OPML 分组自动创建拾流文件夹后置。

## Feed Identity

- 添加订阅源和 OPML 导入时，MVP 只按规范化后的 feed URL 判断订阅源重复。
- 数据库必须对 `feeds.feed_url` 建唯一约束。
- 订阅源标题、站点链接和描述不参与去重。
- feed URL 规范化只做 trim、去掉 fragment、scheme / host 小写、默认端口归一。
- MVP 不做跨 URL canonical 探测、feed autodiscovery、相似订阅源合并或按标题 / 站点链接推断重复订阅源。

## Feed Type Model

- MVP 订阅源类型只使用 `rss` 和 `podcast` 两类。
- 包含播客 RSS 语义或音频 enclosure 的订阅源保存为 `podcast`。
- 其余结构化 feed 保存为 `rss`。
- 订阅源类型用于订阅源展示、过滤和默认消费入口。
- 内容条目的最终 `text` / `audio` 类型仍以条目自身字段为准。
- 订阅源自定义分类、博客 / 新闻等更细来源类型和混合 feed 复杂拆分后置。

## Content Type Model

- MVP 内容条目类型只使用 `text` 和 `audio` 两类。
- 文章、博客和新闻统一保存为 `text`。
- 播客单集和音频节目单集统一保存为 `audio`。
- 搜索过滤、自动摘要范围和导出中的内容类型都基于 `text` / `audio`。
- 文章 / 博客 / 新闻等更细分类后置，不进入 MVP 数据模型。

## Content Text Model

- 内容条目保留原始 feed 文本字段：`title`、`description`、`content`、`show_notes`，按 feed 原样存储不改写（保真）。
- 内容条目同时存储净化后字段：`description_safe`、`content_safe`、`show_notes_safe`，由入库净化生成，供阅读渲染。
- 原始与净化后双存：原始用于保真和将来净化规则调整后对存量重新净化（insert-only 不重抓，只能靠 migration 重净化）。
- 后端生成规范化 `available_text`，从净化后内容去除 HTML 标签并规范化空白得到纯文本。
- `available_text` 生成优先级为 `content` → `show_notes` → `description/summary` → `title`（取净化后内容去标签）。
- `available_text` 作为基础搜索、AI 摘要和 Obsidian 导出的统一输入。
- 不抓原网页全文。
- 不做正文抽取。
- 不只保存单一 `content` 字段而丢失来源语义。

## HTML Sanitization

- 外部 feed HTML 不可信；净化发生在抓取解析后、入库前的信任边界，与 insert-only 一致，每条只净化一次。
- 使用单一全局 `bluemonday` 白名单策略（基于 `UGCPolicy` 适度收紧），作为 `pkg` 级单例，所有抓取共用。
- 白名单模型只声明允许输出的标签 / 属性，其余一律剥除；输入多样性不增加规则数量。
- 严禁按来源 / 按站点编写净化适配规则；按来源适配净化明确后置 / 不做，从设计上堵死复杂度膨胀。
- 净化后 HTML 入库供阅读渲染；`available_text` 再从净化后内容去标签生成。
- DB 同时保存原始 HTML，仅供保真与未来对存量重新净化，不直接渲染未净化原始 HTML。
- 相对 URL 补全为绝对 URL 在 MVP 后置，打开原文链接兜底。

## Feed Refresh Boundary

- MVP 支持手动刷新全部订阅源和手动刷新单个订阅源。
- 同一订阅源抓取必须互斥；添加订阅源后的首次抓取、OPML 导入后的抓取、手动刷新和后台定时抓取都必须遵守同源互斥。
- 如果某个订阅源已有抓取进行中，新的同源抓取请求必须跳过，并向调用方返回“已在抓取中 / 已跳过”的结果语义。
- 订阅源记录只保留当前 / 最后状态字段：`fetch_status`（`idle` / `fetching` / `success` / `failed`）、`fetch_started_at`、`last_fetched_at`、`last_fetch_error`。
- `fetch_started_at` 用于同源互斥和进程崩溃后的过期 in-progress 状态恢复。
- MVP 不建抓取历史表，不提供抓取历史列表、错误趋势分析、抓取审计日志或可视化抓取诊断页面。
- 同源互斥可由数据库字段或事务级状态表达，必须能处理进程崩溃后的过期 in-progress 状态恢复。
- 不同订阅源是否并发由实现选择。
- MVP 不引入 Redis / 队列系统、复杂全局抓取队列、可配置并发池、多节点调度或跨实例锁。
- 手动刷新由 HTTP handler 触发，并复用 service 层订阅源抓取能力。
- 全局后台定时抓取由 `cmd/task` / `TaskServer` 触发，并复用同一套 service 层抓取能力。
- 添加订阅源后的首次抓取、OPML 导入后的新增订阅源抓取、手动刷新和后台定时抓取不得分叉实现。
- MVP 不做每源自定义定时抓取规则。

## Scheduled Work

Nunu 模板中的定时任务结构作为拾流 MVP 的后端任务结构：

- `cmd/task` 为独立任务入口。
- `internal/server/task.go` 中的 `TaskServer` 管理 `gocron` scheduler。
- `internal/task` 放置定时任务编排逻辑。
- 订阅源后台抓取和自动摘要由 `TaskServer` 定时触发。
- 长驻 job 与定时任务分开；MVP 不引入独立 worker、Redis 队列、系统 cron 或额外任务服务。

## Deployment Boundary

- 主要使用者部署方式为 Docker Compose。
- Compose 以同一镜像启动 `server` 与 `task` 两个服务。
- 两个服务共享 SQLite 数据 volume。
- 源码本地运行只作为开发方式。
- 公网 VPS 部署必须由使用者自管反向代理或平台能力提供 HTTPS / TLS。
- 应用不内置 HTTPS / TLS 终止、证书签发或证书续期管理。
- 产品内不提供整库备份 / 恢复；部署文档说明备份数据库 / volume。

## Security Boundary

- MVP 支持一个单人用户账户。
- 首次访问通过一次性初始化创建唯一用户账户。
- 单人用户账户使用 `username + password` 登录。
- 用户账户模型只保留鉴权必需字段：`id`、`username`、`password_hash`、登录失败计数、锁定到期时间、创建时间和更新时间。
- 邮箱不进入 MVP 用户账户模型。
- 不做昵称、profile、头像或个人资料编辑。
- 不做邮箱验证、邮件通知或邮箱找回密码。
- 密码最少 12 个字符。
- 不强制大小写 / 数字 / 符号组合。
- 密码使用 bcrypt 保存哈希，cost 固定为 12。
- 不做可配置 hash cost、Argon2 或 scrypt。
- 登录会话采用 JWT Bearer Token。
- 登录成功返回 30 天有效的 access token。
- API 请求使用 `Authorization: Bearer <token>`。
- JWT 签名密钥由使用者自行修改 yml 配置。
- 手动退出由前端删除 token。
- 不做 refresh token、服务端 session 表、“记住我”、多设备会话管理、强制全部设备下线、可配置会话策略、JWT 密钥轮换或生产环境密钥强制校验。
- 登录保护只记录唯一用户账户的失败次数和锁定到期时间。
- 连续 5 次登录失败后锁定 15 分钟。
- 成功登录后清空失败计数。
- 不做 IP / 设备维度限速、指数退避、永久锁定、登录历史、安全审计页面、安全通知或设备列表。
- 已登录状态下可修改密码。
- 登录页不提供 Web 端忘记密码流程。
- 首版不规划 CLI 重置密码能力。
- 忘记密码不承诺产品内恢复，只能通过备份恢复或重置实例数据后重新初始化。
- 不做 2FA、TOTP、Passkey、硬件密钥、设备列表、安全通知或登录历史页面。
- 登录保护只记录失败限速 / 锁定所需的最小信息。

## AI Summary Output

- AI 摘要输出固定结构化 Markdown。
- 摘要正文固定使用简体中文。
- 必要英文术语、代码名、产品名和链接必须保留原文。
- MVP 不支持多语言摘要、跟随原文语言、用户自定义摘要语言或按订阅源配置摘要语言。

## AI Summary State Model

- 每个内容条目只保留一个当前 AI 摘要及其状态。
- 摘要状态使用 `none` / `pending` / `success` / `failed` / `insufficient_text`。
- 摘要记录至少包含当前摘要 Markdown、生成时间和错误原因。
- 搜索和 Obsidian 导出只使用当前摘要。
- 手动重试成功后覆盖旧失败或旧摘要。
- MVP 不保留摘要版本历史，不做旧摘要回看、多模型摘要对比或重新生成时保留旧版。

## Obsidian Export Boundary

- Obsidian 导出只生成单条内容条目的 Markdown 复制或下载内容。
- 导出不强制生成 AI 摘要。
- 导出不得因为 AI 服务不可用、摘要失败或可用文本不足而被阻止。
- 导出始终包含 `AI 摘要` 区块。
- 导出只包含 `available_text` 前 2000 个字符作为可用文本摘录。
- 可用文本摘录被截断时，必须追加“已截断，请打开原文链接查看完整内容”语义提示。
- MVP 不提供全文导出开关、可配置截断长度或按内容类型设置不同导出上限。
- `success` 状态写入当前摘要 Markdown。
- `none` / `pending` / `failed` / `insufficient_text` 状态分别写明未生成、正在生成、生成失败或可用文本不足。
- MVP 不做导出前强制生成摘要或导出时自动重试摘要。
- MVP 不做 Obsidian frontmatter、模板、插件、双链或 vault 自动同步。

## AI Summary Automation Boundary

- 手动 AI 摘要由 HTTP handler 触发，并复用 service 层摘要能力。
- 自动摘要由 `cmd/task` / `TaskServer` 触发，并复用同一套 service 层摘要能力。
- 自动摘要只对配置开启后新抓取入库的内容条目生效。
- 自动摘要只处理 `none` 状态的内容条目。
- 自动摘要不会自动覆盖 `success`，也不会自动重试 `failed` 或 `insufficient_text`。
- 手动摘要允许在 `none` / `failed` / `success` 状态触发，成功后覆盖当前摘要或错误。
- `pending` 状态下再次触发摘要时，返回正在生成。
- `insufficient_text` 在 MVP 中不可重试。
- 开启自动摘要配置时，不批量回填已有历史内容条目。
- 已有历史内容条目仍可由使用者手动触发摘要。
- 批量回填、按订阅源 / 文件夹 / 搜索结果补摘要、摘要任务队列进度后置。

## AI Service Configuration

- AI 服务由使用者配置 API Base URL、API Key 和模型名。
- MVP AI 服务协议只支持 OpenAI-compatible、非流式 Chat Completions。
- Claude、Gemini、Ollama、本地模型或其他服务必须通过 OpenAI-compatible endpoint / proxy 接入。
- MVP 不引入 provider 类型、原生多供应商适配器或流式响应处理。
- 保存 AI 服务配置时只做格式校验，不强制发起模型测试请求。
- 必须提供可选“测试配置”动作。
- 自动摘要开启前只要求配置字段存在，不要求最近测试成功。
- MVP 不做强制健康检查、定期探测或服务可用性门禁。
- API Key 保存于 SQLite 数据库并只在服务端使用。
- 前端保存后只显示已配置状态，不回显完整密钥。
- 导出内容、搜索结果、错误信息和日志不得包含 API Key。
- 数据库字段加密、外部 secret manager、环境变量式密钥管理、密钥轮换和密钥访问审计后置。

## Tag And Folder Assignment

- 创建标签 / 文件夹是独立动作，与分配解耦。
- 给内容条目分配标签时请求只接受已存在的 `tag_id`；给订阅源分配文件夹时请求只接受已存在的 `folder_id` 或置空。
- 标签名在标签范围内唯一，文件夹名在文件夹范围内唯一；重名创建返回冲突错误。
- 数据库必须对标签名、文件夹名建唯一约束。
- 分配时即时新建（输入新名字自动创建）后置。

## Tag And Folder Deletion

- 删除标签只删除标签本身及标签与内容条目的关联记录，不删除内容条目。
- 删除文件夹只删除文件夹本身，并把原属于该文件夹的订阅源 `folder_id` 置空，不删除订阅源或其内容条目。
- 与删除订阅源的内容条目及派生数据级联删除明确区分：订阅源是内容来源，标签 / 文件夹只是组织维度。
- 标签与内容条目的多对多关联表必须随标签删除清理；文件夹删除必须保证不残留指向已删除文件夹的 `folder_id`。

## Search Model

- MVP 基础搜索使用 SQLite FTS5。
- FTS 索引字段包括内容条目标题、订阅源名称、`available_text` 和当前 AI 摘要 Markdown。
- 结构化过滤仍使用普通表字段和关联表完成，包括内容类型、处理状态、内容标记、标签、文件夹和订阅源；每类过滤条件只接受单值，多个过滤条件之间使用 AND；MVP 不提供发布时间范围过滤、多选过滤、OR 条件组或保存搜索条件。
- API 查询参数只提供单值过滤：`content_type`、`processing_status`、`mark`、`tag_id`、`folder_id`、`feed_id`；未传表示不过滤。
- 内容收件箱与搜索页复用同一套内容查询 / 过滤语义；内容收件箱不传关键词，搜索页可传关键词。
- 内容收件箱、稍后处理、收藏、已完成都是同一内容条目列表查询的预设过滤视图：分别预设 `processing_status=unprocessed`、`mark=later`、`mark=favorite`、`processing_status=completed`，并允许在其上追加其他单值过滤。
- 订阅源详情是同一内容列表预设 `feed_id=X` 的过滤视图，复用同一套查询、分页、排序和条目操作。
- 有关键词时搜索结果默认按 FTS 相关性排序，相关性相同则按内容条目发布时间倒序。
- 无关键词、只有结构化过滤条件时，结果按内容条目发布时间倒序。
- 搜索结果必须遵守统一列表分页：`page + pageSize`，默认 `pageSize=20`，最大 `pageSize=100`。
- FTS 索引必须通过显式 migration 创建，并在内容条目标题 / `available_text`、订阅源名称和当前 AI 摘要变更时保持一致。
- MVP 不引入中文分词插件、拼音搜索、模糊纠错、搜索建议、向量语义搜索或问答式检索。

## Tradeoffs

- 采用 Nunu 可快速获得 Go 后端分层、DI、配置、日志和任务入口，但必须清理默认 MySQL / Redis 假设。
- 采用 SQLite 降低单人 VPS 部署复杂度，但后续多用户、高并发或横向扩展可能需要迁移到 PostgreSQL。
- 采用同一镜像双服务符合 Nunu `cmd/server` / `cmd/task` 分离结构，但比单进程容器多一个 Compose service。
- 采用显式版本化迁移比裸 AutoMigrate 更可控，但需要维护迁移文件和迁移顺序。
