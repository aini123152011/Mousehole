# ARTEX 领域、功能与 UI 审查

本审查针对 `E:/Crazywhite/ARTEX`，覆盖全部侧栏功能入口、任务详情的十个工作页，以及资产/范围、任务/探索图、发现/证据、复测、通知、归档、API 和实时交互的核心实现。它是源码静态审查；没有修改 ARTEX、启动服务、构建、运行测试或向目标发起请求。代码事实、静态推导和重建建议分别说明，不能将本报告当成性能或线上稳定性证明。

已确认产品条件是单企业私有化部署、高自主 Agent、主动枚举与非破坏性验证。V1 不包含自主写入目标、修改账户或配置、持续驻留、影响可用性或不可逆利用。下表中的 P0/P1/P2 是同一 V1 的建设阶段；完成 P2 才形成满足上述条件的首版，P3 为之后增强。

## 1. 核心判断

ARTEX 值得继承的不是页面数量，而是“资产和范围 → 任务与探索 → 工具活动 → 事实与发现 → 证据 → 复测 → 报告”的闭环，以及来源、历史和上下文的关联能力。它已经超出简单的 Agent 聊天壳：双图、独立发现、证据版本、复测快照、通知 outbox、归档状态机都有实际实现。

重建应保留上述业务能力，重新定义权威数据和执行边界。探索图负责解释计划、事实与推理；持久化 Action/Attempt 负责真实执行；Finding 负责结论和处置；授权书版本负责准许执行什么；EvidencePackage 负责证据内容和来源。不能让图节点、资产归属、聊天文本或实时连接代替这些契约。

首版需要形成可自主运行、可中断恢复、可复核结果的完整闭环。可以简化高级图形、外部同步和插件管理，但不能后置范围约束、执行隔离、证据完整性与故障恢复。

## 2. 全量功能入口及取长补短

侧栏有功能区十个入口、系统区十一个入口，定义见 [sidebar-items.ts](E:/Crazywhite/ARTEX/web/src/navigation/sidebar/sidebar-items.ts:66)。以下盘点覆盖全部侧栏入口，子功能另列。

