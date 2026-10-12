# P0 企业身份与项目访问实施计划

## 实施前门槛

- 本任务保持 planning，直到用户审查 PRD、设计和本计划后明确批准。
- 实现从新的功能分支开始并记录到 task.json；不再直接在 `main` 开发。
- ARTEX、真实企业 IdP 和外部目标继续只读/不访问。测试只使用合成 issuer 和本工程 PostgreSQL。
- 开始前由 implement 代理读取本任务 `implement.jsonl`、PRD、design 和本计划；完成后由独立 check 代理读取 `check.jsonl`。

## 顺序与职责

### 1. 迁移与身份存储

- 新增 `00002_identity_access.sql`，建立 users、external identities、projects、memberships、login attempts、sessions、access audit。
- 为历史固定 actor/project 建不可登录兼容记录，增加 commands/audit 外键，不修改 `00001`。
- 增加 sqlc 查询：bootstrap 幂等建档、登录尝试一次性领取、session 查询/撤销、成员共享锁和撤销更新。
- 集成测试先证明迁移保留既有命令，再证明 bootstrap 幂等/冲突和成员锁并发顺序。

风险点：迁移可能破坏已有开发数据；先在随机 schema 验证，再显式迁移开发库。禁止通过删除卷或关闭约束规避失败。

### 2. 身份领域与 OIDC 适配器

- 新建 `backend/internal/identity`，拥有 principal、用户/项目投影、session、登录尝试、bootstrap 与稳定错误类别。
- 锁定 go-oidc/oauth2；实现 discovery、PKCE、state、nonce、audience/azp 和 token 安全边界。
- 使用 auth key 域分离、AES-GCM 加密短期 verifier、SHA-256 session/state 摘要、HMAC CSRF。
- 编写进程内合成 issuer，支持 discovery、JWKS、code exchange、claim 故障和 key rotation；仅供 Go 测试与开发 E2E。

风险点：库不校验 nonce/azp，必须有失败测试；任何测试日志不得包含 code、token、cookie 或 verifier。

### 3. bootstrap 管理入口

- 新增 `backend/cmd/admin` 的 `provision-user` 子命令；参数数组解析、显式 DSN、稳定退出码和中文错误。
- 同事务创建/核对 user、external identity、project、membership、管理审计；相同参数重放，异参数冲突。
- README 写明首次管理员/成员预置流程和撤销方式。CLI 不提供 email-only、group claim 或 first-login admin 路径。

### 4. HTTP 会话与权限边界

- 重构配置为 development/production 明确模式，加入公共 URL、OIDC、auth key、session TTL；生产强制 HTTPS/Secure。
- 增加 login、callback、logout、session handler 和 Cookie/CSRF 中间件，统一 request_id、中文错误及日志脱敏。
- 移除 `X-Mousehole-Development` 和固定 actor/project 注入；健康路由保持公开且只表示进程存活。
- command 服务把成员 capability 锁放入同一事务，GET 同时检查活动成员与原 actor；更新接口 mock 和集成测试。

风险点：不要在 middleware 检查后把未受信 actor 字符串直接传入业务；不要为权限错误泄露命令是否存在。

### 5. OpenAPI 与 Web 登录体验

- 更新 `api/openapi.yaml` 的 auth/session/cookie/CSRF/错误合同并重新生成完整前端类型与 schema。
- API client 使用 `credentials: include`，PUT/POST 从 session 内存状态携带 CSRF；删除开发 header。
- 首屏实现登录、登录失败、无项目、会话过期、项目选择和退出；保留既有草稿未知结果和刷新恢复状态。
- Query key 含 actor/project；401 清理 session/命令缓存，项目切换清除不相干回执。不得使用 Web Storage 保存 token、cookie、CSRF 或服务端快照。

风险点：callback 是浏览器导航而非 JSON API；错误页和应用登录状态需要实际浏览器核对，不能只测组件 mock。

### 6. 工具链、文档和完整验证

- `scripts/dev.py` 管理合成 issuer、API、迁移与 Web；E2E 只停止自己启动的进程。
- 更新 `.env.example`、根/后端/Web README、CI、工具清单和验证记录。秘密只写占位符。
- 扩展生成漂移、文本、Go、Web 和 E2E 检查；缺 PostgreSQL、浏览器或合成 issuer 时失败。
- 独立 check 代理审查权限跨层数据流、OIDC 失败矩阵、迁移兼容、日志脱敏、CSRF 和并发撤销。
- 将实际有效的身份/会话/权限约定回写 `.trellis/spec`，再按 Trellis 提交、归档和日志流程收尾。

## 预期文件范围

| 范围 | 主要文件                                                                       |
| ---- | ------------------------------------------------------------------------------ |
| 合同 | `api/openapi.yaml`、生成 schema                                                |
| 后端 | `backend/internal/identity/`、`httpapi/`、`command/`、`cmd/admin/`、`cmd/api/` |
| 数据 | `backend/db/migrations/00002_identity_access.sql`、queries、sqlc 生成目录      |
| Web  | `web/src/api/`、`app/`、新增 auth/project 特性、现有 commands 特性             |
| 验证 | Go 单元/集成、Vitest、Playwright、`scripts/`、CI、validation 文档              |
| 规范 | 后端/前端实际 spec、README、父任务进度                                         |

## 验证命令

准确参数可随实现补充，但不得降低现有门禁：

```powershell
python scripts/dev.py database
$env:MOUSEHOLE_TEST_DATABASE_URL = 'postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole_test?sslmode=disable'
python scripts/check.py --integration --e2e
python scripts/verify_restart.py
```

新增针对性验证至少包括：

- Go OIDC/session/CSRF 单元与合成 issuer HTTP 测试。
- 真实 PostgreSQL 的 bootstrap、session、权限撤销和命令并发集成测试。
- Vitest 的 session schema、项目切换和认证错误状态。
- Chromium 的登录、刷新、草稿、撤销、退出及无 token 泄露。
- 迁移前既有命令记录在迁移后仍存在且归属未改写。

Linux CI 继续运行 race；Windows 本地如无 C 工具链必须如实记录未执行，不能视为通过。

## 回滚边界

- 代码回滚不自动删除身份数据；数据库迁移优先前向修复。
- auth key 或 Cookie 合同变更可主动撤销所有 session，让用户重新登录，不迁移浏览器秘密。
- 出现 OIDC/权限缺陷时关闭登录和新命令入口，保留历史命令、审计与会话撤销记录。
- 不以恢复固定开发 header 作为生产回滚方案；本地诊断通过测试专用 adapter 完成。

## 完成定义

PRD A1-A11 均有行为证据；现有命令幂等/未知结果检查无回归；独立质量检查无未处理高风险发现；实际 spec、README 和验收记录与代码一致。完成本任务仍不代表完整 P0 或高自主 V1 已交付。
