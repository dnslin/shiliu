# [切片06] User 模型重构 + users 迁移 + repository

## Goal

把 Nunu 模板用户模型替换为拾流单实例产品的最小用户账户持久化地基：`users` 表、`model.User` 和 `UserRepository` 都只表达后续首次初始化、登录保护和改密码所需的鉴权数据。

User value: 拾流的唯一用户账户不应携带邮箱、昵称、profile 或软删除等 SaaS/模板语义。持久层先收敛到明确的鉴权数据，后续初始化、登录、锁定和改密码切片才能基于同一 SQLite schema 实现。

## Source Requirements

- GitHub issue: https://github.com/dnslin/shiliu/issues/7
- Parent PRD: https://github.com/dnslin/shiliu/issues/1
- Local source slice: `.trellis/tasks/06-17-shiliu-subscription-center/issues/slice-06.md`
- Parent task: `.trellis/tasks/06-17-shiliu-subscription-center/`
- Blocker already completed: issue #5 / archived task `.trellis/tasks/archive/2026-06/06-23-golang-migrate-migration-mechanism/`

## Confirmed Facts from Repository Inspection

- `internal/model/user.go` still has Nunu fields: `UserId`, `Nickname`, `Password`, `Email`, and `gorm.DeletedAt` soft delete.
- `internal/repository/user.go` still exposes `GetByEmail` and queries `user_id` / `email` columns.
- `test/server/repository/user_test.go` currently uses real SQLite but still calls `AutoMigrate(&model.User{})`; this must move to checked-in migrations for this slice.
- Existing migration mechanism uses `internal/migration.Run` with `golang-migrate`, checked-in SQL files under `migrations/`, and public `cmd/migration -direction up|down` behavior.
- Current migrations only contain `000001_baseline`; `users` should be the next business-schema migration.
- Parent PRD/design require a single user account using `username + password`; email, nickname, profile, and soft delete are explicitly out of MVP.
- Parent plan assigns first initialization to #8, login / JWT / login locking to #9, and password change to #10. This slice only creates the storage/model/repository foundation.
- Existing service/API/JWT code still references old `UserId` / `Email` / `Nickname` / `Password` names. Implementation must remove stale compile-time references or adapt them to the new vocabulary, but must not complete the later initialization/login/password-change behavior in this slice.

## Requirements

1. `internal/model.User` must contain only these persisted authentication fields:
   - `id`
   - `username`
   - `password_hash`
   - failed login count
   - lock expiration time
   - `created_at`
   - `updated_at`
2. Remove Nunu template user semantics from the model and active tests/code paths:
   - no `UserId` field separate from `id`
   - no `Nickname`
   - no `Email`
   - no raw `Password` field
   - no `DeletedAt` soft delete field
3. Add paired `golang-migrate` SQL files for the `users` table after the baseline migration.
4. The `users` table migration must be reversible and must enforce unique `username` values.
5. `UserRepository` must support behavior required by this slice:
   - create a user account record;
   - fetch by `username`;
   - update `password_hash`, failed login count, and lock expiration time.
6. Fetching a missing username should be a non-exceptional repository result (`nil, nil`) so later auth code can map both missing users and wrong passwords to the same invalid-credentials response.
7. Repository tests must use a temporary SQLite database plus checked-in migrations, not Gorm `AutoMigrate`, go-sqlmock, MySQL SQL-string assertions, or any non-SQLite dialect.
8. Tests must verify observable persistence behavior through repository public methods and real SQLite constraints:
   - create then fetch by username;
   - missing username returns no user and no error;
   - duplicate username violates the unique constraint;
   - update persists `password_hash`, failed login count, and lock expiration time;
   - migration up creates `users`, and down rolls back the latest migration boundary.
9. Existing repository tests must be migrated or replaced so they no longer mention deleted fields.
10. Keep downstream code compiling without preserving stale repository/model contracts. Any service/API compatibility edits must be limited to vocabulary/schema alignment and compilation, not full implementation of #8/#9/#10 behavior.
11. Follow TDD as vertical slices: write one behavior-level failing test, implement that behavior, make it pass, then proceed. “不允许最小化实现” means no placeholder/stub or deliberately under-scoped persistence; it does not override the TDD rule that each green step should be only the code needed for the current behavior.

## Acceptance Criteria

- [ ] `internal/model.User` only contains `id`, `username`, `password_hash`, failed login count, lock expiration time, `created_at`, and `updated_at` fields.
- [ ] Active code/tests no longer reference deleted `UserId`, `Nickname`, `Email`, `Password`, or `DeletedAt` model fields.
- [ ] `migrations/000002_users.up.sql` and `migrations/000002_users.down.sql` exist as a reversible pair.
- [ ] Migrating up from a fresh SQLite database creates `users` with a unique `username` constraint.
- [ ] Migrating down after all migrations rolls back the `users` migration boundary without removing the baseline boundary.
- [ ] `UserRepository` exposes and implements create, get-by-username, and update behavior for the new model.
- [ ] Repository integration tests run against real temporary SQLite after checked-in migrations.
- [ ] Repository tests assert duplicate `username` behavior through the real SQLite unique constraint.
- [ ] Repository tests assert update behavior for `password_hash`, failed login count, and lock expiration time.
- [ ] `test/server/repository/user_test.go` no longer uses `AutoMigrate`, go-sqlmock, MySQL dialect assumptions, or deleted model fields.
- [ ] Focused tests for the repository and migration behavior pass.
- [ ] `go test ./...` passes or any non-slice failure is captured with exact output and a justified follow-up.
- [ ] `go build ./...` passes.

## Out of Scope

- First-time initialization endpoint, initialized-state checks, and blocking second account creation (#8).
- Login endpoint behavior, password length validation, bcrypt cost 12, 30-day JWT issuance, failed-login lock state transitions, and success-reset logic (#9).
- Password-change endpoint or service behavior (#10).
- Frontend behavior, settings UI, or route/navigation changes.
- Multi-user accounts, email/profile fields, password recovery, refresh tokens, session tables, IP/device rate limiting, 2FA, or audit logs.
- Adding non-SQLite database support or reintroducing AutoMigrate as production schema management.

## Open Questions

None blocking. Repository-answerable questions have been resolved by inspecting issue #7, parent PRD/design, migration specs, and current code. Technical naming defaults are recorded in `design.md` and may be adjusted only if implementation evidence shows a simpler compile-safe shape.
