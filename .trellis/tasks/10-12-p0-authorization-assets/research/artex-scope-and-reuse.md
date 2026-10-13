# Research: ARTEX 授权范围、规范化与 Mousehole 复用边界

- Query: 在只读复核 ARTEX 源码的基础上，确认 P0 授权版本、项目资产与静态预检查应继承的设计、应纠正的边界，以及现有 Mousehole 命令账本和权限锁的兼容要求。
- Scope: internal；外部官方资料仅列为设计参考，未进行外部文档核验。
- Date: 2026-10-12
- 活动任务: `.trellis/tasks/10-12-p0-authorization-assets`，由主会话明确派发。
- 调研方法: 阅读真实源文件、SQL、测试和项目规范；未构建、运行或测试 ARTEX，未发起目标 DNS、HTTP 或 TCP 请求。

## Findings

### 1. 文件清单

ARTEX 源码根目录为 `E:/Crazywhite/ARTEX/`。以下路径均相对于该根目录：

| 文件                          | 用途                                                           |
| ----------------------------- | -------------------------------------------------------------- |
| `db/asset_intercept_match.go` | 资产拦截候选、精确/模糊匹配和禁止优先的任务闸门。              |
| `db/company_scope.go`         | 企业范围输入分类、主机名/IDNA、IP、CIDR 和公共后缀检查。       |
| `db/company_scope_test.go`    | 范围解析样例；包含 URL userinfo 被接受并提取 host 的现有行为。 |
| `db/nkey.go`                  | 域名/IP 自然键和 eTLD+1 根域提取。                             |
| `db/assets.go`                | URL 规范化、资产写入以及范围归属更新事务。                     |
| `db/companies.go`             | 范围变更 advisory transaction lock 与事务写入。                |
| `db/schema.sql`               | 六类共享资产、自然键、任务来源关联等表结构。                   |
| `db/task_scope.go`            | 显式资产插入所生成的任务范围、Agent 范围解析和粗略覆盖度。     |
| `db/asset_dsl.go`             | 全局和任务范围内资产查询路径。                                 |
| `db/exploration.go`           | 探索节点到资产的谱系锚点。                                     |
| `go.mod`                      | 实际依赖版本；`golang.org/x/net v0.58.0`。                     |

Mousehole 源码根目录为 `E:/Crazywhite/Mousehole/`：

| 文件                                                   | 用途                                                     |
| ------------------------------------------------------ | -------------------------------------------------------- |
| `backend/db/migrations/00001_command_foundation.sql`   | 已发布草稿命令、任务、审计、outbox 四类记录的原子边界。  |
| `backend/db/queries/commands.sql`                      | 草稿专用幂等插入、原回执查询和固定事件写入。             |
| `backend/db/queries/identity.sql`                      | 活动成员查询和管理行锁；存在实际锁范围与规范表述不一致。 |
| `backend/internal/command/domain.go`                   | 固定草稿命令类型、请求规范化和回执。                     |
| `backend/internal/command/service.go`                  | 权限检查、数据库唯一键串行化和四类记录事务。             |
| `backend/internal/command/service_integration_test.go` | 真实 PostgreSQL 并发、撤销锁顺序、回滚等验收模式。       |
| `backend/internal/identity/store.go`                   | 用户禁用、成员撤销与管理审计的事务边界。                 |
| `web/src/app/app.tsx`                                  | 会话、项目选择、退出和命令工作台组合。                   |
| `web/src/features/commands/use-command-workspace.ts`   | 身份/项目/命令缓存键，401/项目 403 清理。                |
| `api/openapi.yaml`                                     | 已发布 HTTP 契约，后续接口与生成类型的权威来源。         |
| `backend/go.mod`                                       | 当前工具链 `go1.26.9`，`pgx/v5 v5.11.0`。                |

### 2. ARTEX 值得继承的部分及准确限制

