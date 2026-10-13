# P0 授权与资产代码实施计划

当前实施及本地验收已完成，待提交归档；用户于 2026-10-12 批准最终规划后运行 `task.py start`。以下为本切片的实施与验收顺序，完成情况以实际检查记录为准。完整中文风格沿用 [coding-standards.md](../../../10-11-artex-rebuild-plan/coding-standards.md) 和 [coding-templates.md](../../../10-11-artex-rebuild-plan/coding-templates.md)，实施前同时加载实际 backend/frontend spec。

## 1. 顺序与代码所有权

| 顺序 | 文件/模块                                                                                                                                     | 交付与通过条件                                                                                                                                 |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | `api/openapi.yaml`；`backend/internal/target/` 的 doc/domain/normalize/match 与测试                                                           | 固定 target-v1、结构化 scope/operation/error/receipt。表驱动反例先覆盖域名边界/公共后缀、IP family、CIDR、URL path/query；纯函数不引入目标 I/O |
| 2    | `backend/db/queries/identity.sql`、`internal/access/`、`internal/command/service.go`、`internal/identity/store.go`                            | 按 user→project→membership 完整锁定；旧服务复用。管理员成员撤销同序，用户禁用保持稳定 user 排他锁；受控并发验收真实事务排序                    |
| 3    | `backend/db/migrations/00003_authorization_assets.sql`、`db/queries/{assets,authorization,policy,scope_commands}.sql`、`internal/store/sqlc/` | 新表、组合 FK/CHECK/索引、正文不可变、active 一致性、稳定 policy 行和幂等账本；新项目 provision 同事务建 state；旧 migration 不改              |
| 4    | `internal/scopecommand/`、`internal/asset/`                                                                                                   | 身份/规范 payload/hash/replay、回执查询、整批导入、来源与跨项目隔离；审计/outbox 整体回滚，稳定排序避免资产 upsert 反转                        |
| 5    | `internal/authorization/`、`internal/policy/`、`backend/cmd/admin/main.go`                                                                    | candidate/激活/替代/撤销/deny 状态、CAS、预检查及持久化解释；CLI 原 ID 回执查询，忠实 actor=null/source；预算/目录不冒充执行能力               |
| 6    | `internal/httpapi/{server,decode,config}.go` 及 API 入口组装                                                                                  | 会话/CSRF/能力、项目查询过滤、批次限制、seek cursor、错误矩阵、请求 ID/安全日志；新模块通过明确接口注入                                        |
| 7    | `web/src/api/client.ts`、生成目录、`features/{assets,authorization,prechecks}/`、`app/`                                                       | OpenAPI 生成 + Ajv 边界检查；RHF/Zod、Query keys、command unknown/replay、中文页面/键盘路径。路由与新增 shadcn provenance/lockfile 同步        |
| 8    | `backend/**/*_test.go`、`web/src/**/*.test.tsx`、`web/e2e/`、`scripts/verify_restart.py`                                                      | 真实 PG、并发/故障注入、前端行为与真实全栈；合成角色/项目隔离、断线恢复、重启和旧草稿回归                                                      |
| 9    | `README.md`、`docs/validation/p0-authorization-assets.md`、实际 spec 与父任务进度                                                             | 准确 CLI/API/迁移/恢复说明、PA-A1–A9 验收矩阵、中文注释样本；仅在行为已通过后把新合同写入 spec                                                 |

开始执行时先派遣 Trellis implement 代理负责以上产品代码，随后独立 check 代理检查并修复；主代理负责范围、集成和验收证据。共享工作区不得回退其他代理/用户改动。任务无需创建新线程，也不在规划中提前派遣实施。

接口/规范化小步稳定后再扩展数据库/页面；一旦发现需要改变 operation、授权内容、权限或 URL 支持边界的实质产品取舍，先更新规划并重新最终审查，不以代码选择暗改范围。

## 2. 中文注释和统一格式的具体编写规则