| ARTEX 入口/能力 | 保留价值                                           | 重建取舍与阶段                                                                                          | 实现锚点                                                                                                                                |
| --------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| 仪表盘          | 汇总任务、发现、活动、工具与模型用量               | P0 基本运行概况；P2 增加待审批、证据齐备率、结果未知、预算和失败原因；不只显示 Token 或漏洞数量         | [聚合 API](E:/Crazywhite/ARTEX/server/server.go:779)                                                                                    |
| 对话            | 独立会话、模型/Agent 选择、消息详情、停止          | P0 保留任务内解释和只读问答；P2 将会话发起的执行显式转换成 Action；独立聊天不隐式拥有扫描权限           | [会话 API](E:/Crazywhite/ARTEX/server/server.go:824)                                                                                    |
| 任务            | 模板、类别、批量管理、队列、暂停/恢复/重跑、归档   | P0 保留已批准模板与基本组织；P1 持久化工作流、取消及恢复；高级批量操作/完整归档恢复 P3                  | [任务 API](E:/Crazywhite/ARTEX/server/server.go:673)                                                                                    |
| 发现            | 列表、分组、资产树、筛选排序、状态处置、导出、详情 | P0 以 Finding 为单一权威源、统一分页；P2 非破坏性复测；P3 高级分组和工单联动                            | [发现 API](E:/Crazywhite/ARTEX/server/server.go:755)、[发现页](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/findings/page.tsx:252>) |
| 流量            | 查询真实请求/响应并回链至漏洞证据                  | P0/P1 按 Action 最小采集、受控查看；V1 支持具体执行的 HTTP 证据，P3 再考虑全量代理分析工作台            | [流量 API](E:/Crazywhite/ARTEX/server/server.go:786)、[证据接口](E:/Crazywhite/ARTEX/server/finding_traffic.go:369)                     |
| 工具执行        | 查看调用参数、输出与统计，复核执行事实             | P1 保留为任务执行时间线及运维查询；补 ToolCapability、策略决定、Attempt、退出原因和版本                 | [工具执行 API](E:/Crazywhite/ARTEX/server/server.go:793)                                                                                |
| LLM 录制        | 查模型输入输出、用量、按模型统计                   | P2 保留模型观测、脱敏追踪和预算；与证据/审计分开保留，限制敏感 prompt 的访问                            | [LLM 记录 API](E:/Crazywhite/ARTEX/server/server.go:795)                                                                                |
| 资产            | 六类全局资产、公司归属、网络/HTTP/应用信息         | P0 保留共享资产目录和关联；分离当前 Asset 与带时间/来源的 AssetObservation，单企业改为业务单元/项目归属 | [资产 schema](E:/Crazywhite/ARTEX/db/schema.sql:40)、[资产 API](E:/Crazywhite/ARTEX/server/server.go:742)                               |
| 资产同步        | ScopeSentry 数据源、项目/任务选择、同步            | P0 手动/API 导入足够；P3 接企业资产平台，保留来源与冲突规则；新导入资产不自动获得执行授权               | [ScopeSentry API](E:/Crazywhite/ARTEX/server/server.go:871)                                                                             |
| 工作空间        | 文件浏览、读写、上传下载与目录管理                 | P0 任务附件与证据查看；P1 每次运行隔离的临时工作目录；P3 再做通用文件编辑器，不暴露控制面工作目录       | [workspace API](E:/Crazywhite/ARTEX/server/server.go:710)                                                                               |
| LLM 配置        | 多 profile、激活、重试、池状态、模型检测           | P2 保留角色级配置、供应商故障降级与用量预算；将密钥放入企业密钥系统，配置变更有版本                     | [profile/retry API](E:/Crazywhite/ARTEX/server/server.go:898)                                                                           |
| Agent 配置      | 配置、Prompt/版本、wrapup、触发器、变量预览        | P2 保留版本化角色配置和预览；执行角色只能选择批准的能力；自由编辑触发器/脚本化配置 P3                   | [Agent API](E:/Crazywhite/ARTEX/server/server.go:835)                                                                                   |
| MCP             | 服务 CRUD、工具枚举和刷新、可见性                  | P2 可接少量经批准适配器；任意可执行 MCP 私有注册表 P3；MCP 工具同样经过参数和网络策略                   | [MCP API](E:/Crazywhite/ARTEX/server/server.go:865)                                                                                     |
| Skill           | 上传、编辑文件、使用统计、缺失 Skill 与可见性      | P2 保留只读、版本化知识流程；执行脚本走签名适配器；在线编辑可执行 Skill 或市场 P3                       | [Skill API](E:/Crazywhite/ARTEX/server/server.go:877)                                                                                   |
| 工具管理        | 内置工具配置、按 Agent 绑定、自定义工具及测试      | P1 已批准工具能力目录；P2 角色绑定；自定义任意命令/代码 P3，不能让改工具描述就扩大权限                  | [工具 API](E:/Crazywhite/ARTEX/server/server.go:856)                                                                                    |
| 通知推送        | 多渠道、过滤、实时/汇总、投递历史与重试            | P0 站内事件；P3 邮件/企业 webhook；继承 outbox、lease、重试和积压年龄指标，外部发送使用脱敏内容         | [通知 API](E:/Crazywhite/ARTEX/server/server.go:804)、[通知 outbox](E:/Crazywhite/ARTEX/db/notification.go:275)                         |
| 拦截规则        | allow/deny/ask 与审批记录                          | P1 保留可解释的策略决定，改为授权书+能力目录默认拒绝；审批不改变授权范围或首版操作上限                  | [拦截 API](E:/Crazywhite/ARTEX/server/server.go:908)                                                                                    |
| 资产拦截        | 全局域名/IP/URL/CIDR 规则                          | P1 保留紧急阻断和 deny override；授权书白名单为正向边界；任务规则不得放开全局禁止项                     | [资产拦截 API](E:/Crazywhite/ARTEX/server/server.go:915)                                                                                |
| 审批记录        | 待决、决定、历史、关联执行                         | P1 保留 Action 精确预览、决定原因、失效时间；审批绑定参数/策略/授权版本，修改后失效                     | [审批 API](E:/Crazywhite/ARTEX/server/server.go:922)                                                                                    |
| 日志            | 运行排障                                           | P0/P1 运维查询和关联 Trace；与独立 append-only 审计分开，实施脱敏及保留策略                             | [审计入口](E:/Crazywhite/ARTEX/server/server.go:784)、[日志导航](E:/Crazywhite/ARTEX/web/src/navigation/sidebar/sidebar-items.ts:94)    |
| 系统配置        | 全局配置与连接测试                                 | P0 基础配置；P1 OIDC、存储、授权策略、Runner 池；P2 模型；配置变更审核和版本化，生产升级走签名制品      | [settings API](E:/Crazywhite/ARTEX/server/server.go:800)                                                                                |

任务详情的十个 Tab 定义见 [page.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/page.tsx:53>)。它们均有价值，但无需作为十个同权重入口。

