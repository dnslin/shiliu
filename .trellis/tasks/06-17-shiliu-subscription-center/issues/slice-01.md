## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

把 Go module 名从 `shuliu` 统一重命名为 `shiliu`。这是面向后续所有切片的机械性地基改动：改 `go.mod` 的 module 行，全仓替换 import 路径 `shuliu/...` → `shiliu/...`，重新生成 Wire 生成代码（`wire_gen.go`），并确保整仓可编译。作为独立提交，便于单独回滚。

不在本切片引入任何业务逻辑或结构变化，纯重命名。

## Acceptance criteria

- [ ] `go.mod` 的 module 行为 `module shiliu`
- [ ] 全仓不再存在 `shuliu/` import 路径（含测试文件 `test/...`）
- [ ] `cmd/server`、`cmd/task`、`cmd/migration` 三处 Wire 生成代码已重新生成且引用 `shiliu/...`
- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] 现有测试包可编译（运行结果不在本切片范围内保证）

## Blocked by

None - can start immediately