1. **禁止优先并提供原因。** `db/asset_intercept_match.go:178` 说明 block → allow 的判定顺序；`:185` 实现先命中 block 则拒绝，`:35` 为拒绝提供可读原因。可继承为稳定 reason code、中文说明、命中条目 ID 和版本 ID，便于页面解释与审计。
2. **保留事务一致性。** `db/assets.go:101` 和 `:104` 将范围归属解析及资产写入放在同一事务，并复用嵌套事务；`db/companies.go:426` 使用 `pg_advisory_xact_lock`。因此不能把 ARTEX 概括为没有范围一致性或完全依赖进程锁。Mousehole 应继承数据库原子边界，同时按项目/授权头部明确锁粒度。
3. **谱系与来源是有价值的资产知识。** `db/schema.sql:40` 定义六类共享资产，`:51` 的 `task_ids[]` 表示来源关联；`db/exploration.go:245` 明确 anchor 是 node → asset 的谱系边，并不承担可见性授权。新平台应保留来源记录与追踪能力，但使用项目边界控制数据读取。
4. **既有任务查询并非完全无边界。** `db/asset_dsl.go:552` 的 `QueryDSLInScope` 按当前任务及直接来源任务的范围筛选，`:560` 才在 `taskID<=0` 时退回全局查询。应准确描述两条路径，不能只摘取 anchor 注释便推断所有查询均不受限制。

### 3. 必须调整的授权语义

| 源码事实                                                                                       | 对新平台的要求                                                                             |
| ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `db/asset_intercept_match.go:183` 明确无启用 allow 时全部放行。                                | 授权版本不存在、无正向条目、未激活、未生效、过期或已撤销时默认拒绝。                       |
| 同文件 `:69`、`:76`、`:82` 的 fuzzy domain/IP/URL 使用子串匹配。                               | 正向授权禁止使用子串匹配。域名仅支持 `exact` 和标签边界上的 `include_subdomains`。         |
| 同文件 `:119` 起收集 Domain、RootDomain、BoundDomains 和 URL host 等候选。                     | 资产描述字段不能取代目标身份；预检查必须规范化具体候选目标，再匹配明确类型的规则。         |
| `db/task_scope.go:104` 明确只有顶层显式插入资产会调用 `AddAutoScope`，派生资产不应自动扩范围。 | 新平台进一步拆开资产与授权：手动录入、API 导入、发现及派生都不得写授权条目或激活授权。     |
| `db/task_scope.go:177` 的 `root_domain` 分支经 `RootDomain` 转成 eTLD+1。                      | 不得将 `a.example.com` 静默升为 `example.com` 的执行授权；是否包含子域必须由明确字段表达。 |
| `db/task_scope.go:276` 的覆盖度说明将 fact anchor 作为“约已测”的依据。                         | fact/资产引用仅为谱系，不能证明已执行、已验证漏洞或覆盖完成；此切片不生成这类执行结论。    |

主会话已明确本切片只提供纯本地预检查。返回值限定为 `deny | requires_runtime_checks`；即使匹配静态规则，也不返回可执行 `allow`。域名和 URL 的最终 IP 必须受授权版本内显式 `resolved_ip_ranges` 约束。DNS、真实连接、每跳重定向、预算、审批和 lease 的最终检查属于后续 P1。

### 4. 统一规范化的必要性与推荐合同

ARTEX 目前存在多条不同的规范化路径：

- `db/company_scope.go:166` 的 `scopeHostname` 从 URL 提取 host，`:190` 调用 `idna.Lookup.ToASCII`，后续检查总长度及标签。
- `db/nkey.go:16` 的 `DomainKey` 仅 trim/lower/去一个末尾点，没有 IDNA；`:20` 的 `IPKey` 仅 trim。
- `db/assets.go:164` 的 `normalizeURL` 仅小写 scheme/host、去裸根末尾 slash；解析失败原样返回。
- `db/company_scope_test.go:54` 明确将 `https://user:pass@example.com/path` 接受为 `example.com`，`:52`、`:53` 将 URL 路径和 query 丢弃为域名范围。这是企业归属解析行为，不能当作新平台的 URL 精确授权语义。

推荐资产、授权条目、候选目标使用同一纯函数所有者，但通过不同输入类型维持语义，不能接受 URL 后静默降格成域名授权。

#### 域名及公共后缀

- 使用固定 `x/net` 版本的 IDNA Lookup，转 ASCII、小写，允许去除一个合法 FQDN 末尾点；拒绝空标签、无效长度、userinfo、端口、路径、百分号和裸通配符。
- `include_subdomains` 仅在 `target == rule` 或 `target` 以 `.` 加规则域名结尾时命中；`badexample.com`、`example.com.attacker.test` 均不得命中 `example.com`。
- `db/company_scope.go:267` 只在 `icann && suffix == domain` 时拒绝裸公共后缀，因此 PRIVATE 部分裸后缀未被该条件覆盖。新规则必须同时处理 ICANN 和 PRIVATE 部分，例如拒绝以 `github.io`、`blogspot.com` 作为泛子域授权；这不等于拒绝其合法租户子域。
- 不能把 `icann=false` 当作“没有公共后缀限制”。若支持企业内部域名，应另写显式合同和样例；该布尔值还可能表示 PSL PRIVATE 项或未列入 PSL 的名称。
- 固定规范化版本并保存规范化快照，避免升级 IDNA/PSL 后静默改变已激活授权版本的解释；新增版本时再应用新规则。

