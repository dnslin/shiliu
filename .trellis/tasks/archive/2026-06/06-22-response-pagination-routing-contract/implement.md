# 响应 / 分页 / 路由契约对齐实现计划

## Preconditions

- 当前分支：`issue-4-response-pagination-routing-contract`。
- Trellis task：`.trellis/tasks/06-22-response-pagination-routing-contract`。
- GitHub issue：#4。
- 父 PRD / design 已确认 `/api/v1`、Nunu envelope、分页 `page + pageSize`。
- 当前代码：
  - `api/v1/v1.go` 有 `Response`、`HandleSuccess`、`HandleError`。
  - `api/v1/errors.go` 有 Nunu 示例错误注册。
  - `internal/server/http.go` 仍使用 `/v1` BasePath 和 group。

## TDD Execution Checklist

> 采用纵向 tracer bullet，不一次性写完所有测试。每一步保持 RED → GREEN，再进入下一步。

### 1. Pagination helper tracer bullet

- [x] RED：新增 `api/v1` 分页测试，先覆盖默认 query：未传 `page/pageSize` 得到 `page=1,pageSize=20,limit=20,offset=0`。
- [x] GREEN：新增分页请求类型、默认值常量、解析函数、`LimitOffset()`。
- [x] RED：追加边界测试：`page=1`、`page<1`、`pageSize<1`、`pageSize>100`。
- [x] GREEN：完善 normalization。
- [x] RED：追加 malformed query 测试：非法字符串不 panic，对该字段回退默认值。
- [x] GREEN：使用安全解析路径，不让 `ShouldBindQuery` 错误中断分页默认化。

### 2. Pagination response data

- [x] RED：测试 `NewPageData` 对非空 items 返回 `data.items` 与 `data.page`。
- [x] GREEN：实现 `PageMeta`、`PageData`、`NewPageData`。
- [x] RED：测试 `total=0` 与空 items 仍保留 `page:{page,pageSize,total}`。
- [x] GREEN：确保空列表不会被替换成 nil/空对象。

### 3. Response / error envelope behavior

- [x] RED：测试 `HandleSuccess(nil)` 返回 HTTP 200、`code=0`、`message="ok"`、`data` 为对象。
- [x] GREEN：如现有行为已满足则不改；否则最小调整。
- [x] RED：测试 `HandleError` 对已注册错误保留业务 `code/message` 与传入 HTTP status。
- [x] GREEN：如现有行为已满足则不改；否则调整。
- [x] RED：测试未注册错误返回 `code=500,message="unknown error"` 与传入 HTTP status。
- [x] GREEN：如现有行为已满足则不改；否则调整。

### 4. Error code skeleton

- [x] RED：测试关键拾流错误变量在 `HandleError` 中映射到预期业务 code/message。
- [x] GREEN：在 `api/v1/errors.go` 注册分段错误码骨架。
- [x] 保留已有 common errors；处理 `ErrEmailAlreadyUse` 时避免破坏当前 user service/handler 测试。

### 5. Route prefix contract

- [x] RED：新增/更新 server 或 router 行为测试，证明 user routes 挂在 `/api/v1` 下。
- [x] GREEN：修改 `internal/server/http.go`：`docs.SwaggerInfo.BasePath = "/api/v1"`，`s.Group("/api/v1")`。
- [x] 如现有 handler 测试直接注册裸 `/register`，不强制改为 server-level 测试；它们测试 handler seam，不测试 server prefix。

### 6. Full validation

- [x] `go test ./...`
- [x] `go build ./...`
- [x] `go vet ./...`

## Expected Files to Change

- `api/v1/v1.go`：分页类型 / helper；必要时微调 response helpers。
- `api/v1/errors.go`：拾流业务错误码骨架。
- `api/v1/*_test.go`：分页、response/error behavior tests。
- `internal/server/http.go`：BasePath 和 API group 改为 `/api/v1`。
- 可能新增 `internal/server/*_test.go` 或现有 server/router 测试：路由前缀行为。

## Validation Details

### Requirement-driven test scenarios

- Happy path：默认分页、成功 envelope、已注册错误 envelope、`/api/v1` route。
- Edge cases：`page=1`、`page<1`、`pageSize<1`、`pageSize>100`、`total=0`、malformed query。
- Error handling：未注册 error 走 unknown envelope；HTTP status 不被 helper 覆盖。
- State transitions：本切片无状态机。

### Commands

```bash
go test ./...
go build ./...
go vet ./...
```

## Risky Files / Rollback Points

- `api/v1/v1.go`：公开 API contract；只增加 helper，避免改 envelope 字段名。
- `api/v1/errors.go`：错误码一旦被前端依赖应保持稳定；本次只加骨架，不重新定义所有未来错误。
- `internal/server/http.go`：路由前缀变更影响前端和 Swagger；变更必须由测试覆盖。
- `docs/docs.go`：生成文件默认不手改；除非 Swagger runtime contract 需要，否则不纳入本切片。

## Follow-up Before `task.py start`

- [x] 使用者确认错误码分段数值范围：采用 `1000` 段递增规则。
- [x] 规划材料经使用者批准。
- [x] 已运行 `python ./.trellis/scripts/task.py start .trellis/tasks/06-22-response-pagination-routing-contract`，任务状态进入 `in_progress`。
