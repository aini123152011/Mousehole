# ARTEX 梳理与重建取舍

审查范围为 `E:\Crazywhite\ARTEX`，未修改源码、未启动服务、未运行构建或扫描任务。

## 产品闭环

ARTEX 的有效产品闭环是：资产和范围管理、任务执行、工具/流量留痕、漏洞及证据、复测、报告、归档和通知。README 将架构概括为 Go 单体、PostgreSQL、Next.js、SSE、Manager/Engine、planner/worker、全局资产图与任务探索图。[`README.md`](E:\Crazywhite\ARTEX\README.md:294)

每个任务的探索图有 `goal`、`intent`、`fact`、`finding` 与 `hint` 节点，通过 anchor 关联全局资产图；这比仅保存 Agent 对话更能解释一个结论为何出现。[`README.md`](E:\Crazywhite\ARTEX\README.md:354) [`schema.sql`](E:\Crazywhite\ARTEX\db\schema.sql:153)

复测是一个可独立复用的模式：绑定已有 finding 与上下文，单 finding 同时仅运行一次，结果为 `fixed`、`reproduced`、`inconclusive`，并保留原始证据。[`finding_retests.go`](E:\Crazywhite\ARTEX\server\finding_retests.go:20)

## 值得继承的部分

- 双图数据模型：`assets`、`explorations`、`nodes`、`edges`、`anchors` 构成资产和行为证据的关系图。[`schema.sql`](E:\Crazywhite\ARTEX\db\schema.sql:40) [`schema.sql`](E:\Crazywhite\ARTEX\db\schema.sql:276)
- 活动先持久化、再通过 SSE 推送，并支持快照 cursor 和事件序号衔接历史与实时视图。[`engine.go`](E:\Crazywhite\ARTEX\server\engine.go:635) [`server.go`](E:\Crazywhite\ARTEX\server\server.go:2730)
- 工具批准规则允许 allow、deny、ask，人工审批可回溯到原始活动。[`intercept.go`](E:\Crazywhite\ARTEX\server\intercept.go:474) [`intercept.go`](E:\Crazywhite\ARTEX\server\intercept.go:514)
- 证据存储先使用 staging 区，校验长度与 SHA-256、同步落盘，再在数据库事务中绑定，具备较好的完整性基础。[`store.go`](E:\Crazywhite\ARTEX\evidence\store.go:60)

## 重建时必须替换的部分

运行时以进程内状态为主：Engine 使用 `sync.Map` 保存运行信息，Server 的 triggerQ 是 map，注释明确承认重启会丢失等待触发；恢复依赖重置状态和重新启动 goroutine。[`engine.go`](E:\Crazywhite\ARTEX\server\engine.go:93) [`server.go`](E:\Crazywhite\ARTEX\server\server.go:77) [`server.go`](E:\Crazywhite\ARTEX\server\server.go:264)

Agent 侧同时具备 Bash、WebFetch 和扩展工具，worker/planner 配置 `permission.ModeBypass`；MITM 留痕不是隔离边界。[`worker.go`](E:\Crazywhite\ARTEX\agent\worker.go:373) [`worker.go`](E:\Crazywhite\ARTEX\agent\worker.go:434) [`planner.go`](E:\Crazywhite\ARTEX\agent\planner.go:409)

拦截规则未匹配时默认允许。资产闸门中 block 始终优先；有 allow 规则时必须命中 allow，没有 allow 时默认允许。它主要作用于资产入库与意图提交，未统一覆盖最终网络连接；因此缺少 allow 时没有正向范围边界，不能直接作为高自主系统的执行授权模型。[`intercept.go`](E:\Crazywhite\ARTEX\server\intercept.go:505) [`asset_intercept_match.go`](E:\Crazywhite\ARTEX\db\asset_intercept_match.go:178) [`db.go`](E:\Crazywhite\ARTEX\db\db.go:288)

