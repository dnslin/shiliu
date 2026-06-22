# 响应 / 分页 / 路由契约对齐技术设计

## Scope

本设计覆盖 issue #4 / 切片03：响应 envelope、错误码骨架、分页公共结构与 `/api/v1` 路由前缀。它是后续 handler 的 API 基座，不包含任何业务实体 CRUD 或数据库迁移。

## First-Principles Reasoning

### 1. Challenge Assumptions

- 默认假设：应该重构 Nunu 响应模型为更“现代”的错误对象。未验证；会破坏父 PRD 明确要求的 envelope。
- 默认假设：分页辅助一定属于 repository。未验证；分页入参来自 HTTP query，而 `limit/offset` 是跨 SQL / FTS 的派生值。
- 默认假设：错误码范围可以等业务切片出现时再定。潜在错误；公开 API 一旦被前端依赖，后续改码会破坏调用方。
- 默认假设：只改 Gin group 即可完成路由前缀。未验证；Swagger runtime BasePath 也是对外契约。
- 默认假设：最小实现只要函数能编译。错误；本切片是后续所有 API 的复用基座，必须有行为测试守住边界。

### 2. Bedrock Truths

- JSON 客户端只能稳定依赖实际返回字段、HTTP status、路由路径和数值错误码。
- HTTP query 参数是字符串且可能缺失、非法、为负数或过大；服务端必须在边界归一化。
- SQL / SQLite / FTS 的分页执行需要非负 `offset` 和正数 `limit`。
- `page` 从 1 开始时，数学换算唯一：`offset=(page-1)*pageSize`，`limit=pageSize`。
- `total=0` 仍是一个有效列表状态；响应必须携带分页元数据，不能让前端猜测。
- 当前仓库已有 `api/v1` 作为 API contract 包；把跨 handler 的响应 / 分页结构放在这里可减少重复解析和重复 payload 定义。
- 父 PRD 明确规定：保留 Nunu `{code,message,data}`，API 前缀 `/api/v1`，默认 pageSize 20，最大 100。

### 3. Rebuild From Ground Up

- 从父 PRD 的公开合同出发，不重写 `Response`，只让现有 `HandleSuccess` / `HandleError` 的行为更可测试、更稳定。
- 在 HTTP 边界解析 `page` / `pageSize`，把异常输入归一为产品允许的最接近值，保证后续层只看到有效分页对象。
- 用一个分页请求类型承载 `page` / `pageSize`，用方法导出 `LimitOffset()`，使 SQL / FTS 层拿到唯一换算结果。
- 用一个分页响应类型承载 `items` 与 `page`，让后续所有列表 handler 复用同一 JSON 形状。
- 在 `api/v1` 注册错误码骨架，后续切片只追加具体业务错误，不重新发明错误码归属。
- 在 server 初始化同时更新 Gin group 和 Swagger BasePath，保证运行时路由与开发辅助文档一致。

### 4. Contrast With Convention

常规脚手架思路可能只局部修改 `s.Group("/api/v1")`，分页等到第一个列表 API 再写，错误码等到第一个业务失败再加。这样短期代码少，但会让多个后续切片各自定义分页 payload、错误码命名和边界处理。本设计选择先固化公共契约，因为这些是所有列表和错误响应共享的不可逆 API 面。

### 5. Conclusion

本切片应作为“API contract package”建设，而不是业务功能实现：保留既有 envelope，新增少量深模块式分页 / 错误码工具，并用测试锁定边界行为。

## Architecture and Boundaries

### API contract package: `api/v1`

`api/v1` 继续作为公开请求 / 响应类型的归属位置：

- `Response`：保持 `{code,message,data}` JSON 结构。
- `HandleSuccess`：成功响应出口。
- `HandleError`：失败响应出口。
- Error variables：集中注册 `errorCodeMap`。
- Pagination types：新增分页请求、分页响应与换算辅助。

这样后续 handler 只引用 `v1.ParsePagination(ctx)` / `v1.NewPageData(...)`，不在各自文件里重复 query 解析和 JSON shape。

### HTTP server boundary: `internal/server/http.go`

- Swagger runtime BasePath 改为 `/api/v1`。
- Gin API group 改为 `s.Group("/api/v1")`。
- 根路径 `/` 和 `/swagger/*any` 保持不变；它们不是业务 API group。

## Proposed Contracts

### Response envelope

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

- success: HTTP 200, `code=0`, `message="ok"`。
- registered business error: caller-provided HTTP status, registered numeric code, `err.Error()` message。
- unknown error: caller-provided HTTP status, `code=500`, `message="unknown error"`。

### Pagination request

Recommended Go shape:

```go
const (
    DefaultPage     = 1
    DefaultPageSize = 20
    MaxPageSize     = 100
)

type PageRequest struct {
    Page     int `form:"page" json:"page"`
    PageSize int `form:"pageSize" json:"pageSize"`
}
```

Behavior:

