## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

实现 HTML 净化单例与 `available_text` 纯文本生成，作为入库信任边界的可复用 `pkg` 能力。

端到端范围（纯 `pkg` 级，无 DB 依赖）：
- 单一全局 `bluemonday` 白名单策略（基于 `UGCPolicy` 适度收紧）作为 `pkg` 级单例，所有抓取共用。**禁止按来源 / 按站点适配净化规则**——白名单只声明允许输出的标签 / 属性，输入多样性不增加规则数量。
- `available_text` 生成：从净化后内容去除 HTML 标签并规范化空白，得到纯文本；优先级 `content` → `show_notes` → `description/summary` → `title`。
- 纯函数式 API，便于在抓取 service 与测试中复用。

## Acceptance criteria

- [ ] 提供 `pkg` 级 bluemonday 单例净化函数，剥除 `<script>` / 事件属性 / 危险标签
- [ ] 提供 `available_text` 生成函数，去标签 + 规范化空白，按 `content`→`show_notes`→`description/summary`→`title` 优先级取值
- [ ] 无任何按来源 / 按站点的分支适配逻辑
- [ ] 单元测试覆盖：恶意 HTML 剥除、各优先级回退、空白规范化、全空输入
- [ ] `go build ./...`、`go test ./...`（相关包）通过

## Blocked by

- #2 module 重命名 `shuliu` → `shiliu`