当前认证是单管理员密码与 7 天 HS256 JWT，前端从 `localStorage` 读取 token，SSE 通过 URL 参数传递；它适合可信内网单管理员，不是企业身份基线。[`auth.go`](E:\Crazywhite\ARTEX\server\auth.go:48) [`auth.go`](E:\Crazywhite\ARTEX\server\auth.go:86) [`api.ts`](E:\Crazywhite\ARTEX\web\src\lib\api.ts:109) [`api.ts`](E:\Crazywhite\ARTEX\web\src\lib\api.ts:165)

随仓 Dockerfile 将服务、代理、扫描工具、Node、Playwright 和 Chromium 放在同一个容器内，未设置非 root USER；随仓 Compose 未补独立执行隔离和资源/网络边界。这是默认交付配置的观察，不能据此断言所有外部部署均以 root 运行。[`Dockerfile`](E:\Crazywhite\ARTEX\Dockerfile:35) [`docker-compose.yml`](E:\Crazywhite\ARTEX\docker-compose.yml:2)

## 安全与部署结论

通知 HTTP 客户端已有可继承的拨号阶段地址校验：拒绝 loopback、link-local、unspecified、multicast，处理 IPv4-mapped IPv6，并限制跨主机重定向。它明确允许 RFC1918 私网以支持内部通知服务；ARTEX_NOTIFY_ALLOW_LOCAL 可放宽本地限制，默认 transport 的代理行为仍需单独评估。因此应继承其连接时复核思路，不能说它拒绝所有私网或已经保证所有代理路径的 SSRF 防护。[`http.go`](E:/Crazywhite/ARTEX/notify/http.go:41) [`http.go`](E:/Crazywhite/ARTEX/notify/http.go:59)

流量正文默认可保存明文，较大内容按 SHA-256 地址化。新平台需要默认最小采集、敏感字段剔除、保留策略、版本化对象存储和不可变审计链。[`traffic.go`](E:\Crazywhite\ARTEX\traffic\traffic.go:89) [`traffic.go`](E:\Crazywhite\ARTEX\traffic\traffic.go:541)

结论是保留领域数据闭环和证据完整性做法，替换为持久化工作流、默认拒绝的范围策略、隔离 Runner 与企业身份体系。

## 审查方法与证据边界

本记录来自 README、依赖清单、PostgreSQL schema、Go 服务/Agent/存储实现、前端页面与 API、部署和测试代码的静态检查。领域审查逐项覆盖 21 个侧栏入口和 10 个任务详情标签，另外两个专项覆盖执行/恢复/模型及权限/网络/证据，详细记录见 [领域与 UI 审查](artex-domain-ui-audit.md)、[执行审查](artex-execution-audit.md)、[安全与部署审查](artex-security-audit.md)。专项中的 Python Agent、VM Runner 或 PG 调度等是可比较的备选，本记录和 design.md 才记录最终建议选型。静态检查可以确认代码分支、领域契约和部署配置；不能证明实际性能、生产隔离效果、模型发现质量或测试当前是否通过。本次未运行 ARTEX 的服务、构建、测试或目标请求。

这里的“保留”默认指保留产品概念、行为契约和验收场景，具体源码复用另行逐文件判断。仓库根许可证为 AGPL-3.0，`web/LICENSE` 为 MIT，不能以局部前端许可证概括整个产品；`go.mod` 还说明 norma 上游不再公开，项目使用恢复到本地的 v0.4.3。应在复用前核对文件来源和相应义务，新核心通过自己的契约实现，避免把 SDK 的维护状态带入产品。[`LICENSE`](E:/Crazywhite/ARTEX/LICENSE:1) [`web/LICENSE`](E:/Crazywhite/ARTEX/web/LICENSE:1) [`go.mod`](E:/Crazywhite/ARTEX/go.mod:3)

## 全量功能盘点与版本取舍

P0、P1、P2 是同一 V1 的三个建设阶段；必须完成 P2 才满足用户选择的高自主首版。P3 是首版之后的增强，不能把缺少 Agent、恢复或隔离的 P0 演示称为 V1。