#### IP、CIDR 与 IPv4-mapped IPv6

- 使用 `net/netip` 严格解析，不做 DNS，不接受整数/八进制/缩写 IPv4，也不接受带 zone 的 IPv6 作为普通资产或授权 IP。
- 推荐单个 mapped 地址先 `Unmap()` 到 IPv4，使 `::ffff:192.0.2.1` 与 `192.0.2.1` 具有相同资产键；授权范围与运行时连接检查也必须同样处理，不能混用字符串 IP 与二进制 IP。
- `db/company_scope.go:225` 通过 `net.ParseCIDR` 获得 masked prefix，`:233` 存 `ipnet.String()`；这会把含 host bits 的输入转成网络地址。新平台需要明确选择拒绝 host bits，或在提交前显示原输入及规范化网络并绑定规范化命令参数，不能使用户误解范围大小。
- 本切片推荐拒绝 mapped IPv6 CIDR，仅接受标准 IPv4/IPv6 CIDR；单个 mapped 地址仍可 Unmap。若主设计选择接受映射前缀，则仅对处于 `::ffff:0:0/96` 内且长度 `>=96` 的前缀转换为 IPv4 的 `bits-96`，更短映射前缀不能直接 Unmap，且转换后仍须检查范围宽度。
- prefix 匹配统一使用同一地址族策略与 `Prefix.Contains`。需测试 `/32`、`/128`、含 host bits、mapped 地址、`::ffff:192.0.2.0/120`、过宽 `/96` 映射前缀、非映射 IPv6 和 zone。
- ARTEX 的 IPv4 最小 `/16`、IPv6 最小 `/32` 是现有业务限制（`db/company_scope.go:227`、`:230`），不能未经讨论机械继承为新项目通用规则。

#### URL 路径与 query

- 仅接受绝对 HTTP/HTTPS URL；拒绝 userinfo、fragment、opaque URL、反斜杠、控制字符、无效 percent encoding 和非法/超范围端口。
- scheme 小写；host 使用同一域名/IP 规范化；默认端口是否省略和空路径是否统一 `/` 必须有唯一合同，推荐省略显式默认端口并把空路径视为 `/`。
- 精确 URL 授权必须匹配 scheme、规范化 host、有效端口、escaped path 和原始 query，不能只用 host 匹配，也不能默认继承所有端点。
- query 保留字节顺序、重复项、`+`/`%20` 区别和空 query 标记；不得通过 `url.Values.Encode` 排序后假设服务端语义等价。`?a=1&a=2` 与 `?a=2&a=1` 的响应可能不同。
- 路径保持大小写、尾 slash 和转义语义；不得 `path.Clean`、双重 decode 或把 `%2F` 当普通 `/`。`/a`、`/a/`、`/A`、`/a%2Fb`、`/a/b`、`/%252e` 均需明确边界测试。
- 对 literal/编码 dot segment、percent hex 大小写、空 query marker 的接受或拒绝策略应由最终设计写死。保守实现可拒绝歧义路径，但不能在未说明时把它们折叠成更广的授权。
- P0 精确匹配结果只代表当前规范化版本中的静态关系；P1 使用实际请求序列化和出口网关时，仍要保证执行目标与已校验 URL 相同，并逐次校验 DNS/重定向。

### 5. 已发布命令账本的兼容边界

不能仅扩展 `kind` CHECK 就承载新的资产和授权命令：