| 代码位置                               | 必须交付的说明                                                                                                                                                   |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 新 Go 包 `doc.go`                      | `Package 包名` 开头，中文说明职责、数据权威、依赖方向及目标 I/O 禁止边界                                                                                         |
| 手写导出类型/函数/常量                 | 以英文符号名开头，中文说明输入/输出；版本、UTC 时间、单位、空值及敏感字段有实际语义说明                                                                          |
| canonicalization/matching              | 为 IDNA、域名 label、mapped IP、CIDR host bit、escaped path/raw query 等说明为何保守；禁止“保证所有 URL 等价”之类失真注释                                        |
| access/authorization/scopecommand 事务 | 明确锁序、持锁时段、先提交语义、CAS、幂等核对、审计失败回滚、Commit 不确定与原 ID 恢复；不宣称历史决策仍有效                                                     |
| SQL/query/migration                    | 中文用途、预期行数、筛选/seek 排序、稳定锁、约束原因、部署顺序/兼容/Down 数据损失；保留 sqlc/goose 固定英文指令                                                  |
| TS 类型、组件、Hook、API client        | 中文 TSDoc 描述业务含义、权限、command unknown、缓存范围及迟到响应；param/returns/throws 只在有实际语义时使用                                                    |
| OpenAPI/错误/日志/UI                   | 中文 summary/description/message/操作提示；英文 snake_case 字段、机器错误码和状态保留。日志只记录安全 ID/hash/reason/count，不记录目标 query/联系人/整个 payload |
| 测试、脚本、README/验收                | 中文说明测试意图、依赖、预期失败和恢复；已执行与未执行分别登记，测试名称可遵循英文工具约定                                                                       |

UTF-8 无 BOM、LF、单个末尾换行、无尾随空白；Go 用 gofmt/goimports，Web 用现有 Prettier/ESLint/TS strict，不另建冲突规则。生成目录只运行生成器，保留声明与许可证。手写文件不加个人真实姓名或维护流水；Git 保存历史。TODO 使用带问题引用的中文完成条件，授权/事务缺口不能以 TODO 代替已完成实现。

中文完整性由人工抽查包说明、导出符号、规范化、事务 SQL、Hook、公开错误和测试说明；字符比例扫描、逐行重复语法及增加注释数量都不能证明语义完整。

## 3. 行为验证矩阵

- PA-A1/A4：公开接口的输入/结果表驱动测试，不镜像函数内部实现；包括 Unicode/尾点/公共后缀、错误子域、非标准 IPv4、zone/mapped、CIDR 主机位和家族、URL 默认端口/空 path/空 query/参数次序/%2F/点段/用户名/片段/控制字符。
- PA-A2：同 canonical 跨项目导入，仅本项目来源可见；role/csrf/跨 actor command/版本与 decision 对象错配；DB 组合 FK 拒绝跨项目绑定。
- PA-A3/A6：两个真实 pgx 连接按 channel/锁信号控制次序，不靠 sleep 猜测。激活竞争/旧 expected revision、撤销/deny 前后提交、空 deny 表、用户禁用/成员撤销与旧/新命令均覆盖两种提交顺序；数据库会话持续时间设上限，失败释放锁。
- PA-A5：固定时间输入/边界时刻、目录/参数默认拒绝、同条目完整匹配、域名不授权直接 IP、私网进入待检、runtime_checks 完整。纯 policy/target 测试不需要任何目标服务，测试构造不注入外部执行接口。
- PA-A7：多 goroutine 同 command/异参数竞争、同资产多 command、跨 actor replay；DB 触发器注入必需审计/outbox 失败，确认 ledger/业务/decision 均为零；受理后断开响应，原 ID 查询返回原 checked_at，不发新命令。
- PA-A8：Testing Library 测用户行为/键盘/aria，而非 setState 次数；Playwright 使用真实 API、PG、合成 OIDC 和大于一页合成数据，验证分页/返回/冲突输入/会话切换/unknown 恢复/窄屏。服务端拒绝优先于前端隐藏按钮。
- PA-A9：随机 schema 先执行 00001/00002 并写合法旧历史，再 00003，核对旧 actor/task/audit/outbox 未变；直接篡改已保存正文拒绝，状态变更可审计；重启后资产/授权/decision/command 可恢复。

现有 E2E 会撤销固定成员，不能依赖再次 provision 覆盖 revoked 状态。新测试使用独立合成 project/user 或仅测试环境可用的受控夹具；每轮随机 schema/独立数据，生产服务不加入测试重置接口。清理只删除测试拥有的 schema 或 fixture，禁止删除卷/数据库历史掩盖失败。