| 任务内能力 | 保留/改造                                                                                      | 阶段                   |
| ---------- | ---------------------------------------------------------------------------------------------- | ---------------------- |
| 会话       | 保留主 Agent/Planner/Worker 的过程解释、历史、用户介入；默认页改为运行概况，会话作为上下文侧栏 | P0/P2                  |
| 总览       | 汇集目标、约束、资产、状态、成本；任务状态、执行状态和证据状态各自明确                         | P0/P1/P2               |
| 探索链路   | 保留计划→行动→事实→发现回链；与真实 Action/Attempt 对应，支持按需展开                          | P0/P2；复杂大图 P3     |
| 播报板     | 保留可读活动摘要，合并进执行时间线，摘要必须链接原活动/证据                                    | P1/P2                  |
| 发现       | 保留任务筛选和来源标签；直接查询权威 findings，不从节点 JSON 另造结果集                        | P0                     |
| 复测       | 保留独立 run 和三态结论；同 finding 单活跃复测，证据包/授权版本齐备                            | P0/P2                  |
| 测试资产   | 保留授权内目标列表、观测来源、查看关联行动和证据                                               | P0/P1                  |
| 资产覆盖图 | 保留未覆盖方向；V1 用分组列表和检查矩阵；全量力导图 P3                                         | P0/P2/P3               |
| 拦截审批   | 保留待决 Action、拒绝原因、停止入口；整合成任务状态侧栏和独立审批工作区                        | P1                     |
| 报告       | 保留 Markdown、复制/导出；版本化并绑定证据 manifest，展示生成时间与是否过期                    | P0/P2；复杂模板/PDF P3 |

其他已实现子能力也要归入产品取舍：目标/约束编辑、任务资产挂接、单意图与批量重跑、主会话 reset、任务继承、finding lineage、深入利用、finding-traffic 编辑、报告证据版本、archive/restore/delete 队列。它们的主要路由集中在 [任务控制](E:/Crazywhite/ARTEX/server/server.go:717)、[发现控制](E:/Crazywhite/ARTEX/server/server.go:762)、[会话 reset](E:/Crazywhite/ARTEX/server/server.go:775)、[归档 API](E:/Crazywhite/ARTEX/server/server.go:694)。

## 3. 资产、归属与范围：继承双图，拆开授权

**事实。** `assets` 是全局共享资产，包含 root_domain、ip、subdomain、app、service、endpoint 六类及来源任务等字段；公司 scope 支持域名、IP/CIDR、备案和关键词，并以 CHECK 确保字段一致性。[schema.sql](E:/Crazywhite/ARTEX/db/schema.sql:40)、[公司范围](E:/Crazywhite/ARTEX/db/schema.sql:115)。资产归属重算使用事务/锁，不能把它概括为毫无一致性处理。[assets.go](E:/Crazywhite/ARTEX/db/assets.go:101)。

任务创建可关联公司并建立当前资产上下文；探索 anchor 仅记录谱系。源码明确允许任意任务通过 search 阅读全局资产，anchor 不负责可见性授权。[tasks.go](E:/Crazywhite/ARTEX/db/tasks.go:91)、[exploration.go](E:/Crazywhite/ARTEX/db/exploration.go:245)。task_scope 又可以由显式插入资产自动生成或由 Agent/人工增加。[task_scope.go](E:/Crazywhite/ARTEX/db/task_scope.go:104)、[AddAgentScope](E:/Crazywhite/ARTEX/db/task_scope.go:142)。

**建议。** 保留“共享资产事实 + 每次评估的独立探索”。关系拆成 Asset、AssetObservation、Engagement、EngagementAsset、AuthorizationVersion。Asset 是当前目录；Observation 保留谁、何时、如何观察到什么；EngagementAsset 锁定本次评估引用的资产与观测快照；AuthorizationVersion 是已批准的执行许可。来源关系用关联表表达，不将 `task_ids[]` 当成权限或永久历史。

公司/项目归属与资产发现是上下文，不能自动扩大授权。Agent 可提出新增资产或范围申请，但实际连接前必须匹配已生效授权书、工具能力及当前策略。授权快照不能被后续 IP、标签或范围编辑覆盖；紧急撤销和更严格的当前 deny 策略应立即生效。

## 4. 探索、任务与执行生命周期

**事实。** 任务和 exploration 是 1:1；创建事务同时产生 origin fact。[tasks.go](E:/Crazywhite/ARTEX/db/tasks.go:12)、[创建](E:/Crazywhite/ARTEX/db/tasks.go:161)。图节点包含 begin/goal/intent/fact/finding/hint/digest，意图有 open/running/paused/done/blocked/exhausted/stopped/deleted 等状态；关系包括 spawns/derived_from/yields/proves/covers。[节点](E:/Crazywhite/ARTEX/db/schema.sql:170)、[边](E:/Crazywhite/ARTEX/db/schema.sql:248)。意图 claim/状态更新含 CAS；重跑与软删除保留谱系。[状态 CAS](E:/Crazywhite/ARTEX/db/exploration.go:360)、[重开](E:/Crazywhite/ARTEX/db/exploration.go:322)、[软删除](E:/Crazywhite/ARTEX/db/exploration.go:385)。

任务持久化了 status、paused、queued、queue_mode，但仍与 Manager 的进程内 mutex/map 协同；任务 admission 也有原子写入。[manager.go](E:/Crazywhite/ARTEX/server/manager.go:38)、[admission](E:/Crazywhite/ARTEX/server/manager.go:1141)。当前 terminal 判定是 done/failed/timeout。[tasks.go](E:/Crazywhite/ARTEX/db/tasks.go:72)。启动恢复会将 running intent 改回 open，这体现已有恢复意识，也意味着恢复不能自动等同于“可以无条件重放原网络操作”。[恢复](E:/Crazywhite/ARTEX/db/exploration.go:307)。

