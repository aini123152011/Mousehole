# ARTEX 平台重建方案

## 目标

仔细梳理 ARTEX 的实际产品和代码，以其已验证的业务闭环规划一套面向单企业安全团队私有化部署的授权安全验证平台。保留资产知识、双图、Agent 认知组织、证据和发现闭环，重建运行协调、授权边界、企业身份和交付机制。方案阶段交付研究、需求、架构和实施路线；用户已于 2026-10-12 批准从 P0 开始，首个实施切片见 [工程骨架与命令闭环](../archive/2026-10/10-12-p0-command-foundation/prd.md)。总体平台仍在建设，实际交付以 [根 README](../../../README.md) 和 [验收记录](../../../docs/validation/p0-command-foundation.md) 为准。

用户已选择高自主 Agent 执行，最高操作强度为主动枚举与非破坏性验证，并明确要求新平台比 ARTEX 更强大、更好用。平台的价值是缩短从已授权任务到可复核发现的周期，让每次真实执行可追溯、可中断、可恢复，并说明未覆盖和结果未知的部分。产品改进以自主任务完成、故障恢复、发现可复核和分析员操作效率衡量；组件数量或页面数量不能替代这些结果。

本次规划还交付 [统一中文代码编写规范](coding-standards.md) 与 [注释、配置及文档模板](coding-templates.md)。用户要求手写注释等格式统一、完整、使用中文；因此规范覆盖命名/目录、格式、中文说明、接口/数据、错误/日志、测试、迁移、文档与审查，机器标识和工具固定语法保持稳定。规划验收包括模板覆盖、跨文档合同一致、链接有效及开发/审查上下文可加载；实际工具链和代表模块在后续实施中验收。

## 背景与已核实证据

ARTEX 使用 Go、PostgreSQL 和静态导出的 Next.js/React Web，已经形成资产/范围、任务、探索图、工具/流量、独立发现、证据、复测、报告、归档和通知闭环。全局资产图与任务探索图通过 anchor 连接行动、事实和发现，漏洞还可回链到流量及证据。[README.md](E:/Crazywhite/ARTEX/README.md:354) [资产 schema](E:/Crazywhite/ARTEX/db/schema.sql:40) [探索图 schema](E:/Crazywhite/ARTEX/db/schema.sql:153) [anchor schema](E:/Crazywhite/ARTEX/db/schema.sql:276)

它并非没有持久化和恢复：任务、意图、活动、模型健康等已落库；通知有 outbox/投递 lease，归档有数据库队列。需替换的是与本机 Manager/Engine、goroutine、sync.Map 和 triggerQ 耦合的 Agent 执行协调。[engine.go](E:/Crazywhite/ARTEX/server/engine.go:93) [运行字段](E:/Crazywhite/ARTEX/server/server.go:77) [启动恢复](E:/Crazywhite/ARTEX/server/server.go:264)

工具拦截、资产 block/allow 闸门和证据 staging/hash 校验值得继承，但资产闸门主要作用于资产入库与意图提交，不等于网络出口授权。ModeBypass、默认允许的工具匹配、单管理员 JWT 和同容器执行不适合企业高自主边界。[worker.go](E:/Crazywhite/ARTEX/agent/worker.go:434) [intercept.go](E:/Crazywhite/ARTEX/server/intercept.go:474) [asset_intercept_match.go](E:/Crazywhite/ARTEX/db/asset_intercept_match.go:178) [auth.go](E:/Crazywhite/ARTEX/server/auth.go:86) [store.go](E:/Crazywhite/ARTEX/evidence/store.go:60)

源码证据、完整功能盘点、各项保留/替换/后置决定见 [artex-review.md](research/artex-review.md) 及其中三个专项审查。本次仅静态检查，未修改 ARTEX 或运行其服务、构建、测试、扫描；不能把静态结论当作性能、隔离或模型效果的实测证明。

ARTEX 已实际使用 shadcn/ui 与 Radix；采用 shadcn/ui 是延续成熟组件基础，改进来自统一工作台、数据与操作语义。[components.json](E:/Crazywhite/ARTEX/web/components.json:2) [Button](E:/Crazywhite/ARTEX/web/src/components/ui/button.tsx:3) 用户已批准首个切片，并已采用 React/TypeScript/Vite 与 shadcn/ui；选型研究与官方资料见 [前端选型研究](research/frontend-stack-and-ux.md)。

## 首版范围

V1 为 P0–P2 的完整交付，包含：