| ARTEX 现有能力                                | 产品价值                       | 新平台取舍                                                                                                                 | 阶段与依据                                                         |
| --------------------------------------------- | ------------------------------ | -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| 公司、公司范围与资产归属                      | 让资产有业务归属，减少重复录入 | 单企业下改成项目/业务单元，不因此建设 SaaS 租户层                                                                          | P0；`db/schema.sql:27,115`                                         |
| domain/IP/service 资产、标签与关系            | 跨任务累积资产事实             | 保留全局资产图，补观测来源、时间和置信度；首次用 PostgreSQL 节点/边表                                                      | P0；`db/schema.sql:40`                                             |
| 任务范围、任务资产链接、任务关系              | 复用已有资产和历史上下文       | 保留显式关联和来源；继承上下文不继承授权、凭据或执行许可                                                                   | P0；`db/schema.sql:636,648,738`                                    |
| 任务模板、类别、置顶、批量组织                | 把常用评估流程标准化           | 首版保留少量已批准模板和基本分类；高级批量管理后置                                                                         | P0/P3；`db/schema.sql:493,613`                                     |
| 探索图与资产 anchor                           | 解释行动、事实与发现对应的资产 | 保留双图；明确展示计划/已执行/已证实；图是查询投影                                                                         | P0/P2；`db/schema.sql:153,170,248,276`                             |
| Planner frontier 与意图优先级                 | 把大目标拆成可并行工作项       | 保留计划 DAG、依赖和优先级；调度改为持久化工作流                                                                           | P1/P2；`README.md:392`                                             |
| Planner 多轮共享 todolist                     | 避免重启计划后忘记未完成方向   | 保留带版本、状态和来源的待办；与 Action/证据互相引用                                                                       | P2；`README.md:438`                                                |
| Worker 过程记录查询与交换                     | 利用进行中的发现，避免重复试探 | 保留 task-scoped trace 检索和结构化事实摘要；不把无限聊天记录塞回模型                                                      | P2；`README.md:419`                                                |
| 主 Agent、Worker/Planner 会话与 side question | 让人解释目标、了解过程和介入   | 保留任务内对话、历史和停止入口；侧问默认只读，不隐式启动工具                                                               | P0/P2；`db/schema.sql:346,954`                                     |
| 活动流、历史快照与 SSE                        | 不刷新页面即可追踪长任务       | 保留 cursor/seq 语义，增加事务 outbox、权限过滤与重连测试                                                                  | P0/P1；`server/server.go:2736,2874`                                |
| 任务暂停、取消、排队与并发控制                | 控制执行成本和影响             | 保留用户语义，改用 lease、fencing、恢复和可观察的停止完成状态                                                              | P1；`server/engine.go:93`                                          |
| 拦截 allow/ask/deny 与审批细节                | 将人工介入和执行记录关联       | 保留可解释审批 UI；以授权书和 ToolCapability 默认拒绝替代默认允许                                                          | P1；`server/intercept.go:474,505`                                  |
| LLM profile、协议适配、retry/health/failover  | 降低供应商故障对任务的影响     | 首版保留每角色 profile、健康/降级、预算；模型切换不重放未知结果的工具                                                      | P2；`db/schema.sql:362,479,696`                                    |
| Prompt、MCP、Skill 与自定义工具               | 扩展专业能力                   | 首版仅允许受版本控制的提示和签名适配器；后续私有注册表也必须走同一能力目录                                                 | P2/P3；`agent/toolcatalog.go`、`agent/tools.go`                    |
| 流量、工具输出、附件证据                      | 支撑复核和复现                 | 继承 staging/hash 思路；正文按任务最小采集，脱敏、加密、受控访问                                                           | P0/P1；`evidence/store.go:60`、`traffic/traffic.go:89,541`         |
| 独立 findings 与资产关联                      | 从模型叙述转为可管理结论       | Finding 成为权威对象，图节点为事务一致的投影；保留人工确认和证据绑定                                                       | P0；`db/schema.sql:1081`                                           |
| finding retest 快照和结论历史                 | 验证修复而不重启原任务         | 首版保留独立复测、同 finding 并发去重、三态结论                                                                            | P0/P2；`db/schema.sql:1112`、`db/finding_retests.go`               |
| 报告与导出                                    | 形成团队可交付结果             | 保留证据版本/CAS、冻结导出；首版 Markdown/HTML 与 manifest 固定 finding/evidence 版本并显示过期；复杂模板/PDF/工单联动后置 | P0/P3；`evidence/store.go:118,265`；详见领域审查                   |
| 资产覆盖图与已测高亮                          | 显示遗漏方向                   | 首版以授权范围快照为分母，分别记录未测/完成/失败/拒绝/不适用/结果未知；高级热图后置                                        | P0/P2/P3；`README.md:360`                                          |
| 通知渠道与投递历史                            | 提醒审批、完成、失败与发现     | 首版站内事件；邮件/webhook 与限流、签名、重试、脱敏后置                                                                    | P0/P3；`notify/`、`db/notification_delivery.go`                    |
| 任务归档、包导出与恢复                        | 保留长期历史，降低活跃数据量   | 首版完成可验证导出；完整归档恢复后置，恢复后不自动恢复执行授权                                                             | P0/P3；`db/schema.sql:571`、`db/task_archives_restore.go`          |
| ScopeSentry 等资产同步                        | 降低资产维护成本               | 首版手动/API 导入；同步要保留来源，外部新资产不会自动进入授权书                                                            | P0/P3；`README.md:57`                                              |
| 单二进制、静态 Web、多平台发布与自更新        | 降低研究环境安装门槛           | 保留静态 Web 和易部署体验；生产用签名镜像、离线包、受控 migration 和回滚                                                   | P0/P3；`web/next.config.mjs:3`、`.github/workflows/release.yml:11` |

