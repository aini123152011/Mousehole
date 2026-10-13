# 授权、资产与静态预检查合同

## 1. 范围与触发

适用于 `internal/target`、`access`、`asset`、`authorization`、`policy`、`scopecommand`、新 HTTP 入口及 `00003_authorization_assets.sql`、前向补强 `00004_scope_integrity.sql`。新范围账本独立于草稿表；资产来源表示导入声明，预检查不代表目标执行或已验证观测。接口权威为 [OpenAPI](../../../api/openapi.yaml)，实际检查结果见 [验收记录](../../../docs/validation/p0-authorization-assets.md)。

## 2. 签名

```go
// target 包：调用方输入类型已由领域入口限制。
func Normalize(kind, value string) (string, error)
func Matches(kind, rule string, include bool, targetKind, value string) bool
// access 包：调用方持有并提交事务。
func Lock(ctx context.Context, q *sqlc.Queries, actor, project string, write bool) error
// scopecommand 包：独立范围命令的受理与查询。
func (s *Service) Accept(ctx context.Context, actor, project, commandID string, r Request) (Receipt, bool, error)
func (s *Service) Get(ctx context.Context, actor, project, commandID string) (Receipt, error)
func (s *Service) AdminAccept(ctx context.Context, project, commandID, operatorRef string, r AdminRequest) (Receipt, bool, error)
func (s *Service) AdminGet(ctx context.Context, project, commandID, operatorRef string) (Receipt, error)
```

项目 HTTP 前缀为 `/api/v1/projects/{project_id}`：

| 方法与后缀                                                             | 行为                                              |
| ---------------------------------------------------------------------- | ------------------------------------------------- |
| `GET /assets`、`POST /assets/preview`                                  | 项目可见资产与整批规范化预览；预览不写业务表。    |
| `PUT /scope-commands/{command_id}`、`GET /scope-commands/{command_id}` | analyst 受理；仅原 Web actor 恢复命令。           |
| `GET /authorization-versions`、`/{version_id}`                         | 候选与历史正文只读。                              |
| `GET /policy`                                                          | 当前活动指针、全局/项目 revision 与有效禁止规则。 |
| `GET /prechecks`、`/{decision_id}`                                     | 当前项目成员读取历史静态决策。                    |

`scope_command_namespaces` 具有独立 UUID，kind 为 project/global，项目关联唯一、全局唯一；`scope_commands` 主键 `(namespace_id,command_id)`。项目资产、范围条目、决策和来源使用项目组合 FK；namespace 的 NULL 使用显式 IS NOT DISTINCT FROM 校验，不能依赖 MATCH SIMPLE 可空 FK。不可变正文/条目/决策与追加历史由数据库约束保护；新条目 INSERT 在提交时与正文的有序集合比较，避免正文保存后追加范围。必需审计/outbox 由提交时约束校验。旧 `commands/tasks/audit_events/outbox_events` 及已发布 migration 不改写；00004 对已有账本先核查后增加约束，异常必须失败，不以清理历史通过升级。

## 3. 请求、响应与环境合同

Web kind 限定 `import_assets`、`create_authorization_version`、`precheck_target`。使用 Cookie 会话和 session-bound CSRF，身份不从 payload 接收；viewer 只读，analyst 写入候选/导入/预检查，不能激活、撤销或改禁止。

导入 `entries` 为 1–100 个 domain/ip/url，原值至多 2048 字节，来源 `manual|api_import`，备注/引用受限；任意无效行整批拒绝，预览提供原行号。全企业自然键去重，HTTP 始终通过 `project_assets` 读取，仅公开本项目来源。写入按自然键稳定排序，同一命令来源唯一，不泄露其他项目是否已存在。

授权正文包括 title/basis_ref/contact、UTC 时窗、固定 algorithm/policy/catalog version、操作上限、请求/并发/每秒速率预算、禁止重定向/凭据和 1–100 个范围。预算、sequence/revision/count 为十进制字符串。规范正文排序去重后 SHA-256，不含数据库 ID、actor、created_at 或可变状态；`entry_ids` 与 `content.entries` 同序。有效期为 `[valid_from,expires_at)`。

target-v1 的保守边界：

| 类型          | 规范化与匹配                                                                                                                                                                                                  |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| domain        | 严格非 transitional IDNA、小写、去一个合法尾点；标签边界子域；ICANN/PRIVATE 裸公共后缀禁止泛授权。内部名称可明确授权。                                                                                        |
| ip            | netip 严格地址，mapped 单地址 Unmap；拒绝 zone 与歧义 IPv4。                                                                                                                                                  |
| cidr          | 仅用于范围；拒绝 mapped prefix 与 host bits，后者给 masked 建议，不暗中扩大。                                                                                                                                 |
| url/url_exact | HTTP(S) exact，主机共享 domain/ip 算法，去默认端口、空 path 为 /；仅 path percent hex 大写，raw query 的字节/顺序/空 ? 保留。拒绝 userinfo、fragment、反斜杠、非法 percent、编码 slash/backslash 和点段歧义。 |

三条入口共用该算法，版本升级不得重写旧内容。URL query/联系人/完整 payload 不进日志或审计摘要。禁止子串、正则或前缀获得正向许可。