- 单企业私有化、企业 OIDC、项目/业务单元、成员/角色、资产登记与版本化授权书。
- 全局资产图、任务探索图、锚点、模板和任务历史；继承有出处的事实，保留其时间和有效期。
- 持久化工作流、结构化 Action、隔离 Runner、目标出口策略、审批、暂停/取消和故障恢复。
- 高自主 Planner/Observer、持久化共享 todo、按任务/意图查询的 Worker trace、模型 profile/健康降级、预算及停止规则。
- 活动时间线、快照/cursor SSE、任务对话、默认只读侧问、以范围版本为分母的基础覆盖。
- 独立发现、不可变证据包、复测、固定 finding/evidence 版本的 Markdown/HTML 报告、冻结导出及过期提示、站内通知和审计。
- 基础备份恢复、最小指标/告警、可升级的私有化部署包；六个工作区为项目与范围、任务、发现与复测、证据、审批、平台设置。
- 统一任务工作台、列表/图/证据联动、可恢复筛选与工作位置、明确数据新鲜度和命令受理状态、键盘可达的核心操作，以及与 ARTEX 可比较的操作场景。

## 范围外

- 未经授权、无有效匹配或超出时窗的执行。
- 主动写入目标业务数据、创建/修改身份、改变配置、持续驻留、破坏性/不可逆利用和可用性影响测试。V1 所有执行路径均拒绝，人工审批也不能解锁；普通访问引起的日志/缓存更新不被解释为获准的业务写入能力。
- 任意 Bash、任意 payload、运行时上传执行脚本、未审核 MCP/Skill 执行能力和公共扩展市场。
- 多租户 SaaS、MSSP 隔离/计费、跨客户 Worker 池。
- 全量 MITM、未有专用校验适配器的 UDP/ICMP/原始套接字探测；全量资产同步、跨任务自动语义推理、向量记忆、完整归档恢复、复杂报告模板和外部通知属于 P3。
- 直接迁移 norma SDK 和单进程编排。具体源码复用需核对文件来源与许可证，默认继承产品概念、行为契约和验收场景。

## 产品需求