## 数据模型：保留双图，补上权威对象和时间边界

ARTEX 的图实际保存在 PostgreSQL，产品需要图可视化并不意味着需要 Neo4j。已有 `exploration_nodes/edges/anchors`、JSONB、GIN/GiST 索引给了一个低运维成本的起点。新平台先采用关系表表达身份、授权、计划、执行、证据、发现，用查询投影生成图；只有图遍历负载成为瓶颈才评估图数据库。[`schema.sql`](E:/Crazywhite/ARTEX/db/schema.sql:170) [`schema.sql`](E:/Crazywhite/ARTEX/db/schema.sql:95)

建议的关系是：企业/项目 → 授权书版本 → Task/Run → PlanRevision/Intent/Action → Attempt/Observation → Evidence → Finding → Retest/Report。Asset 由 observation 更新，Anchor 关联计划节点与资产。授权版本、运行目标快照和证据不会随资产标签或 IP 的后续变化而被改写。

要明确四组容易混淆的概念：资产事实不等于执行授权；待办或图上的 intent 不等于已执行 Action；模型推测不等于已证实 Finding；网络 attempt 完成不等于业务结论成立。界面、数据库状态和报告都要保持这个区分。Graph finding node 与 Finding 独立双写不应成为长期一致性方案，更新业务对象、活动和 outbox 应在同一事务内完成，再生成图投影。

跨任务复用优先复用有出处、时间和失效条件的事实及证据引用。旧任务范围、账号和审批不能随上下文继承。首版保留全局资产图；跨任务自动知识推理、向量记忆和广泛的语义检索后置。

## 源码级取长补短清单

以下逐项对应现有实现与重建合同；具体故障影响来自静态逻辑推导，未进行生产故障复现。专项审查的风险 P0/P1 与本方案建设阶段 P0–P3 含义不同。

