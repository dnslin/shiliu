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
- 成功响应统一为 `{ "data": ... }`。
- 失败响应统一为 `{ "error": { "code": "...", "message": "...", "details": ... } }`。
- HTTP status 表达错误类别。
- 分页列表在 `data` 内包含 `items` 和 `page`。
- 列表 API 使用 `limit + offset` 分页。
- 默认 `limit=20`，最大 `limit=100`。
- `page` 返回 `limit`、`offset`、`total`。
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

- 内容条目只在同一订阅源内去重。
- 每个内容条目保存 `feed_id` 和 `dedupe_key`。
- `dedupe_key` 生成优先级为 feed item `guid` → `link` → `title + published_at` 稳定哈希。
- 数据库必须对 `(feed_id, dedupe_key)` 建唯一约束。
- 音频 enclosure URL 是音频字段，不作为主去重键。
- 跨订阅源重复、相似内容合并和 canonical URL 归一化后置。

## Content Text Model

- 内容条目保留原始 feed 文本字段：`title`、`description`、`content`、`show_notes`。
- 后端生成规范化 `available_text`。
- `available_text` 生成优先级为 `content` → `show_notes` → `description/summary` → `title`。
- `available_text` 作为基础搜索、AI 摘要和 Obsidian 导出的统一输入。
- 不抓原网页全文。
- 不做正文抽取。
- 不只保存单一 `content` 字段而丢失来源语义。

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

## AI Service Configuration

- AI 服务由使用者配置 API Base URL、API Key 和模型名。
- API Key 保存于 SQLite 数据库并只在服务端使用。
- 前端保存后只显示已配置状态，不回显完整密钥。
- 导出内容、搜索结果、错误信息和日志不得包含 API Key。
- 数据库字段加密、外部 secret manager、环境变量式密钥管理、密钥轮换和密钥访问审计后置。

## Tradeoffs

- 采用 Nunu 可快速获得 Go 后端分层、DI、配置、日志和任务入口，但必须清理默认 MySQL / Redis 假设。
- 采用 SQLite 降低单人 VPS 部署复杂度，但后续多用户、高并发或横向扩展可能需要迁移到 PostgreSQL。
- 采用同一镜像双服务符合 Nunu `cmd/server` / `cmd/task` 分离结构，但比单进程容器多一个 Compose service。
- 采用显式版本化迁移比裸 AutoMigrate 更可控，但需要维护迁移文件和迁移顺序。
