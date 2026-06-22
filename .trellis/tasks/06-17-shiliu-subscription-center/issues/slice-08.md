## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现登录、JWT 会话、业务路由保护与登录失败锁定。

端到端贯穿 service + handler + middleware + 测试：
- 登录：校验 `username + password`（bcrypt 比对），成功签发 30 天有效的 JWT access token，前端通过 `Authorization: Bearer <token>` 调用 API；JWT 签名密钥读自 yml 配置。
- 登录失败锁定：连续 5 次失败锁定 15 分钟，期间拒绝登录；一次成功登录清空失败计数。只按唯一用户账户记录失败次数与锁定到期时间。
- JWT 中间件：保护所有业务路由，未携带 / 无效 / 过期 token 返回鉴权错误。

## Acceptance criteria

- [ ] 正确凭据登录返回 30 天有效 JWT；错误凭据返回鉴权错误
- [ ] 连续 5 次失败后锁定 15 分钟，锁定期内即使密码正确也拒绝
- [ ] 一次成功登录后失败计数清零
- [ ] JWT 中间件保护业务路由：缺失 / 无效 / 过期 token 被拒
- [ ] service 层用 gomock 覆盖：成功、密码错误、用户不存在、锁定中、计数清零
- [ ] handler 层用 httpexpect 断言登录响应 `data` 含 token、受保护路由的鉴权行为
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #7 User 模型重构 + users 迁移 + repository
- #4 响应 / 分页 / 路由对齐