1. `backend/db/migrations/00001_command_foundation.sql:12` 限制 kind 为 `create_task_draft`；`:14` 的 `task_id` 非空且唯一，`:15` 的 `task_title` 非空；`:34` 到 tasks 的外键在提交时检查。
2. 同迁移 `:37` 起的审计和 `:48` 起的 outbox 都要求 task 外键；事件类型固定为 `task_draft_created`。
3. `backend/db/queries/commands.sql:3` 的插入固定 task_id/task_title，`:18`、`:22` 固定草稿事件；不是通用命令记录接口。
4. `backend/internal/command/domain.go:62` 的 `NormalizeRequest` 只接受草稿命令；`backend/internal/command/service.go:29` 整体围绕创建任务和草稿回执，`:66`、`:70` 绑定 actor、kind 和规范化标题。
5. 当前 Web 和公开契约依赖固定 `accepted/draft`、任务字段、版本字符串和未知结果恢复；不能为了新命令改变历史回放内容或把资产 ID 塞进 task_id。

设计需要明确二选一：新增本切片专用账本及明确命令命名空间，或通过新增迁移将通用受理头与具体结果分开并保持旧接口的兼容投影。前者改动小，后者便于未来统一，但迁移和历史兼容成本更高。无论选择哪种，必须保留 actor/project 参数绑定、数据库唯一键防重、未知 Commit 保留原 ID 查询、审计/outbox 同事务，以及同键异参冲突行为。

已发布 `00001`/`00002` 不得覆写。需要真实 PostgreSQL 先写入旧草稿记录，再运行新迁移并逐字段比较原回执，验证外键、审计、outbox 和重启恢复未改变。

### 6. 实际权限锁缺口及复用建议

`.trellis/spec/backend/database-guidelines.md` 声明命令事务锁定活动 user/project/membership。实际 `backend/db/queries/identity.sql:127` 查询 JOIN users/projects 并检查活动状态，但 `:138` 仅 `FOR SHARE OF pm`。

- 成员撤销锁 membership，因此已有成员撤销两种顺序可以串行化；`backend/internal/command/service_integration_test.go:349` 已有该验收模式。
- 用户禁用在 `backend/internal/identity/store.go:264` 起事务并于 `:274` 锁 user，再更新状态、撤销会话和写管理审计；当前命令读取活动 user 却未锁住该行，不能声称用户禁用与命令已具备同样串行化保证。
- `backend/internal/identity/store.go:300` 的成员撤销于 `:310` 锁 membership；管理审计也在同一事务。

主会话已选择新增最小事务 helper（最终设计命名 `internal/access`），按 **user → project → membership** 显式取得 `FOR SHARE`，草稿服务与新资产/范围服务共同使用。helper 不创建独立事务，不持有请求之外的进程缓存；权限拒绝与依赖故障仍由领域边界映射为稳定错误。管理变更需保持兼容锁顺序，未来多对象变更必须说明排序规则。

新增真实 PG 用例至少覆盖“禁用/撤销先持锁并提交 → 命令拒绝且零写入”及“命令先持锁 → 完整提交后禁用/撤销”，同时保留历史成员撤销用例。门闩/触发器和数据库锁观察可沿用既有测试模式；不能仅用 mock 或单纯依赖 sleep 断言隔离正确。

### 7. 前端复用及缓存清理

现有 `web/src/features/commands/use-command-workspace.ts:27` 的 key 包含 actor/project/command，可作为资产、授权版本和预检查 Query key 的模式。`web/src/app/app.tsx:37` 的退出、`:82` 的项目切换，以及 Hook `:53` 的 401、`:60` 的项目 403，目前仅清理 `['commands']`。

新增 Query 不能只复制键结构而遗漏清理。退出、会话/业务 401 必须清理全部受保护业务缓存；项目 403 清理当前 actor/project 缓存；项目切换应终止或隔离旧请求，并重置选择、未知提交和通知。迟到响应不得写入新身份/新项目。令牌、CSRF、回执和敏感输入仍不存 Web Storage。

shadcn 基础控件、React Hook Form/Zod 输入校验、OpenAPI 生成类型、Ajv 运行时响应校验和 TanStack Query 服务器状态已存在，可直接沿用；后端为唯一授权权威，浏览器的规范化预览不取代服务端检查。

### 8. 受影响边界与验收建议

预计修改现有权限查询/草稿服务、新增 `projectaccess`、资产/授权/预检查领域服务和 SQL 迁移/查询，并同步 OpenAPI、生成 Go/TS 文件、中文页面及真实 E2E；具体文件清单与顺序由主会话最终实施计划确定。sqlc/API 生成文件只能通过固定版本生成器更新。

本切片的有意义验收：

