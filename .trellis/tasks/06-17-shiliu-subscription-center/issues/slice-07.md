## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现**首次初始化**：单实例第一次被访问时创建唯一用户账户，创建成功后永久关闭创建入口。

端到端贯穿 service + handler + 测试：
- 提供查询"是否已初始化"的能力（前端据此决定展示初始化页还是登录页）。
- 初始化接口：仅当尚无用户账户时接受 `username + password` 创建唯一账户；密码至少 12 字符（不强制大小写 / 数字 / 符号组合），bcrypt cost 12 存哈希。
- 账户创建成功后，再次调用初始化接口必须被拒绝（入口永久关闭），不得创建第二个账户。

## Acceptance criteria

- [ ] 提供"是否已初始化"查询能力
- [ ] 无账户时可用 `username + password` 创建唯一账户
- [ ] 密码 <12 字符被拒绝；密码以 bcrypt cost 12 存储
- [ ] 已存在账户后调用初始化接口返回冲突 / 拒绝，且不创建第二个账户
- [ ] service 层用 gomock repository 覆盖"已初始化则拒绝"与"未初始化则创建"两条路径
- [ ] handler 层用 httpexpect 断言 `{code,message,data}` envelope 与状态
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #7 User 模型重构 + users 迁移 + repository
- #4 响应 / 分页 / 路由对齐