Activity 保存工具、错误、摘要、详情、metadata、token，详情按需加载，适合过程解释。[Activity schema](E:/Crazywhite/ARTEX/db/schema.sql:298)、[activity detail](E:/Crazywhite/ARTEX/db/exploration.go:86)。但 Activity 随 exploration 级联删除，不能作为永久独立审计源。[级联关系](E:/Crazywhite/ARTEX/db/schema.sql:300)。

**建议。** 保留节点类型、优先级、来源和重跑语义，执行真源变为 TaskRun、PlanRevision、Action、Attempt、PolicyDecision、Observation。Intent 是计划语义，Action 是一次结构化能力请求，Attempt 是已发生/结果未知的真实尝试。图是查询投影，数据库事务同时写权威对象、事件和 outbox；工作流引擎处理等待、恢复、取消、超时。Runner ownership 用 lease、heartbeat、fencing 管理；网络副作用不能凭幂等键声称 exactly-once。

“暂停调度”“停止当前执行”“取消任务”“只读对话”需有不同状态和明确确认。ARTEX paused task 仍允许 Main chat、reset 主会话只清 transcript 而不清图/资产/目标，这是有用的语义，应在 UI 明示。[主会话 reset](E:/Crazywhite/ARTEX/server/server.go:3638)、[paused chat](E:/Crazywhite/ARTEX/server/server.go:3669)。

## 5. 任务继承与覆盖率

**事实。** 继承只展开直接来源，不递归；每次重新查询关系，来源删除后会消失；继承节点/活动保留 SourceTaskID/Inherited。写入仍使用当前 exploration，源任务意图只展示 terminal 的记录；按 ID 读取源活动也限制到 terminal intents，避免任意全局活动 ID 暴露 source planner/main 对话。[exploration_sources.go](E:/Crazywhite/ARTEX/db/exploration_sources.go:16)、[terminal filter](E:/Crazywhite/ARTEX/db/exploration_sources.go:74)、[活动 ID 过滤](E:/Crazywhite/ARTEX/db/exploration_sources.go:347)。这些都是值得继承的明确来源与只读边界。

覆盖率源码称它是粗略参考：Tested 指至少 anchor 到 fact 的资产。[task_scope.go](E:/Crazywhite/ARTEX/db/task_scope.go:276)。带来源版本将当前/直接源任务 scope 匹配资产以及 anchor 到的资产合并作分母，用当前/直接源任务 fact anchor 作分子。[task_assets_context.go](E:/Crazywhite/ARTEX/db/task_assets_context.go:24)。

**推导。** fact 可能只表示资产枚举或信息观察；存在 fact anchor 不证明某类漏洞检查已执行/成功。因此当前指标不能被解读为严格的安全验证完成率。来源 live 数据或当前资产目录变化还会改变覆盖集合。这是代码语义推导，未用运行任务验证。

**建议。** V1 的覆盖分母锁定到授权版本+目标快照+检查模板。按 `(asset, check_type, template_version)` 记录未执行、已执行、失败、不适用、被策略拒绝及结果未知，并显示原因和 Attempt/证据。资产触达率可作为单独参考指标。P3 再引入跨任务知识推理和丰富 coverage 图；跨任务复用只导入有来源/时间/失效条件的事实，旧授权、凭据和审批不随之继承。

## 6. Findings：保留生命周期，统一权威源

**事实。** 独立 `findings` 可脱离 task/node 存在，删除任务通常保留 finding；状态包括 pending、in_progress、confirmed、resolved、fixed、false_positive、ignored、duplicate、risk_accepted。[findings.go](E:/Crazywhite/ARTEX/db/findings.go:15)、[状态](E:/Crazywhite/ARTEX/db/findings.go:39)、[schema](E:/Crazywhite/ARTEX/db/schema.sql:1081)。全局页从独立表读取；任务发现 Tab 从 graph kind=finding 读取。[server.go](E:/Crazywhite/ARTEX/server/server.go:1913)。

创建路径 `RecordFindingTx` 在同一事务中建立图节点、anchor/edge、独立 finding 和流量绑定，并验证任务/exploration/intent 对应关系。这是强项，不应误称所有 finding 写入都没有事务。[finding_traffic.go](E:/Crazywhite/ARTEX/db/finding_traffic.go:342)。

**静态缺陷。** 名称、分类、severity 修改先提交独立 finding，再 best-effort 更新 node JSON，失败被忽略，双读源可能不一致。[findings.go](E:/Crazywhite/ARTEX/db/findings.go:712)。任务发现接口对本任务及每个直接源任务仅调用 `ListByKind(...,200)`，没有完整分页或截断说明；没有 graph node 的独立 finding 不会出现在该任务 Tab。[server.go](E:/Crazywhite/ARTEX/server/server.go:1951)、[源任务](E:/Crazywhite/ARTEX/server/server.go:1976)。复测 Tab 却用独立 findings 的 task-filter 分页，进一步放大视图差异。[retests-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/retests-tab.tsx:34>)。