- 没有、空、未激活、未生效、过期、撤销授权时默认拒绝；禁止条目优先；当前紧急禁止不能被旧版本快照绕过。
- 激活版本内容不可变，新授权需新版本；管理员 CLI 激活，analyst 不得自行激活或添加 Web 管理员角色。
- 资产录入/导入/派生不改变授权；去重只在明确项目和同一规范化版本内进行，保留来源与导入回执。
- IDNA/PSL PRIVATE、合法租户子域、标签边界、userinfo、端口、escaped path、重复 query、mapped IP/CIDR、host bits 等边界在资产和授权路径一致。
- 预检查只产出 `deny | requires_runtime_checks`，携带规则/版本/时间上下文与原因；domain/URL 不具备明确 `resolved_ip_ranges` 时不得被描述为可执行。
- 幂等回放不重复业务、审计或 outbox；同键异参冲突不变；用户禁用/成员撤销竞争满足两种序列；审计/outbox/延迟约束失败时整体回滚。
- 新迁移保留历史草稿回执与四类记录，真实数据库和进程重启验收原字段不变。
- viewer 可见能力和 analyst 写能力按最终合同验证；跨项目、跨 actor 回执访问拒绝；退出/401/项目403/项目切换和迟到请求不泄露缓存。
- 在代码结构和测试依赖中确认预检查不调用 resolver、dialer、HTTP client 或目标工具；不存在由审计/outbox 触发目标行为的消费者。

### 9. 相关规范

已阅读 `.trellis/workflow.md` 和以下规范，后续实现需按实际包/层加载：

- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/database-guidelines.md`：事务、唯一约束、已发布迁移、管理审计和真实 PG 验收。
- `.trellis/spec/backend/error-handling.md`：Cookie/CSRF、稳定错误码、未知提交结果和管理员 CLI 退出码。
- `.trellis/spec/backend/quality-guidelines.md`：UTF-8 无 BOM、LF、中文注释、固定生成器及检查入口。
- `.trellis/spec/frontend/index.md`
- `.trellis/spec/frontend/state-management.md`：Query 权威、缓存隔离、未知恢复、Web Storage 边界。
- `.trellis/spec/frontend/type-safety.md`：strict、OpenAPI/Ajv、BIGINT 版本字符串和对象匹配。
- `.trellis/spec/guides/cross-layer-thinking-guide.md`
- `.trellis/spec/guides/code-reuse-thinking-guide.md`

### 10. 外部参考与版本

下列官方资料是后续实现参考；本次未访问外部页面，不能据此声称已完成文档版本核验：

- Go `net/netip`：<https://pkg.go.dev/net/netip>，重点 `Addr.Unmap`、`Addr.Is4In6`、`Prefix.Masked`、`Prefix.Contains`。
- Go `net/url`：<https://pkg.go.dev/net/url>，重点 `RawPath/EscapedPath`、`RawQuery`、`ForceQuery`、userinfo 和 Hostname/Port。
- `x/net/idna`：<https://pkg.go.dev/golang.org/x/net/idna>；ARTEX 当前依赖 `x/net v0.58.0`，Mousehole 若引入需固定兼容版本。
- `x/net/publicsuffix`：<https://pkg.go.dev/golang.org/x/net/publicsuffix>，重点 ICANN/PRIVATE 及 PSL 版本对不可变授权解释的影响。
- PostgreSQL explicit locking：<https://www.postgresql.org/docs/current/explicit-locking.html>；具体运行 PG 版本以项目工具链/验收环境为准。

## Caveats / Not Found

- 本报告为源码调研和设计建议，不代表实现、迁移或运行时控制已经完成。只写本活动任务 `research/`；没有修改代码、spec、任务元数据或 Git。
- 静态预检查不能防止 DNS rebinding、真实连接出口变化、重定向越界、Agent 绕过、预算消耗或请求改变目标状态；这些属于 P1 的实际 admission/Runner/出口能力。
- 调研时的命令账本、命名空间、URL 和 CIDR 备选项已由 [最终设计](../design.md) 收敛：独立 scope 账本/namespace、exact URL 保留 raw query/空 query、拒绝歧义点段、拒绝 CIDR host bits/mapped prefix。研究建议不替代最终设计，且这些合同尚未实施。
- 当前 ARTEX 公共后缀条件只覆盖 `icann=true`，但本报告没有运行 ARTEX 测试来枚举所有 PRIVATE 案例；示例用于表达新合同边界。
- 本次没有测试数据库操作或浏览器验证；后续真正验收必须按项目检查入口执行并如实记录，不能以阅读测试源码代替通过结果。