1. 范围即执行边界：每个 Run 固定版本化授权书，包含资产/CIDR/域名、解析与重定向规则、适配器/参数、操作上限、速率、时窗、联系人、凭据规则与策略版本，同时叠加当前紧急 deny、撤销和平台硬上限。不存在有效完整匹配时拒绝执行。审批只批准已经满足授权的具体 Action，不能扩大范围、解锁禁止操作或修改策略；变化通过新授权版本重新计划，撤销旧版本停止新执行。
2. Agent 不持有长期网络、系统或密钥权限。控制面签发绑定 Action/Runner/TTL 的单次领取凭证，原子兑换 lease，再签发绑定 lease/fencing/目标/预算的短期执行 capability 和最小机密；多请求 Action 在 capability 上限内执行，每个目标连接均复核。允许明确授权的企业私网 CIDR，但平台基础设施、metadata、loopback/link-local 地址另行禁止。TLS 应用语义由受信任适配器校验，普通 L4 网关不承担加密正文检查。
3. 任务支持 draft/planned/queued/running/paused/waiting_approval/completed/failed/cancelled；Action 与 Attempt 独立建模，包含持久 awaiting_approval 与 Attempt in_flight/outcome_unknown。转换有幂等键、审计、超时和取消路径。每项 Action 只有一个有效 lease，使用 fencing generation 拒绝过期 Runner 的新连接/提交；已发请求仍可能在途，不承诺网络 exactly-once。结果未知不伪装为失败/取消，只有明确安全可重放且前次 Runner 已隔离的适配器允许自动重试。暂停/取消停止新目标请求，记录撤销/停止的请求与确认，完成前界面显示停止中。
4. 行动产生结构化活动、Action/Attempt/Runner 身份、工具/镜像版本、授权与策略决定、目标快照、时间、输入输出摘要、证据引用及 full/partial/unavailable 采集覆盖。Finding 必须引用真实 Evidence Package 和 Action；候选、机器验证、人工确认与处置分开。verified 由独立验证器确认授权、足够证据、可复核来源、明确环境/时间窗口，模型推测或正常退出不能自动确认。必需采集不可用停止后续验证，覆盖不足不能生成 verified。保留 staging/hash 与证据独立快照；哈希/签名不单独证明被攻陷 Runner 的陈述真实。
5. 复测独立于原任务，锁定 finding 和上下文，同 finding 仅一个活跃复测，输出 reproduced/fixed/inconclusive；只有完成且明确 fixed 更新发现状态。证据在配置保留期/法律保全期内不可改写，期满经审计删除或销毁数据密钥。ReportVersion 固定 finding_version、evidence_version、授权/覆盖与模板版本，生成/导出冻结原索引与内容；发现/证据变化后旧报告显示过期，可查看原版并重新生成。这个基础报告与复核导出合同在 V1 完成。
6. V1 支持企业 OIDC、项目/资产级 RBAC+ABAC、会话/CSRF、审计导出、对象访问审计与保留策略。令牌不进入 URL 或浏览器持久化存储。已锚定审计用独立权限域 WORM checkpoint 检测改写/缺口，不声称可抵御同时控制应用、密钥和外部锚点的全域管理员。SAML/SCIM 后置。
7. 模型可生成计划、工具提案和结果解释；授权、操作等级、密钥、审批和保留由确定性代码裁定。ToolCapability 固定 schema、协议/端口/方法、速率/并发、凭据模式、重放属性和操作等级。未知工具/参数拒绝，wrapper/deferred/MCP 内层实际能力也经过统一 admission；HTTP 方法本身不足以证明非破坏性。策略、关键审计或必需证据采集/持久化故障时停止新外部动作，普通 UI/指标故障不必阻断执行。
8. 首版保留双图、todo、trace 与任务历史。Finding 为权威对象，图节点为 outbox 投影；业务对象、活动和 durable event 同事务提交。todo 随计划版本持久化，不因 provider 重建丢失；trace 按 task/intent/run 分页检索，模型输入有限。侧问默认只读，执行请求必须产生同样受控的 Action。
9. 全局资产知识不等于当前授权；旧任务的事实可引用，授权/审批/凭据不可继承。覆盖分母固定授权范围版本及检查维度，展示未测/完成/失败/拒绝/不适用/结果未知及原因，不把 fact anchor 等同已完成验证。
10. 每个模型角色有获准主/fallback profile、健康与降级记录，设置 Token/费用、回合、时间、请求数、并发预算和无新证据停机条件。显式链耗尽停止，不回落全局；chain revision/CAS 防旧请求覆盖配置，流式 committed 后不当次 failover，不重放已发工具请求。项目声明允许模型目的地与发送数据级别，凭据和未脱敏敏感证据默认不发往外部模型；目的端点变化不隐式沿用旧秘密。
11. SSE 从持久化活动序号/快照 cursor 衔接历史与实时，广播只唤醒，再查询 durable events，支持断线恢复、去重和权限过滤；溢出不能跳过缺口。业务 durable event 原子提交，通知由可重试消费者派生，投递失败不丢业务事件。DB→Temporal 命令通过同事务 outbox、稳定 Workflow ID 和 reconcile 处理跨系统故障，不维护第二份可独立修改的动作真相。
12. 私有化生产有独立控制/执行部署单元、非 root Runner、资源/网络限制、镜像/配置版本、编号 migration、备份恢复与最小可观测性。集成 CI 必须实际启动所需数据库/对象存储/工作流依赖，不能将 skip 当成数据库验证通过。扩展多 Runner 池不改变 Action/证据契约。
13. 工作台以分析员完成任务为组织原则：任务概况展示真实执行进度、阻塞/未覆盖原因、预算、待审批与未知结果，并可进入相应明细。同一 finding 在全局、任务和复测入口使用相同 ID/版本。列表统一服务端筛选/排序/分页、总量或明确数量未知及截断提示、URL 筛选和返回位置恢复；批量选择/导出明确作用范围。双图按需展开，保持视口/选中节点并提供列表替代视图。界面区分 loading/empty/error/stale/reconnecting，显示最后同步时间；关键命令区分提交、受理、执行与确认完成，客户端发送前持有命令 ID/幂等键，服务端持久化受理，超时按该 ID 查询结果且不重复产生业务操作。核心流程支持键盘、可见焦点、文本状态与一致密度，敏感凭据不进入可恢复草稿。使用同一批合成场景对照 ARTEX 的任务完成率、查证步骤、操作时间、误操作及恢复表现，在取得基线后确定性能和效率门槛。

## 产品验收标准