另有可复核的 PATCH 原子性问题：先校验并提交 status，之后才校验 severity。传入合法 status 和非法 severity 时，返回 400 但 status 已修改。[提交 status](E:/Crazywhite/ARTEX/server/server.go:2278)、[之后校验 severity](E:/Crazywhite/ARTEX/server/server.go:2299)。这是静态分支推导，未执行请求复现。

**建议。** Finding 作为唯一聚合根；图节点仅引用稳定 finding ID，展示内容从权威源查询或可重建投影。用 finding_assets、finding_evidence、finding_status_history 表达角色、外键和历史。PATCH 先完成全部校验，再在一个事务中更新数据/审计/outbox，并带 version/ETag 防止覆盖。保留处置状态，但将“模型置信度”“人工确认”“修复复测结论”独立表达，避免把 Agent 写入 graph confirmed 当成已核实的企业结论。

全局发现页的 flat/grouped/asset tree、筛选、排序、分页、Markdown/ZIP/CSV/JSON 导出有实际价值，首版保留基本筛选与 Markdown/JSON，不必复制所有布局。[布局](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/findings/page.tsx:70>)、[导出](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/findings/page.tsx:669>)。详情的 overview/lineage、Evidence、FindingTrafficPanel、FindingRetestPanel 可重组为“结论、证据、执行来源、处置历史”。[详情页](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/findings/detail/page.tsx:155>)。

## 7. 证据与报告：继承内容寻址和并发版本

**事实。** 流量证据独立快照保存请求/响应头、hash/len，通过 binding 关联 role、note、position，finding 持有 evidence_version/report_evidence_version。[schema.sql](E:/Crazywhite/ARTEX/db/schema.sql:1143)。快照 ID 基于规范化元数据 SHA-256，插入时验证；绑定幂等新增，有变化时递增版本。[finding_traffic.go](E:/Crazywhite/ARTEX/db/finding_traffic.go:130)、[新增绑定](E:/Crazywhite/ARTEX/db/finding_traffic.go:205)。

编辑绑定使用 finding 行锁和 evidence version CAS；重排核对完整 ID 集合，避免并发覆盖。归档 queueing 与证据写入也用 task lock 协调，已进入归档后拒绝继续加证据。[锁与 CAS](E:/Crazywhite/ARTEX/db/finding_traffic.go:141)、[编辑](E:/Crazywhite/ARTEX/db/finding_traffic.go:276)。报告生成记录所依据的证据版本；未传版本且已有证据时标记为 -1，使其不能伪装为最新证据报告。[报告版本](E:/Crazywhite/ARTEX/db/finding_traffic.go:456)。前端证据操作携带版本，失败后会重新拉取，不应一概说前端没有冲突处理。[finding-traffic-panel.tsx](E:/Crazywhite/ARTEX/web/src/components/finding-traffic-panel.tsx:75)。

工具鼓励读取真实流量正文并分段获取，报告携带 evidence_version；无 HTTP 流量时可引用文字/命令证据。[finding_traffic.go](E:/Crazywhite/ARTEX/server/finding_traffic.go:369)。但“文字字段非空”本身不足以证明某个观察已经发生。

**建议。** 延续 snapshot/hash/role/CAS，扩成通用不可变 EvidencePackage，支持 HTTP exchange、命令输出、截图、文件和人工附件。manifest 至少含 Action/Attempt、目标快照、授权/策略/工具版本、采集时间、长度/hash、脱敏规则、采集来源与对象引用；结论必须绑定实际证据对象。版本化报告绑定 finding/证据版本和 manifest，证据变动时显示过期，不静默用旧报告。保留内容寻址，但加密、访问控制、受审计保留/删除另行实现，hash 不能代替不可变存储或权限。

## 8. 复测与深入验证

**事实。** 发起复测对 finding 加锁，在事务中快照 finding/assets/constraints，创建 conversation/retest/首消息，并复用同 finding 已有 active run。[finding_retests.go](E:/Crazywhite/ARTEX/db/finding_retests.go:74)。结论为 reproduced/fixed/inconclusive，summary/evidence 必填；只有 completed+fixed 才在同事务更新 finding fixed。重启遗留 active run 置 stopped。[结果](E:/Crazywhite/ARTEX/db/finding_retests.go:181)、[自动 fixed](E:/Crazywhite/ARTEX/db/finding_retests.go:207)、[重启](E:/Crazywhite/ARTEX/db/finding_retests.go:256)。

复测工具从服务端 conversation 上下文取 finding，参数不能任意重定向结果；返回冻结 snapshot 以及当前 task constraints。这体现已有上下文隔离，历史快照与当前限制同时存在。[finding_retests.go](E:/Crazywhite/ARTEX/server/finding_retests.go:142)。删除 conversation 会停止 active retest，但记录保留；删除 finding 会级联删除 retest。[schema.sql](E:/Crazywhite/ARTEX/db/schema.sql:1112)。

