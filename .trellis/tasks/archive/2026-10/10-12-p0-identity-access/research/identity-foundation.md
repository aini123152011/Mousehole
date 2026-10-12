# 身份与项目访问基础研究

- 查询：如何在既有草稿命令闭环上增加企业 OIDC、服务端会话、CSRF 和项目成员权限。
- 范围：当前 Mousehole 源码、已批准父方案及锁定依赖源码；未访问 ARTEX、真实 IdP 或外部目标。
- 日期：2026-10-12

## 当前实现约束

1. `backend/internal/httpapi/server.go:55-59` 把 `/api/v1` 全部置于固定开发身份中间件下，`server.go:136-157` 依赖开发标记和固定项目常量。企业接入必须替换这条边界，不能叠加一个看似生产但仍可回落固定身份的分支。
2. `backend/internal/httpapi/server.go:161-195` 直接把固定 actor 传给 command 服务；`web/src/api/client.ts:28-101` 则给 session、PUT、GET 都发送开发请求头。OpenAPI、生成 client、页面查询键和 E2E 必须一起迁移。
3. `backend/internal/command/service.go:29-77` 在事务前调用固定常量校验，事务本身只处理命令、草稿、审计和 outbox。成员权限若只在 HTTP 层检查，管理员撤销与命令提交可发生检查后竞态。
4. `backend/internal/command/service.go:81-96` 额外要求查询 actor 等于命令 actor。这个边界应保留：项目成员资格不是读取其他成员命令回执的授权。
5. `backend/db/migrations/00001_command_foundation.sql:8-55` 已有 actor/project UUID，但没有用户、项目或成员外键。新迁移需要保留旧数据并为固定开发 actor/project 建立不可登录的历史记录，不能改写已发布的 `00001`。

## 依赖研究

通过当前终端临时设置 `GOPROXY=https://goproxy.cn,direct` 查询模块版本，未修改全局配置：

- `github.com/coreos/go-oidc/v3` 最新可见稳定版本为 `v3.21.0`。
- `golang.org/x/oauth2` 最新可见稳定版本为 `v0.37.0`。

下载并检查 `go-oidc/v3 v3.21.0` 源码后确认：

- `oidc/verify.go:183` 明确说明 `Verify` 不执行 nonce 校验，调用方必须比较登录尝试保存的 nonce。
- `oidc/verify.go:248-257` 验证配置的 client ID 存在于 audience，但源码明确说明这不能保证 client ID 就是 authorized party。
- 多 audience 时调用方必须解析 `azp` 并要求其等于配置 client ID；单 audience 时仍拒绝与 client ID 不一致的 token。
- 不启用 `SkipClientIDCheck`。issuer、签名、有效期和 audience 由库校验，state、一次性登录尝试、nonce、azp、PKCE 与回调绑定由本项目补齐。

## 推荐合同

1. 使用 OIDC Authorization Code + PKCE；登录尝试落 PostgreSQL，保存 state 摘要、短期浏览器绑定 Cookie 摘要、nonce 摘要、短期 PKCE verifier、return path、过期和一次性消费状态。callback 必须同时匹配 state 与 HttpOnly 登录绑定 Cookie，防止攻击者把自己的 callback 链接交给受害者造成登录 CSRF。
2. 未预置 `(issuer, subject)` 时拒绝创建平台会话。email、姓名、groups 只更新展示资料或留待未来显式映射，不能作为当前授权依据。
3. 会话 cookie 使用 256-bit 随机值，数据库只保存 SHA-256 摘要。API 从原 cookie 派生 session-bound CSRF token，`GET /api/v1/session` 返回 token；浏览器仅保存在内存并在写请求头发送。
4. `project_memberships` 不物理删除，撤销改状态并递增版本。命令事务用 `SELECT ... FOR SHARE` 锁定活动成员行，撤销用 `FOR UPDATE` 更新同一行，从而形成“命令先提交或撤销先提交”的明确顺序。
5. 查询原命令同样在事务中锁定成员资格并检查命令 actor；防止权限检查通过后撤销已提交、响应仍泄露回执。
6. bootstrap CLI 直接使用受控数据库连接，要求 issuer、subject、用户 ID/显示名、项目 ID/名称和角色。重复参数幂等，身份或项目冲突拒绝，管理审计与变化同事务。

## 测试与风险

- Go 单元测试覆盖 state/nonce/azp/audience、PKCE、cookie 属性、CSRF、公开错误和日志脱敏。
- PostgreSQL 集成测试覆盖 bootstrap 幂等/冲突、会话摘要、禁用/撤销、命令与成员撤销并发、同项目跨 actor 回执拒绝和迁移保留旧数据。
- 合成 issuer 由测试进程拥有，支持 discovery、JWKS、code exchange、错误 claim 和密钥轮换；测试不依赖真实企业 IdP。
- Playwright 覆盖跳转登录、刷新后 cookie 会话恢复、项目选择、CSRF、退出和撤销。浏览器上下文不得出现 token 或 session 原值。
- 生产 HTTPS 与反向代理边界必须显式配置；开发模式只允许精确 loopback HTTP 来源，不能把开发 cookie 例外带入生产。

## 未纳入本切片

SAML、SCIM、JIT/group claim 自动授权、多企业、账户合并、资产 ABAC 和执行权限后置。OIDC 供应商的实际兼容性需在未来部署联调中验证，合成 issuer 只证明标准合同和本项目行为。