## 4. 工具和准确命令

从根目录使用 `scripts/tooling.py` 检查已锁定本地 Go/Node/pnpm；需要临时 PATH 时只在该进程注入 `.tools` 路径，不改系统/用户 PATH。DSN/认证配置由已存在显式环境传入，先检查是否存在、不打印其值、不自动 source .env。

生成步骤按现有入口：Go 使用固定 sqlc (`scripts/toolchain.json`) 对 `backend/sqlc.yaml` 生成，Web 使用本地 pnpm 执行 `web/package.json` 的 generate。示意中的 `<本地 go>`/`<本地 pnpm>` 必须由 tooling 解析，不能原样执行或升级工具版本。

```text
python scripts/dev.py database
python scripts/dev.py migrate
python scripts/dev.py migration-status
<本地 go> run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate -f sqlc.yaml  （工作目录 backend）
<本地 pnpm> generate                                                     （工作目录 web）
python scripts/check.py --scope backend --integration
python scripts/check.py --scope web
python scripts/verify_restart.py
```

完整浏览器验收使用统一入口，负责构建/迁移/新随机身份与双项目/独立 API 和 issuer；Playwright 启动 Web，finally 只收尾自有进程。重复运行更换显式夹具身份，不恢复已撤销成员：

```text
python scripts/verify_authorization_e2e.py --environment-file <Git 忽略目录的显式环境映射>
python scripts/check.py --scope text
git diff --check
```

backend check 已含 gofmt/goimports、sqlc 漂移、mod verify、vet、单元和真实数据库测试；web check 已含根格式、lint、类型、生成漂移、Vitest 和 build。同一变更通过后无需机械重复。Go race 仅在具备支持的 C 工具链时运行 `python scripts/check.py --scope backend --integration --race`；Windows 不支持则列为未执行，并在 Linux CI 实跑后登记，不能把配置当通过。

`verify_restart.py` 本切片扩展为创建/恢复新领域对象，仍只使用合成 OIDC/内部控制面；保留旧草稿重启回归，不连接目标。实际运行的命令、环境、结果和 PA-A 映射写入验收文档，失败/跳过/环境不可用均明确保留。

## 5. 风险、回滚与审查

- 权限 helper 会触及旧命令入口：独立 PG 并发及旧四类记录回归是合并前必需检查，避免以新领域测试替代兼容性。
- 新稳定 policy 行、版本 pointer/状态及 command replay 顺序是高风险处：无行依赖失败，先 permission/幂等回放、后 expected revision/业务变更，约束与测试共同证明不覆盖历史。
- 公共后缀表和 IDNA 依赖升级可能改变匹配：target-v1 内容不重写；新算法版本需新授权及迁移设计，升级记录具体版本。
- 列表数量未知、URL exact、无执行凭证须在页面与 API 同时说明，不以精美 shadcn 控件隐去语义限制。
- migration 只扩展新表：部署先迁移后 API，回滚代码保留新历史/停新入口并前向修复；Down 仅随机可丢弃测试 schema。停止临时服务只处理本任务创建的句柄。

规划阶段检查 task manifests、需求/验收映射、链接、UTF-8/LF 与 diff；产品检查只在实施后运行。独立 check 必须读最新 PRD/design/implement，修复漂移后登记验收；验证通过再更新实际 spec、GitHub 用户作者提交/推送及 Trellis 归档记录，不擅自 merge/publish。父任务继续跟踪完整 P0–P2，不因第三切片完成而宣布平台 V1 交付。

## 6. 实际交付

00003/00004前向升级、共享权限锁、冻结范围账本、受控字段错误与跨页面恢复已落盘；backend完整真实PG、Web56测试、12个真实浏览器场景和API重启通过。最后修复手机项目入口与复制按钮字色并重跑Web/E2E。验收文档登记中文抽查、截图、竞争/故障矩阵和未执行race/云CI/企业IdP。完整平台父任务仍在实施；shadcn-admin真实底座整合后续独立规划，不能把当前官方shadcn控件等同于完整底座。