“深入利用”将用户 description 变成优先级 10 Worker intent、恢复原任务，并原子记审计。[findings_groups.go](E:/Crazywhite/ARTEX/server/findings_groups.go:92)、[intent 与恢复](E:/Crazywhite/ARTEX/server/findings_groups.go:145)。这与独立 retest 不是同一功能。

**建议。** V1 保留独立 ValidationRun、单活跃 run、上下文快照和三态结论；要求实际证据引用，inconclusive 说明阻塞而不误判 fixed。冻结历史上下文与授权来源，执行时仍应用当前撤销/deny 规则；更宽范围须新授权版本，不能以复测绕过。首版不提供任意“深入利用”自动复活入口，改为选择批准的非破坏性二次验证模板。记录/证据在保留期内可复核，finding 删除与其证据/复测的保留关系需明确设计，不能无条件级联永久审计。

## 9. SSE：良好衔接设计与丢事件缺口

**事实。** Broadcaster 是 per-task 进程内 pub/sub，subscriber buffer 256，队列满直接 default 丢消息。[broadcast.go](E:/Crazywhite/ARTEX/server/broadcast.go:9)、[发布](E:/Crazywhite/ARTEX/server/broadcast.go:49)。SSE 先 subscribe 再 replay DB，支持 Last-Event-ID、标准 SSE id 和 500 条批量回补，避免 history/live 中间窗口，这是强项。[server.go](E:/Crazywhite/ARTEX/server/server.go:2979)、[replay](E:/Crazywhite/ARTEX/server/server.go:3019)。客户端有 task-wide SSE、快照 cursor、seq 合并去重、按会话分流、request token 防旧请求覆盖，且显示断线。[sessions-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/sessions-tab.tsx:634>)、[合并](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/sessions-tab.tsx:942>)。

**静态推导。** “掉事件后重连就能补齐”并非始终成立：若 live buffer 丢掉活动 A，随后继续送出更大 ID 的 B，handler 将 `since` 更新成 B 并写 SSE id；浏览器重连携带 B，数据库只查 `id>B`，A 便永久被跳过。关键路径是 [since 更新](E:/Crazywhite/ARTEX/server/server.go:3058)、[事件写入](E:/Crazywhite/ARTEX/server/server.go:3068)、[DB 游标查询](E:/Crazywhite/ARTEX/db/exploration.go:1479)。并非所有断线都造成丢失；问题成立的条件是 overflow 后连接继续收到后续事件。本次未运行压力测试。

**建议。** 实时广播仅充当“有新数据”的唤醒信号；实际交付总从持久化事件表按任务游标查询。或 overflow 后立即断开，禁止先发后续 ID，重连从最后可靠游标回补。多实例用共享 durable event store/outbox，统一定义 snapshot_cursor、event_id、对象版本及分页。不能用全局 ID 连续性自行推断缺口，因为别的任务也可能占用 ID；顺序真源必须是查询契约或任务内序号。前端重连刷新权威缓存，并明确展示连接状态、最后同步时间和未完成回补。

## 10. API 与 UI 的具体改进

| 已读实现                                                          | 问题或保留点                                                    | 重建建议                                                                                                                                                                                  |
| ----------------------------------------------------------------- | --------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 手写 `api.ts` DTO，返回通过 TS cast                               | 编译类型不验证服务端实际 payload；接口变化容易前后端漂移        | OpenAPI 作为契约，生成 TS client；入口关键数据用 Zod 校验，统一错误码/分页 envelope；[api.ts](E:/Crazywhite/ARTEX/web/src/lib/api.ts:1)                                                   |
| 全局 findings 无分页参数返回 bare array，有参数返回 page envelope | 同一个 endpoint 的响应形状和完整性取决 query；无参数最多 500 条 | 列表始终分页，统计用专门聚合接口；[server.go](E:/Crazywhite/ARTEX/server/server.go:1919)                                                                                                  |
| task detail 5 秒 polling，有 in-flight guard，暂时错误保留旧值    | guard 和保留旧视图值得继承，但需要让旧数据可见                  | 统一 query cache，显示 stale/last synced，并按可见性启停；[page.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/page.tsx:245>)                                         |
| overview 3 秒多 endpoint polling，局部 refresh 静默失败           | 请求分散，数据来自不同时刻；局部旧值容易被误读                  | 统一 query keys、对象版本与事件 invalidation，降低重叠请求；[overview-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/overview-tab.tsx:350>)                 |
| finding tab 3 秒 polling，无 in-flight guard，错误静默            | 慢请求可能重叠，连接失败缺少提示                                | TanStack Query 调度/去重、AbortSignal 与显式错误；[findings-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/findings-tab.tsx:149>)                           |
| graph 每 20 秒拉全图，JSON 签名避免重建                           | 签名减少无用布局，但大任务的全量查询/序列化成本会增长           | 初次分页/按需子图，之后增量更新；大图按节点类型/状态聚合；[graph-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/graph-tab.tsx:13>)                          |
| report 一次性拉取，错误置空                                       | 网络错误与“还没有报告”无法区分                                  | 明确生成中/失败/已有版本/过期四种状态，支持事件刷新；[report-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/report-tab.tsx:19>)                             |
| retest 顺序 setTimeout、不重叠、显式 error                        | 这比无 guard 的 polling 更可靠，可继承语义                      | 统一放入 query cache，而非在每个 Tab 自造循环；[retests-tab.tsx](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/retests-tab.tsx:34>)                                 |
| token 存 localStorage，SSE query token                            | 长期令牌进入持久存储和 URL，易进入日志/历史                     | 同源 BFF/控制面 session、HttpOnly/Secure cookie、CSRF 防护；SSE 无 URL token；[api.ts](E:/Crazywhite/ARTEX/web/src/lib/api.ts:109)、[SSE URL](E:/Crazywhite/ARTEX/web/src/lib/api.ts:189) |
| 默认进入会话，另有十 Tab 与 21 个一级入口                         | 对开发者透明，但分析员需多页回答“进度、依据、遗漏、待办”        | 六个工作区：项目/范围、任务、发现/复测、证据、审批、平台设置；[默认 Tab](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/page.tsx:212>)                                     |

