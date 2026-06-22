## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现 AI 服务配置的存储与管理，供手动 / 自动摘要共用。

端到端贯穿 migration + repository + service + handler + 测试：
- 迁移：AI 服务配置表（API Base URL、模型名、server-only API Key）。
- 保存配置：只做格式校验，不强制发起模型测试请求即可保存。
- 可选"测试配置"动作：按需主动发起一次连通性验证（复用 #24 的客户端边界或先以接口占位）。
- 安全红线：API Key 只存服务端 SQLite、只在服务端使用；读取配置时前端只得到"已配置"状态，**不回显完整密钥**；导出 / 搜索 / 错误信息 / 日志均不得包含 API Key。

## Acceptance criteria

- [ ] AI 服务配置表迁移建立（base url / model / api key），up/down 双向可执行
- [ ] 保存配置只做格式校验、不强制测试成功
- [ ] 提供可选"测试配置"动作
- [ ] 读取配置不回显完整 API Key，只返回"已配置"状态
- [ ] 错误信息与日志中不出现 API Key（有测试断言守护）
- [ ] handler 用 httpexpect 断言保存 / 读取 / 测试动作与密钥不回显
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #5 golang-migrate 迁移机制
- #4 响应 / 分页 / 路由对齐
