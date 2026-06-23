# 响应 / 分页 / 路由契约对齐

## Goal

对齐拾流 MVP 后端的 REST JSON 基座，让后续 handler 在同一响应结构、错误码注册、分页结构和路由前缀上开发，避免每个业务切片重复定义 API 契约。

## User Value

- 使用者和前端调用方获得稳定的 `/api/v1` API 入口。
- 所有成功与失败响应都沿用一致 `{code,message,data}` 结构，便于前端统一处理。
- 后续内容列表、搜索、订阅源、标签、文件夹和摘要接口可复用同一分页结构，避免分页语义漂移。

## Confirmed Facts

- GitHub issue：#4 `[切片03] 响应 / 分页 / 路由契约对齐`。
- 父 issue：#1 `拾流订阅中心 MVP 后端实现 PRD`。
- 已满足 blocker #2：当前 `go.mod` module 为 `shiliu`。
- 父 PRD 要求后端 API 统一挂载在 `/api/v1`，采用 REST JSON，不做 GraphQL、tRPC 或多版本 API。
- 父 PRD / design 要求沿用 Nunu 模板统一响应结构 `{ "code": <int>, "message": "...", "data": ... }`，成功 HTTP 200 + `code=0`。
- 失败响应由 `HandleError` 返回业务 `code` / `message`，HTTP status 表达错误类别。
- 错误码集中在 `api/v1` 的 `errorCodeMap` 注册，本切片只建立拾流业务错误码骨架和已有模板错误的兼容边界。
- 列表分页采用 `page + pageSize`，`page` 从 1 开始，默认 `pageSize=20`，最大 `pageSize=100`。
- 分页响应要求 `data` 内含 `items` 和 `page:{page,pageSize,total}`。
- 当前代码证据：
  - `api/v1/v1.go` 已有 `Response`、`HandleSuccess`、`HandleError`、`errorCodeMap` 和 `newError`。
  - `api/v1/errors.go` 已有 Nunu 示例错误：success、400、401、404、500、`ErrEmailAlreadyUse=1001`。
  - `internal/server/http.go` 当前设置 `docs.SwaggerInfo.BasePath = "/v1"` 且路由 group 为 `s.Group("/v1")`。
  - 现有 handler 测试 seam 在 `test/server/handler`，使用 `httpexpect` 断言 HTTP JSON 行为。

## Requirements

- 保留 `api/v1.Response` 的 JSON envelope：`code`、`message`、`data`。
- 成功响应必须继续通过 `HandleSuccess` 返回 HTTP 200、`code=0`、`message="ok"`，且 `data` 为空时仍返回对象而不是省略字段。
- 失败响应必须继续通过 `HandleError` 返回调用方指定 HTTP status；已注册错误返回对应业务 `code` 和错误消息；未注册错误继续落入明确的 unknown error 响应。
- `api/v1` 必须注册拾流业务错误码骨架，至少为以下领域预留命名段：鉴权 / 用户账户、订阅源、内容条目、标签 / 文件夹、AI 摘要 / AI 服务、导出 / 导入。
- 错误码骨架不得强行实现后续业务逻辑；只提供后续切片可引用的错误变量和分段约定。
- 必须提供可复用分页请求结构，支持从 Gin query 解析 `page` 与 `pageSize`。
- 分页解析必须满足：未传时 `page=1,pageSize=20`；`page<1` 归一为 1；`pageSize<1` 归一为 20；`pageSize>100` 截断为 100。
- 必须提供 `page + pageSize` 到 SQL / FTS 查询所需 `limit + offset` 的公共换算辅助。
- 必须提供分页响应结构：`items` + `page:{page,pageSize,total}`；`total=0` 时仍返回 page 元数据。
- HTTP API group 必须从 `/v1` 改为 `/api/v1`。
- Swagger runtime BasePath 必须同步为 `/api/v1`。
- 本切片不得重写整体 Nunu 响应模型、不得引入 cursor pagination、不得新增业务 handler。

## Acceptance Criteria

- [x] `HandleSuccess` 的成功 envelope 保持 `{code,message,data}`，HTTP 200，`code=0`，`message="ok"`。
- [x] `HandleError` 对已注册错误返回业务 `code` / `message` 和调用方传入 HTTP status；对未注册错误返回明确 unknown error envelope。
- [x] `api/v1` 注册拾流业务错误码骨架，并保留或兼容已有模板错误码。
- [x] 提供 `page+pageSize` 请求解析，默认 `pageSize=20`、最大 100、`page` 从 1 开始。
- [x] 提供 `page+pageSize` → `limit+offset` 的公共换算辅助。
- [x] 提供 `data.items` + `data.page:{page,pageSize,total}` 分页响应结构。
- [x] 单元测试覆盖分页边界：`page=1`、缺省值、`page<1`、`pageSize<1`、超过 max、`total=0`。
- [x] HTTP 路由挂载在 `/api/v1` 前缀下，旧 `/v1` 业务前缀不再作为本切片 API group。
- [x] Swagger `BasePath` 为 `/api/v1`。
- [x] `go test ./...`、`go build ./...`、`go vet ./...` 通过。

## Out of Scope

- 不实现订阅源、内容列表、搜索、标签 / 文件夹、AI 摘要或 Obsidian 导出的业务接口。
- 不改数据库 schema，不引入 migration。
- 不改变 JWT 中间件语义或用户账户模型。
- 不引入 cursor pagination、GraphQL、tRPC、多版本 API 或前端代码。
- 不重生成 Swagger 文档作为必要验收；本切片只要求 runtime `docs.SwaggerInfo.BasePath` 与 server group 对齐，除非实现过程中发现生成文档必须同步才能 build/test 通过。

## Decisions

- 错误码骨架采用 `1000` 段递增规则：鉴权 / 用户账户 `1000-1999`，订阅源 `2000-2999`，内容条目 / 收件箱 / 搜索 `3000-3999`，标签 / 文件夹 `4000-4999`，AI 摘要 / AI 服务 `5000-5999`，导出 / 导入 `6000-6999`。

## Open Questions

- 无。