| 具体实现                                            | 应保留的长处                                              | 应补的缺口与新合同                                                                                                                                                                                                                        |
| --------------------------------------------------- | --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 任务/deadline/Intent/对话恢复，Intent CAS 认领      | 已有持久状态与重启恢复，不能概括为“没有恢复”              | running 重置为 open 缺少租约/fencing；补 Action/Attempt、未知结果、所有权与恢复协调。`server/server.go:264`、`db/exploration.go:312,1172`                                                                                                 |
| Planner 内存 todo、Worker 按需 trace                | 计划组织、固定 mandate、摘要检索与 drilldown 有价值       | provider/bundle 重建会丢 todo；计划版本落库，trace 分页，正常返回与证据完成分别判定。`agent/planner.go:43,432`、`server/task_llm.go:546`、`server/engine.go:1135`                                                                         |
| 任务模型链 cursor/revision 和流式提交边界           | quota 分类、旧请求不覆盖新链、已交付后不直接换模型重放    | 保留并验收；错误字符串分类改 typed error，多实例健康状态明确一致性。`server/task_llm.go:295,376,431`、`db/task_context.go:464`                                                                                                            |
| Finding 创建事务、证据独立副本、版本/CAS 和冻结导出 | 创建已原子化，证据可脱离原 Traffic 复核，报告版本冻结成熟 | 更新仍有 best-effort 图同步；Finding 单一权威、事务 outbox 生成图，报告明确绑定 finding/evidence 版本并显示过期。`db/finding_traffic.go:342`、`db/findings.go:712`、`evidence/store.go:118,265`                                           |
| Finding PATCH 与图读取                              | 独立发现具备完整业务字段                                  | PATCH 状态先提交、severity 后校验，可能 400 后状态已改变；请求全部校验后一次事务。图接口每任务/来源上限 200，需分页或显式截断标记。`server/server.go:2278,2299,1951,1976`                                                                 |
| 活动先落库、snapshot/cursor SSE                     | 持久活动与 UI 实时订阅已有基础                            | 256 缓冲溢出可丢事件，后续 seq 前进可能掩盖缺口；广播只唤醒，按 durable cursor 查询，或溢出立即断开重连。`server/broadcast.go:9,49`、`server/server.go:3058,3068`                                                                         |
| 通知 outbox、SKIP LOCKED、投递 lease；归档 DB 队列  | 已有可靠后台处理资产，不应重新描述为纯内存通知/归档       | 通知 savepoint 插入失败允许业务继续提交；关键领域 event 必须与业务原子提交，渠道失败仅影响后续投递。`db/notification.go:219,275,464`、`db/notification_delivery.go:211`、`db/task_archives.go:220,358,371`                                |
| 独立复测与三态结果                                  | 上下文快照、同 finding 活跃去重、fixed 的严格条件         | 继续保留，并加入 Action 身份/工具版本/证据覆盖和可比环境；一次失败不视作修复。`agent/retester.go:4`、`server/finding_retests.go:20`                                                                                                       |
| Tested 与 auto scope                                | 锚点与自动范围记录有助于探索统计                          | Tested 是 fact anchor，当前/直接来源 scope 与 anchor 合并会改变分母；它不等于检查完成。新覆盖用固定授权版本×检查维度，包含结果未知。AddAutoScope 是统计范围，不能误称自动扩大授权。`db/task_scope.go:276`、`db/task_assets_context.go:24` |
| 工具顶层 permission、Guard/人工审批                 | 已有治理，不是完全无审批                                  | deferred/custom 内层调用未统一走 SDK permission/hook，外层 ARTEX 拦截是否生效取决于配置；所有实际能力经 admission 与隔离 Runner。`norma/tool/deferred.go:217`、`server/customtool.go:331`                                                 |

关键审计与证据不能沿用 UI activity 的 best-effort 故障语义。现有 activity 保存重试失败仍可丢弃，而 intent/task 删除还会清理相应记录；新平台将可清理 UI trace 与必须留存的执行审计分开。策略、关键审计或必需证据采集/持久化不可用时停止新动作，不因模型继续输出而继续执行。[`engine.go`](E:/Crazywhite/ARTEX/server/engine.go:640) [安全专项](artex-security-audit.md)