目录固定 precheck-v1：dns_lookup 仅 domain、无参数；tcp_connect 为 domain/IP、显式 port；http_metadata 为精确 URL、固定 HEAD、无额外参数。HTTP/TCP 域名范围须有显式解析 CIDR；直接 IP 不借域名许可，不跨条目拼接操作/端口/解析约束。未知操作/参数拒绝。

candidate 可激活/撤销；激活替代旧 active 为 superseded；revoked/superseded 不能再次激活。撤销当前版本清空指针，不自动回退。管理命令要求 expected_policy_revision，只有首次业务执行检查 CAS；同键成功重放必须返回旧回执。

Web 事务按 user → project → membership → global policy → project policy/letter → command → business 取得锁。Web 前三类行各 FOR SHARE；身份管理员变更对应行 FOR UPDATE 且保持同序；范围 CLI 不伪造用户锁，从 global/project policy 的稳定行开始。预检查锁稳定 policy state，禁止变更取相应排他锁，空 deny 表也不能漏检查；migration 回填、新项目 provision 同事务建立 state/letter/namespace，缺行属于依赖故障。

拿齐锁后一次 clock_timestamp 作为 checked_at。权限先检查，再核对原命令 actor/source/kind/规范 payload（hash 之外也比较内容），再做 CAS/首次业务。受理、业务或 deny 决策、审计、outbox 同事务，必需写入失败全部回滚。审计仅存安全 ID/hash/reason/count；本切片无 outbox consumer。

结果仅 deny 或 requires_runtime_checks，保存 checked_at、version/hash、global/project revision、匹配条目、原因与 runtime_checks。历史恢复不重算也不表示现在获准；DNS/final IP/连接、当前权限、适配器、lease/fencing、真实预算和执行审计后续重验，没有 allow token。

列表默认 25、最多 100，按时间和 ID 降序 seek；page_count/has_more/next_cursor 表示本页与是否有后续，不伪造 total。cursor 签名且绑定项目、资源、筛选摘要与排序，不能含搜索原文；搜索参数化并转义 LIKE 特殊字符。

沿用显式部署环境、MOUSEHOLE_DATABASE_URL 和 MOUSEHOLE_AUTH_KEY，不自动加载 .env。迁移先于新 API/admin；回滚优先停新入口与前向修复，Down 只用于可丢弃的随机测试 schema。

## 4. 校验与错误矩阵

| 条件                                                  | 结果                                        |
| ----------------------------------------------------- | ------------------------------------------- |
| 未知/重复/缺失/大小写错键、null、非法范围/目录/cursor | 400 invalid_request，未受理。               |
| 当前用户/项目/成员失效或 viewer 写                    | 403 forbidden；旧回执也复核权限。           |
| 跨项目对象或其他 actor 回执                           | 404 not_found，不泄露 payload。             |
| 同 ID 同规范参数/actor/source                         | 原冻结回执；首次 201，重放 200。            |
| 同 ID 异参数/kind/操作引用                            | 409 command_conflict，原记录不变。          |
| 首次管理业务旧 revision                               | 409 policy_revision_conflict，不覆盖。      |
| 无有效版本、禁止、保留地址或不完整范围                | 成功持久化 deny 业务结果，不能用 403 代替。 |
| DB/Commit 未确认                                      | 503 unavailable；保留原 ID 查询，不换 ID。  |

HTTP 仍保留既有认证、CSRF、来源、大小与媒体类型错误；公开错误固定中文 envelope，不透传底层原因。范围 invalid_request 可附 validation_errors[{field,message}]，field 为受控机器字段路径，message 仅安全中文原因及规范 CIDR 建议；禁止使用任意 err.Error()、payload、URL query 或 SQL 作为公开详情。管理员 actor=null、source=admin_cli，operator_ref 为操作引用，不能当成经过 OIDC 验证的身份。

## 5. 好、基准与坏例

好例：明确授权私网 IP 的 TCP/443，当前活动版本静态完整匹配，结果为 requires_runtime_checks；执行仍须重验运行条件。基准：candidate 预检查持久化 deny，管理员激活后用新 command 检查。坏例：导入资产即扩大授权，badexample.com 子串匹配 example.com，或者拿历史预检查当执行凭证。

## 6. 必需测试及断言

target 反例验证 IDNA/PSL、尾点/标签、mapped/zone、CIDR 边界与 host bits、URL path/raw query；拒绝目标网络的静态夹具仍完成。真实 PG 验证跨项目/actor/角色、不可变正文与组合 FK、命名空间错配、激活替代/撤销/时窗/CAS、同键并发、审计/outbox 回滚、完整权限与 policy 的两种事务先后顺序（含空 deny）。

从 00001/00002 写合法旧历史，再升 00003/00004 核对旧四类记录；重启后旧草稿和新 scope 的 GET/PUT/CLI replay 逐字段相等，新预检查使用撤销后的 revision。HTTP 严格结构与错误矩阵、生成漂移、前端边界/行为与真实 E2E 都必须执行；缺依赖不能跳过后标通过。

## 7. 错误与正确

错误：先找 deny 行并锁住，空表当无限制；每次 replay 重新计算策略，或先验证旧 CAS 再找已成功命令。

正确：锁稳定 policy 行防空表竞态；当前权限通过后先返回原冻结回执，首次业务才检查 CAS 和当前规则。查询历史结果显示原 checked_at/revision，重新检查须新 command。