1. 管理员可激活授权书；范围外、未批准工具/参数、过期时窗与平台保留地址在目标连接前拒绝，决策可查询；明确授权的私网目标可执行。当前紧急 deny/撤销对旧 Run 同样生效。映射 R1/R2/R7。
2. Agent 能自主完成批准的枚举/非破坏性验证模板，节点故障后恢复；重复投递仅产生一个有效 lease，旧 generation 不能发起新连接或污染结果。故障后尚在途请求与 outcome_unknown 可见，仅符合 replay policy 才重派。映射 R3/R7/R10/R12。
3. 任一 Finding 可回溯到任务、授权版本、Action/Attempt/Runner、策略、工具版本、摘要及 hash 校验的证据；篡改内容校验失败，保留期内原始对象不可改写。full/partial/unavailable 可查，必需覆盖不足时不能生成 verified。映射 R4/R5/R6。
4. 仍符合授权但需复核的强度/凭据/速率请求等待审批；批准绑定真实 actor、Action hash/目标/适配器/授权/策略/TTL，修改绑定字段需重新审批。审批不能解锁范围外或禁止操作；暂停/取消停止新调度/新目标请求、撤销凭证并记录出口/Runner 停止确认，在配置撤销传播上限内验证，完成前显示停止中，已发请求显示在途或未知。一个 Action 待审不阻塞其他允许工作。映射 R1/R2/R3/R7。
5. 同 finding 并发复测去重；只有完成且明确 fixed 改变状态，其余结论追加历史。报告与 manifest 固定 finding/evidence 版本，冻结导出不混入并发编辑；变化后旧报告显示过期，原版本仍可复核。映射 R4/R5。
6. 交付环境验证 OIDC、权限隔离、TLS、cookie/CSRF、SSE 权限、非 root Runner、实际出口无旁路、对象访问、备份恢复和签名审计 checkpoint。映射 R2/R6/R11/R12。
7. 同一业务/Action/证据契约可扩展多个隔离 Runner 池，升级/回滚保留历史状态和证据。映射 R3/R12。
8. 双图能关联计划、执行与证据；模型切换/进程重启后 todo 和 trace 仍可查询，迟到事件不破坏投影。SSE 在保留窗口内断线/缓冲溢出重连无遗漏、按 seq 去重，超过窗口明确重快照；侧问不隐式执行。DB 提交后 workflow 尚未启动的故障可恢复而不重复创建同 run。映射 R8/R11。
9. 新资产不自动扩大授权；覆盖分母在固定版本下稳定，失败/拒绝不会被计为完成，各未覆盖项有原因。映射 R1/R9。
10. 预算或无进展阈值触发停止并解释原因；模型链耗尽、旧 revision 迟到和 midstream 故障不越过获准链或重放已提交工具。恶意工具输出不改变策略；模型目的地限制与端点秘密重绑定生效。策略、关键审计、必需证据故障均停止新动作，不能发布无足够证据的 verified finding。映射 R4/R7/R10。
11. 在合成任务中，分析员可从任务概况进入阻塞 Action、待审批、未覆盖项与未知 Attempt，并沿 finding → Action/Attempt → Evidence 完成复核。全局/任务/复测内容版本一致；列表返回恢复筛选/列设置/工作位置，增量刷新不重置图视口，批量操作/导出展示准确范围，截断有明确提示。键盘能完成创建任务、查找发现、查看证据和暂停任务，图有可访问的列表路径。映射 R3/R4/R8/R9/R13。
12. 刷新、断线回补、请求超时与并发编辑后，界面显示连接/数据状态；即使受理响应丢失，仍可用发送前已知的命令 ID 查询受理/执行结果，相同幂等键不重复变更业务，相同键不同参数拒绝。冲突保留用户输入并提示重载。使用相同合成数据与角色对照 ARTEX，提交任务完成率、查证步骤、耗时、误操作、列表/图响应与断线恢复记录；未取得基线前不声称具体性能提升。映射 R3/R5/R11/R13。

## 关键决策与技术建议

用户决定为单企业私有化、高自主执行、主动枚举与非破坏性验证上限，产品目标为能力与易用性优于 ARTEX。其余为本次基于源码提出的技术建议：Go 模块化控制面、PostgreSQL、Temporal、React/TypeScript/Vite + shadcn/ui 静态 Web、S3 兼容证据存储、OIDC、Kubernetes 隔离 Runner 和受控出口。前端进一步推荐 React Router、TanStack Query/Table、React Hook Form/Zod 和一种主要图引擎；版本在实施时锁定。P0–P2 才是 V1；不并行建设第二套生产调度器。

## 风险与后置事项

待隔离 PoC 验证的技术项是实际出口强制约束、适配器只读语义、长任务/图/SSE 容量、模型质量和私有化运维成本。提示注入用结构化 Action 与确定性策略隔离；大证据采用最小采集、脱敏、对象存储和保留规则。HA、KEDA、完整日志追踪、强化沙箱和新数据引擎按负载/交付需求增加，不先于业务闭环。

无阻塞产品问题。规划状态保持 planning；本次建议交付不启动实施。完整架构和建设顺序见 [design.md](design.md) 与 [implement.md](implement.md)。