这些观察不表示前端完全没有可靠性处理。Sessions 的 snapshot+cursor、缓存和 stale response 防护，TrafficPanel 的版本冲突重新拉取，Retests 的不重叠 polling 都应保留为新平台的验收场景；问题是同一产品里多个页面采用了不同的数据加载与错误语义。

## 11. 通知与归档：已有持久化模式，值得推广

**通知事实。** `notification_events` 保存 finding 快照，`notification_deliveries` 保存投递状态、attempts 和 next_attempt_at。[schema.sql](E:/Crazywhite/ARTEX/db/schema.sql:1341)。fan-out 在事务内用 FOR UPDATE SKIP LOCKED 领取并生成投递，支持多个进程不抢同一事件。[notification.go](E:/Crazywhite/ARTEX/db/notification.go:275)。投递 claim 事务内设 sending、递增 attempts、延长 lease，过期 sending 可重新领取；按渠道先计算限额再领取，避免等待消耗重试次数。[notification_delivery.go](E:/Crazywhite/ARTEX/db/notification_delivery.go:115)、[claim](E:/Crazywhite/ARTEX/db/notification_delivery.go:211)。

**准确局限。** 事件插入故意使用 savepoint：通知表插入失败时 rollback 到 savepoint，业务 finding/状态照常提交，返回 false 且只日志。因此“业务成功必有 durable notification event”不是无条件保证，即使调用点注释使用了原子措辞。[notification.go](E:/Crazywhite/ARTEX/db/notification.go:219)、[SetFindingStatus](E:/Crazywhite/ARTEX/db/notification.go:464)。此外 fan-out 使用执行 fan-out 时当前启用渠道/过滤，并非事件产生时的配置快照。[fan-out](E:/Crazywhite/ARTEX/db/notification.go:285)。

**归档事实。** QueueTaskArchive 行锁校验暂停/terminal、排队和直接依赖，再建立持久化 job；claim 使用数据库 FIFO 与 SKIP LOCKED。恢复策略区分 interrupted archive 手动重试、restore/delete 自动回队列，避免资源故障造成归档 crash loop。[queue](E:/Crazywhite/ARTEX/db/task_archives.go:220)、[recover](E:/Crazywhite/ARTEX/db/task_archives.go:358)、[claim](E:/Crazywhite/ARTEX/db/task_archives.go:371)。不能因此笼统称 ARTEX 所有队列都只在内存中。

**建议。** 继承 outbox、lease、重试、投递日志、积压年龄和分类恢复。新平台 durable domain event 与业务/审计同事务提交，通用事件不能因为某个通知渠道失败而丢失；渠道匹配、发送失败、限流在独立投递层恢复，外部服务仍按 at-least-once 和去重设计。核心任务执行加 heartbeat/fencing，而不是直接把通知 lease 实现当成完整执行引擎。V1 完成可校验的导出、备份和恢复演练；完整在线归档恢复 P3，恢复数据后不会自动恢复旧执行授权或启动 Runner。

## 12. 前端及领域技术栈建议