证据再补 action/attempt/runner 身份、工具/策略/授权版本、full/partial/unavailable 采集覆盖、签名 manifest 和独立留存。SHA-256 证明保存后的完整性，不单独证明采集来源或完整；Runner 签名也不能保证被攻陷 Runner 的陈述真实。验证动作需要的证据不充分时不能产生 verified finding。[执行专项](artex-execution-audit.md)

## 执行机制：继承认知组织，替换运行控制

Planner、Worker、共享 todo、trace 查询和 exploration graph 分别解决拆解目标、执行子任务、持续记忆、共享过程、解释结论的问题；这是 ARTEX 最值得吸收的设计。新实现不需要为每个角色各造一个服务，首先保留 Planner + 执行适配器 + Observer 三种职责，模型调用通过统一网关提供。

相应的运行控制应该独立：Temporal 管持久化工作流、等待、超时和恢复；PostgreSQL 管业务对象/审计/查询投影；Runner 管一次已获准的执行；出口网关管实际连接。不能让图节点状态、SSE 是否在线或模型的一句“完成”直接决定执行结束。

ARTEX 并非完全没有恢复和持久化：任务排队、意图、活动、模型健康等已经落库，启动时还会清理遗留 running 意图并恢复已接纳任务。缺口在于这些机制仍与本机 Manager/Engine、goroutine 和触发队列耦合，不能直接提供跨节点 ownership、历史重放和丢失 worker 的确定恢复。应继承其恢复场景和测试，而不是把现实现笼统视为“没有状态管理”。[`server.go`](E:/Crazywhite/ARTEX/server/server.go:270) [`server.go`](E:/Crazywhite/ARTEX/server/server.go:284)

共享 todo 和 Worker trace 进入 V1，但要给模型设置轮次、Token/费用、执行时间、请求次数和并发预算。停止条件包括目标完成、预算用尽、剩余路径均被拒绝、连续无新证据、人工终止；停止时输出完成/未覆盖/失败的原因，避免无限计划循环。

## 工作区：围绕安全分析员的闭环组织

建议六个工作区是：项目与范围、任务、发现与复测、证据、审批、平台设置。任务详情聚合双图、计划/todo、执行时间线、会话、覆盖和成本；不要把每类 Agent 或每张底层表变成一级菜单。

默认视图应让用户直接回答“做到了哪里、依据是什么、还遗漏什么、需要我处理什么”。资产图用于定位影响范围，探索图用于解释推理与行动，时间线用于复核真实执行。大量节点时使用筛选、聚合和按需展开；全量布局、全量活动返回和无限消息列表需要分页与虚拟化。

首版延续 React/TypeScript 与 shadcn/Radix 基础，推荐统一落地 TanStack Query/Table、React Hook Form/Zod 和生成的 OpenAPI client。ARTEX 依赖表声明 G6/Graphin/XYFlow，但目前定向源码核对确认的是 G6 实际入口，不将依赖声明当作三套图引擎同时运行的证据。新平台优先一种主要图交互组件，React Flow 适合任务 DAG/谱系与适度子图，超大资产网络图另行压测。ARTEX 的 Next.js 已支持静态导出而非必需 SSR：新建管理后台可用 Vite，Go 服务处理 OIDC/session；如大量沿用现有页面则保留 Next.js 静态模式。Vite 属于工具链简化，不能声称换框架就获得新的静态部署能力或实测性能优势。[`package.json`](E:/Crazywhite/ARTEX/web/package.json:29) [`next.config.mjs`](E:/Crazywhite/ARTEX/web/next.config.mjs:30) [前端选型研究](frontend-stack-and-ux.md)

## 工程与交付：已有积累与新的质量门槛

ARTEX 有较多 Go 回归测试，覆盖任务并发/删除、范围、模型重试、审批、发现、复测、证据、流量归档和 SSRF，前端也有活动合并测试。这些业务场景适合转为新平台的验收用例，不能把“重建”理解为丢弃已有问题经验。