- missing `page` → `1`
- missing `pageSize` → `20`
- `page < 1` → `1`
- `pageSize < 1` → `20`
- `pageSize > 100` → `100`
- malformed query values should not crash; parsing should fall back to defaults for the malformed field.

`LimitOffset()` returns:

- `limit = PageSize`
- `offset = (Page - 1) * PageSize`

### Pagination response

Recommended Go shape:

```go
type PageMeta struct {
    Page     int   `json:"page"`
    PageSize int   `json:"pageSize"`
    Total    int64 `json:"total"`
}

type PageData struct {
    Items interface{} `json:"items"`
    Page  PageMeta    `json:"page"`
}
```

`NewPageData(items, pageRequest, total)` should preserve empty-but-valid list results:

```json
{
  "items": [],
  "page": {"page": 1, "pageSize": 20, "total": 0}
}
```

### Error code skeleton

Recommended numeric ranges:

- `0`: success.
- `400` / `401` / `404` / `500`: common HTTP-shaped template errors retained for compatibility.
- `1000-1999`: user account / auth.
- `2000-2999`: feed / subscription source.
- `3000-3999`: content item / inbox / search.
- `4000-4999`: tag / folder organization.
- `5000-5999`: AI summary / AI service configuration.
- `6000-6999`: export / import.

Recommended initial variables keep concrete names even when only used later, for example:

- auth: `ErrAccountNotInitialized`, `ErrAccountAlreadyInitialized`, `ErrInvalidCredentials`, `ErrAccountLocked`.
- feed: `ErrFeedInvalidURL`, `ErrFeedFetchFailed`, `ErrFeedParseFailed`, `ErrFeedAlreadyExists`, `ErrFeedFetchInProgress`.
- content: `ErrContentItemNotFound`, `ErrInvalidContentFilter`.
- tag/folder: `ErrTagAlreadyExists`, `ErrTagNotFound`, `ErrFolderAlreadyExists`, `ErrFolderNotFound`.
- AI: `ErrAIConfigMissing`, `ErrAISummaryInProgress`, `ErrAIInsufficientText`, `ErrAISummaryFailed`.
- export/import: `ErrExportFailed`, `ErrOPMLInvalid`, `ErrOPMLImportFailed`.

Existing `ErrEmailAlreadyUse=1001` is Nunu template residue. This slice should preserve compatibility unless it is unused and removal is proven safe. Later auth/user-account slices can replace email-specific naming when removing template email account behavior.

## Data Flow

### List request flow

```text
HTTP query string → api/v1 pagination parser → normalized PageRequest → service/repository limit+offset → handler PageData → HandleSuccess envelope
```

Responsibilities:

- Handler/API boundary owns query parsing and normalization.
- Repository/search code receives only `limit` and `offset` or a validated pagination object.
- Response construction owns `items` + page metadata shape.

### Error flow

```text
service/repository error → handler maps HTTP status + v1 error → HandleError → Response envelope
```

Responsibilities:

- `api/v1` owns error variable registration and numeric codes.
- Handler owns HTTP status selection.
- Service owns business decision, not response serialization.

### Route flow

```text
server bootstrap → docs.SwaggerInfo.BasePath=/api/v1 → Gin group /api/v1 → router.InitUserRouter
```

## Compatibility Notes

- Keeping `Response` shape avoids breaking existing handler tests and future frontend assumptions.
- Changing `/v1` to `/api/v1` intentionally breaks old business prefix per parent PRD. No compatibility alias is planned in this slice because MVP has no stable external client yet, and accepting both prefixes would weaken the contract.
- Existing route annotations like `@Router /login [post]` can remain path-relative to Swagger BasePath. Regenerating docs is not required unless build/test reveals generated docs must change.
- Existing root `/` health/demo route remains outside `/api/v1` and can continue returning success envelope.

## TDD Strategy

Tests should verify public behavior and stable helper contracts, not internal implementation details.

1. RED: add pagination tests in `api/v1` for defaults, boundaries, `LimitOffset`, and `total=0` page data.
2. GREEN: implement pagination types and helpers.
3. RED: add response/error tests using `httptest`/Gin test context for success, registered error, unknown error.
4. GREEN: adjust `HandleSuccess` / `HandleError` only if behavior deviates.
5. RED: add server route prefix behavior test if current test seams can instantiate `NewHTTPServer` safely; otherwise test via focused router setup around `internal/server/http.go` contract.
6. GREEN: update `/api/v1` group and Swagger BasePath.

## Risks and Rollback

- `internal/server/http.go` route prefix changes will make old `/v1/*` tests fail; tests should be updated to assert `/api/v1/*` as the new contract.
- Adding many named errors can look like unused code, but exported package-level variables are acceptable API surface for upcoming slices.
- If Swagger generated files drift, prefer minimal runtime BasePath change first; regenerate docs only if necessary for correctness.
- Pagination helpers must avoid generic over-engineering; Go 1.24 supports generics, but `interface{}` keeps consistency with current `Response.Data` and avoids forcing all handlers into generic response types.
