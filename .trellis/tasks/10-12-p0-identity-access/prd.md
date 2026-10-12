# P0 企业身份与项目访问基础

## 目标与用户价值

将固定开发身份替换为单企业私有化部署可用的企业 OIDC 登录、服务端会话和项目成员权限。分析员登录后只能看到获准项目，并可继续使用既有草稿命令闭环；退出、会话失效或成员权限撤销后，受保护请求立即默认拒绝。

本切片建立身份与项目访问的可信基础，不增加目标执行、授权书、Temporal、Runner 或模型能力。

## 已确认事实

- 当前 API 只允许显式 development、loopback Host/Origin 和 `X-Mousehole-Development: true`，使用固定 actor/project；该标记不是凭据。
- `commands` 已保存 actor，幂等回放要求 actor、命令类型和规范化参数一致；原命令查询只允许原提交者。企业身份接入不得把“项目成员可读”扩大为“可查询其他成员命令回执”。
- 草稿命令的四表事务、未知结果恢复、标准 UUID、严格 JSON 和中文错误合同已通过真实 PostgreSQL 与浏览器验收，身份改造必须保持这些行为。
- 平台形态为单企业私有化，V1 身份首选 OIDC；SAML、SCIM 和多租户 SaaS 已明确后置。
- 浏览器不得把令牌、会话标识或 CSRF 秘密写入 URL、localStorage 或 sessionStorage。测试不得连接真实企业 IdP。
- 首版采用管理员预置授权：部署运维先绑定 `(issuer, subject)` 并分配项目角色；未知 OIDC 用户认证成功后也不创建平台会话。首位管理员和后续成员均通过受控 bootstrap CLI 配置，不做 JIT 自动开通。

## 范围内需求

1. API 实现 OIDC Authorization Code + PKCE 登录，校验 issuer、签名、audience、authorized party、state、短期浏览器登录绑定、nonce、时间与回调地址。OIDC access token、refresh token 和 ID token 不下发给浏览器，也不写普通日志。
2. 登录成功后创建 PostgreSQL 服务端会话，浏览器只持有不透明 Cookie。生产 Cookie 使用 HttpOnly、Secure、SameSite，登录、回调和退出具有明确的缓存与重放边界；会话标识在库中仅保存不可逆摘要。
3. 写接口校验与服务端会话绑定的 CSRF token，并继续执行严格 Origin/Host 校验。认证失败与项目拒绝使用稳定英文 code、中文 message 和 request_id，不通过重定向掩盖 API 错误。
4. 身份以 `(issuer, subject)` 作为不可变外部主键；email、姓名和展示名只作可更新资料，不能单独用作授权依据。actor 使用平台 UUID，审计和命令继续引用稳定 actor。
5. 建立用户、外部身份、项目、项目成员、角色和服务端会话的最小模型。首版角色至少区分项目只读成员和可创建任务草稿成员；部署运维通过 bootstrap CLI 管理身份绑定和成员关系，该能力不等于后续目标执行权限。
6. `GET /api/v1/session` 返回当前 actor、会话状态、可访问项目及每个项目的稳定角色/能力集合；页面从服务端选择项目，不再依赖固定常量或开发请求头。
7. 命令查询继续只允许原提交 actor。草稿写入在同一数据库事务内重验当前项目写权限，并与成员撤销采用可证明的并发顺序；仅在路由中间件检查不满足验收。
8. 权限撤销后，新请求立即拒绝；已受理草稿及其审计保留。会话退出、过期、管理员全局撤销及用户禁用均使后续请求失效，不删除历史 actor 或改写原命令归属。
9. 本地开发与 CI 使用进程内合成 OIDC issuer 或专用测试服务，覆盖发现文档、JWKS 轮换、回调和错误场景；不得访问真实企业 IdP、ARTEX 或外部目标。
10. OpenAPI、Go、sqlc、React 客户端、中文界面、README、配置样例及实际前后端 spec 同步更新。依赖精确锁定，生成漂移、格式、lint、类型、单元、真实 PostgreSQL 集成和浏览器登录流程进入统一检查。
11. bootstrap CLI 必须要求明确的 issuer、subject、平台用户和项目角色，重复执行幂等、参数冲突拒绝，并写管理审计；不得以 email、显示名或首个登录者自动授予管理员权限。

## 验收标准

- [x] A1：未登录访问 session、项目命令读写均得到统一 `401 unauthenticated`；不会回落到固定开发身份。
- [x] A2：合成 OIDC 完成 state、短期浏览器登录绑定、nonce、PKCE 和令牌验证；state/登录绑定/nonce/audience/authorized party/issuer/签名/时间任一无效均拒绝且不创建会话。
- [x] A3：浏览器只收到不透明的短期登录绑定与会话 Cookie；URL、Web Storage、页面源码、公开错误和普通日志中没有 OIDC token 或 Cookie 原值。
- [x] A4：有效成员能看到自己的获准项目与稳定能力；非成员、禁用用户和错误项目得到 `403 project_forbidden`，响应不泄露对象是否存在。
- [x] A5：项目只读成员不能创建草稿；具有草稿写权限的成员可创建、回放和查询自己的原命令，既有幂等及未知结果恢复合同保持不变。
- [x] A6：同项目的另一成员不能查询、回放或冲突探测原 actor 的命令 ID。
- [x] A7：成员撤销与草稿提交并发时，数据库锁顺序产生可解释结果：撤销先提交则命令拒绝；命令先提交则完整四表记录保留且撤销后的新请求拒绝，不出现无审计草稿。
- [x] A8：缺失、重复或错误 CSRF token 的写请求拒绝；合法同源页面可写，跨来源请求不能借 Cookie 写入。GET 不产生业务变化。
- [x] A9：退出、会话到期、管理员撤销和用户禁用均使 Cookie 后续请求失败；重放旧 Cookie 不恢复权限。
- [x] A10：真实 PostgreSQL、合成 issuer 和 Chromium E2E 覆盖登录、项目选择、草稿受理、刷新恢复、权限撤销和退出；缺少必需依赖时检查失败，不静默跳过。
- [x] A11：预置 `(issuer, subject)` 的成员可登录；未知 subject、仅 email 匹配或错误 issuer 均得到明确的未开通结果且不创建用户、成员或会话。bootstrap 重放不重复建档，异参数冲突不覆盖原绑定。

## 范围外

- SAML、SCIM、LDAP 密码登录、社交登录、多企业/多租户隔离和公共自助注册。
- 资产级 ABAC、授权书、审批、执行权限、Runner、Temporal、SSE 和目标访问。
- 企业 IdP 的生产联调、供应商专属 group claim 适配和自动组织结构同步；本切片只保证标准 OIDC 合同及可配置 claim 映射边界。
- 账户合并、跨 issuer 迁移、用户自行绑定第二身份和恢复码。
- JIT 自动开通、按 email/domain/group claim 自动授权，以及“首位登录者自动成为管理员”。