当前 `db/schema.sql` 同时初始化和演进，启动时整体执行；`db.go` 已用固定连接上的 PostgreSQL advisory lock 与 deadlock retry 处理并发 migration，是有意识的可靠性补丁。企业交付应进一步改为编号 migration、schema 兼容窗口和独立 migration Job，运行账号不持有建库/DDL 权限，升级前备份并演练。[`db.go`](E:/Crazywhite/ARTEX/db/db.go:45) [`db.go`](E:/Crazywhite/ARTEX/db/db.go:61) [`db.go`](E:/Crazywhite/ARTEX/db/db.go:268)

数据库不可用时部分测试会 skip，因此单独看到 `go test` 成功不能说明数据库路径被覆盖。已读的 Release workflow 执行前端构建、Go 编译、`-h` 冒烟与 checksum，但未包含集成测试和 lint/typecheck gate；这里是对该 workflow 的观察，不是对所有开发过程的推断。新平台 CI 应使用临时 PostgreSQL/对象存储/工作流环境，要求集成套件实际运行，并加入 lease 失效、结果未知、策略拒绝、SSE 衔接和证据关联场景。[`db_test.go`](E:/Crazywhite/ARTEX/db/db_test.go:54) [`core_test.go`](E:/Crazywhite/ARTEX/server/core_test.go:23) [`release.yml`](E:/Crazywhite/ARTEX/.github/workflows/release.yml:20)

依赖版本也应以仓库事实为准：ARTEX 当前声明 Go 1.26.3、PostgreSQL 16 镜像、Next.js 16.2.9、React 19.2.7、TypeScript 5.9.3。它们是当前项目基线，不是此次查证过的最新安全版本；新建项目在实施时选择受支持稳定版、锁定 patch 和镜像 digest。[`go.mod`](E:/Crazywhite/ARTEX/go.mod:8) [`package.json`](E:/Crazywhite/ARTEX/web/package.json:53) [`docker-compose.yml`](E:/Crazywhite/ARTEX/docker-compose.yml:3)

## 不应直接继承的设计假设

- 单管理员、可信浏览器，以及随仓默认配置中控制面与工具共容器、未声明非 root USER 的交付假设，不适合多人协作的企业私有化平台。
- 自由 Bash、任意 MCP/Skill 或自然语言“请不要越界”，不能承担执行边界；这些扩展都要变为可验证能力。
- 代理环境变量和 MITM 捕获只覆盖遵从代理的流量，不能证明所有网络请求均受控。
- 资产出现在图里、属于某公司或被旧任务发现，不表示当前任务可以扫描它。
- “已测”不能代表所有检测项都完成，也不能代表没有漏洞；覆盖必须有快照分母和检查维度。
- 模型摘要或工具退出码不能直接发布高置信发现；需要证据绑定和人工确认状态。
- 健康 failover、任务恢复和归档恢复均不能自动重放结果未知的目标操作。

## 技术选型的最终理由

首选 Go 模块化控制面、PostgreSQL + pgx/sqlc、Temporal、React/TypeScript 静态 Web、S3 兼容证据存储、OIDC、隔离 Runner 和统一出口。Go 与 ARTEX 的并发/长连接/工具生态匹配；PostgreSQL 已能承担关系图和事务；Temporal 解决长任务恢复；React 保留成熟组件；对象存储承担大证据及保留。需要 Python 库的分析可以封装为 Runner 适配器，不必因此再造 Python 控制面。

LangGraph 适合以 Python 为主的模型流程实验，但不替代执行 lease、授权边界和证据事务；TypeScript/NestJS 适合已有 TS 后端团队，但本次没有证据证明其更能解决 ARTEX 的运行问题。Kafka、Redis、Neo4j、向量库、OpenSearch、KEDA、服务网格与多微服务均不是首版的默认依赖，应由确定负载或交付需求触发。

尚需技术验证的是 Runner 出口强制策略、工具的实际只读语义、图/时间线容量、模型方案质量和私有化运维成本。它们以隔离 PoC 与回归场景验证，不影响已确定的首版产品边界。完整架构与验证顺序见 [design.md](../design.md) 和 [implement.md](../implement.md)。
