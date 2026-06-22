## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

对齐后端 API 的响应、分页与路由契约，作为后续所有 handler 的统一基座。

端到端范围：
- **保留** Nunu 模板的统一响应结构 `{ "code": <int>, "message": "...", "data": ... }`，不改写。成功响应 HTTP 200 + `code=0`；失败由 `HandleError` 返回业务 `code` / `message` + 合适的 HTTP status。
- 在 `api/v1` 的 `errorCodeMap` 注册拾流业务错误码的骨架（鉴权 / 订阅源 / 内容 / 标签文件夹 / 摘要等分段预留），后续切片按需补充具体码值。
- 提供统一分页辅助结构：请求侧 `page + pageSize`（`page` 从 1 开始，默认 `pageSize=20`、最大 100），响应侧 `data` 内含 `items` 和 `page:{page,pageSize,total}`；提供把 `page + pageSize` 换算为 SQL `limit + offset` 的公共辅助。
- 路由前缀从 `/v1` 改为 `/api/v1`（`internal/server/http.go`，含 swagger BasePath）。

## Acceptance criteria

- [ ] 响应结构保持 `{code,message,data}`，成功 `code=0` + HTTP 200，失败业务 `code`/`message` + 对应 HTTP status
- [ ] `errorCodeMap` 已注册拾流业务错误码骨架（可分段预留）
- [ ] 提供 `page+pageSize` 请求解析（默认 20、最大 100、page 从 1）与 `page:{page,pageSize,total}` 响应结构
- [ ] 提供 `page+pageSize` → `limit+offset` 的公共换算辅助，并有单元测试覆盖边界（page=1、超过 max、total=0）
- [ ] 所有路由挂载在 `/api/v1` 前缀下
- [ ] `go build ./...`、`go vet ./...` 通过

## Blocked by

- #2 module 重命名 `shuliu` → `shiliu`
