# P0 企业身份与项目访问验收记录

日期：2026-10-12

## 已落地合同

- OIDC Authorization Code + PKCE，服务端校验 state、短期浏览器绑定、nonce、issuer、签名、audience 与 azp。
- PostgreSQL 一次性登录尝试和摘要会话；浏览器只持有 HttpOnly Cookie，session 响应返回内存 CSRF token。
- 管理 CLI 预置精确 `(issuer, subject)`、稳定用户 UUID、项目与 `viewer|analyst` 角色，并提供全局会话撤销、用户禁用和成员撤销；变更与管理审计同事务提交，重复执行幂等。
- 命令写入和查询在 `READ COMMITTED` 事务内对活动成员关系 `FOR SHARE`；撤销保留同一行并递增版本。
- 同项目其他成员查询和幂等探测统一返回未找到，不泄露原 actor 的命令参数或存在性。
- 开发合成 issuer 实现 discovery、JWKS、一次性授权码、PKCE、错误 claim、未发布签名密钥和 JWKS 换钥；测试不访问真实企业 IdP、ARTEX 或外部目标。
- 前端命令缓存键包含 actor、project 和 command；退出、401、项目 403 与项目切换按范围清理缓存，项目切换重新挂载工作台局部状态。
- Playwright 身份流程关闭会保存请求头和 Cookie 的 trace；管理员子进程错误与 Cookie 断言不向失败日志回显 DSN 或会话原值。

## 本次已通过

```text
python scripts/check.py --integration --e2e
  文本格式：90 个文件
  gofmt / goimports：无漂移
  sqlc / OpenAPI 生成：无漂移
  go mod verify / go vet：通过
  go test -count=1 ./...：通过
  go test -tags=integration -count=1 ./...：真实 PostgreSQL 测试通过
  Prettier / ESLint / TypeScript：通过
  Vitest：5 个文件，31 个测试
  Vite production build：通过
  Playwright Chromium：6 个测试通过

python scripts/verify_restart.py：完整 OIDC 登录、Cookie/CSRF、API 重启和回执恢复通过
pnpm --dir web peers check：通过
git diff --check：通过
```

Go 单元与 HTTP 测试覆盖配置模式、生产 `__Host-`/Secure Cookie、重复 CSRF/登录绑定 Cookie、OIDC error callback、bootstrap 资料 schema、密钥域分离、PKCE 加密、azp/audience、公开错误、callback 秘密脱敏，以及合成 issuer 授权码一次性消费和换钥能力。前端测试覆盖 session schema、actor/project/command 缓存隔离、项目切换、退出、401/403 清理与既有未知结果恢复。

管理员 CLI 单元测试覆盖缺参、非标准 UUID、多余位置参数、帮助输出、DSN 脱敏和稳定错误退出码。真实 PostgreSQL 管理撤销测试已执行，覆盖状态变更、会话失效、幂等审计、历史命令保留和审计失败整体回滚。

## 当前限制

集成检查缺少 `MOUSEHOLE_TEST_DATABASE_URL` 时按合同失败，没有静默跳过。Windows `-race` 未执行；本机没有确认可用的 C 工具链。Linux CI 已配置 PostgreSQL、合成 issuer、身份预置、重启、race 和 Chromium 流程，但配置存在不等同云端 CI 已通过。

## A9/A10 交付证据与限制

- `cmd/admin` 已交付 `revoke-sessions`、`disable-user` 和 `revoke-membership`，使用稳定退出码；真实 PostgreSQL 集成用例覆盖状态变更、会话失效、幂等审计、历史命令保留和审计失败整体回滚。
- Playwright 已在 Chromium 实际执行管理员撤销成员关系后的 `403 project_forbidden`、页面无项目状态、后续请求拒绝，以及原 Cookie 重载后仍无项目访问；CI 显式传入已构建的管理员二进制，失败诊断不回显 DSN 或 Cookie 原值。
- 首轮真实浏览器运行暴露了 E2E 登录 helper 在 React 首次渲染前即时读取可见性的竞态；helper 改为等待“登录或工作台”稳定状态后，合并门禁 6/6 通过。
- 真实企业 IdP 兼容性不在本切片范围内；当前证据只证明标准 OIDC 合同与合成 issuer 行为。