ARTEX 的前端框架为 React 19、TypeScript 5.9、Next.js 16 与 Tailwind 4，源码确认实际使用 shadcn/Radix、Zustand 和 G6。[package.json](E:/Crazywhite/ARTEX/web/package.json:23)、[components.json](E:/Crazywhite/ARTEX/web/components.json:2)、[Button](E:/Crazywhite/ARTEX/web/src/components/ui/button.tsx:3)、[preferences-store.ts](E:/Crazywhite/ARTEX/web/src/stores/preferences/preferences-store.ts:1)、[G6 入口](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/coverage-graph-tab.tsx:539>)。TanStack Table、React Hook Form、Zod、Graphin、XYFlow 已在依赖表声明，但定向搜索 `web/src` 未发现直接导入，不能据此认定已全面应用或三套图引擎同时运行。Next 支持 `NEXT_EXPORT=1` 静态导出，生产没有必须运行 Node SSR 的需求，开发模式反代 Go API。[next.config.mjs](E:/Crazywhite/ARTEX/web/next.config.mjs:3)。最新前端建议见 [选型研究](frontend-stack-and-ux.md)。

| 层           | 推荐                                                                                                                                            | 对 ARTEX 的取长补短                                                                                             |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| 管理后台     | React + TypeScript + Vite + React Router；若大量迁移现有页面可继续 Next static export                                                           | 保留生态和静态部署体验；SSR 有明确需求再引入 Node/BFF，登录 session 可由控制面处理                              |
| UI/表单      | Tailwind + shadcn/Radix + React Hook Form + Zod                                                                                                 | 延续易维护组件生态；复杂策略输入采用 schema 与预览，不只提供自由文本                                            |
| 服务端数据   | TanStack Query + 生成的 OpenAPI client                                                                                                          | 用统一 cache/分页/错误/取消请求替换散落 polling；Zustand 留给布局/筛选等客户端状态                              |
| 图与表格     | TanStack Table + React Flow；一种主要图引擎                                                                                                     | V1 以 DAG/谱系和按需子图为主；需要超大网络图且性能证实时再采用 G6，避免没有需求依据地维护多套图交互             |
| 领域存储     | PostgreSQL 关系表 + JSONB + 节点/边查询投影；迁移采用编号 migration                                                                             | 双图不要求 Neo4j；关键关系、版本和外键放结构化表，动态模型输出放 JSONB                                          |
| 权威业务契约 | TaskRun、AuthorizationVersion、PlanRevision、Action/Attempt、PolicyDecision、EvidencePackage、Finding、ValidationRun、ReportVersion、AuditEvent | 继承现有闭环，修复 finding 双读、动态 scope、coverage 指标和聊天/执行耦合                                       |
| 持久化执行   | 控制面工作流引擎 + 受控 Runner；最终架构首选 Temporal                                                                                           | 复用 ARTEX 的恢复/等待/暂停需求，替换进程内 ownership；长期状态不靠 SSE 或图节点，动作业务真相保留在 PostgreSQL |
| 证据存储     | 企业 S3 兼容对象存储 + 元数据/manifest 在 PostgreSQL                                                                                            | 继承 hash/staging/CAS；加入加密、权限、保留和不可变版本，正文不无限塞数据库/模型                                |
| 实时层       | SSE + durable event cursor + 事件触发 cache invalidation                                                                                        | 继承单向实时与快照衔接，修复 overflow 后跳过事件，多实例共享事件真源                                            |

不建议首版同时引入 Kafka、Neo4j、OpenSearch、多个图引擎和通用插件市场。优先用一个企业的实际任务规模验证 PostgreSQL 查询、事件回补和图加载；达到明确瓶颈再增加组件。V1 以已批准的检查模板、可信适配器和完整证据闭环建立产品价值，P3 扩展知识复用、同步、复杂报告与私有能力注册表。

## 13. 应转成新平台验收的具体场景

以下是针对发现的验收建议，不表示本次已经执行这些测试：

1. 同一个 finding 在全局、任务和复测页得到相同内容/状态，存在 201 条以上时仍完整分页；无图节点 finding 也能按 task 查询。
2. PATCH 同时传合法 status 与非法 severity，整个请求无修改；并发修改 version 不一致时返回冲突。
3. SSE 历史/live 切换、慢消费者 buffer overflow、断线重连和多实例切换后，不遗漏已持久化任务事件；Last-Event-ID 不越过尚未交付事件。
4. scope/资产目录变化不改写旧 run 的授权快照；继承源任务事实不能继承执行许可；紧急 deny 会阻止新连接。
5. 只有 fact anchor 的资产不被计为检查已完成；覆盖率能区分失败、被拒绝、不适用与结果未知。
6. finding 证据编辑有 CAS；旧报告被显示为过期；原证据内容可按 hash 校验，归档中不得追加未纳入快照的证据。
7. 同 finding 重复发起复测只得到一个 active run；inconclusive 不改变 fixed 状态；历史快照可复核，当前撤销策略生效。
8. 通知渠道失败不丢权威业务事件，expired delivery lease 可恢复，重复投递不会造成业务二次变化。
9. 页面暂时请求失败清楚显示 stale/error，不能显示成“无发现/无报告”；取消任务后 UI 只在实际执行终止后显示停止完成。

这些场景将 ARTEX 的已有积累和暴露出的具体缺口转成可验证契约，比仅列出一个全新框架清单更适合重建。
