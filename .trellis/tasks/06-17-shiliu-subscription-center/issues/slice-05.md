## Parent

#1 拾流订阅中心 MVP 后端实现 PRD

## What to build

落地主部署形态：Docker Compose 用**同一镜像**启动 `server` 与 `task` 两个长期运行服务，并共享同一个 SQLite 数据 volume；`migration` 作为一次性前置 job 在两者启动前执行。

端到端范围：
- 重写 `deploy/docker-compose/docker-compose.yml`：`server`(`cmd/server`) + `task`(`cmd/task`) 共享 SQLite volume，`migration`(`cmd/migration`) 作为前置一次性 job。
- 部署配置 `config/prod.yml` 补齐运行所需键：全局抓取间隔（关闭 / 30 / 60 / 360 / 1440 分钟，默认 60）、AI 服务相关配置占位键。
- 部署文档说明：备份 SQLite volume、TLS 由反向代理或平台提供（应用不内置证书管理）。

## Acceptance criteria

- [ ] `docker-compose.yml` 用同一镜像启动 `server` 与 `task`，共享 SQLite 数据 volume
- [ ] `migration` 作为一次性前置 job，先于 `server` / `task` 正常运行
- [ ] `config/prod.yml` 含抓取间隔（默认 60，可选 关闭/30/60/360/1440）与 AI 服务配置占位键
- [ ] 部署文档说明备份数据库 / volume，以及 TLS 由使用者自管反向代理或平台提供
- [ ] `docker compose -f deploy/docker-compose/docker-compose.yml build` 成功
- [ ] 用户故事 81-84 行为可在 compose 编排层验证

## Blocked by

- #3 数据层 SQLite-only 清理
- #5 golang-migrate 迁移机制
