# 实施路线图

本文件描述总体实现顺序。用户已于 2026-10-12 批准从 P0 工程骨架与草稿命令闭环开始，当前进度见 [子任务](../10-12-p0-command-foundation/implement.md) 和 [验收记录](../../../docs/validation/p0-command-foundation.md)。P0–P2 合在一起为高自主 V1；每一阶段在隔离环境使用模拟资产、合成证据和明确批准的内部目标验收。

## 编写前准备：统一中文规范与工具链

以下是 P0 的第一组工作，不另建产品阶段。首个子任务已落实工具链、实际前后端 spec 与草稿命令样本，并通过本地独立验收；其余领域及执行能力仍按后续阶段建设。按 [代码规范](coding-standards.md) 和 [模板](coding-templates.md) 执行：

1. 在新仓库建立中文领域词表、职责/依赖边界和实际前后端 spec，注释从中文模板起步；保留生成/第三方/固定工具语法例外，不把提案写成既有实践。
2. 锁定受支持 Go/Node、pnpm、生成器与检查工具；落地 `.editorconfig`、`.gitattributes`、gofmt/goimports、Prettier/ESLint、TS strict 和唯一 lockfile。验证 Windows 编辑与 Linux CI 的换行/格式一致，配置互不冲突。
3. 定义 OpenAPI/sqlc 的实际生成命令及漂移检查，约束生成目录、机器字段与中文描述；选定一种编号迁移工具，明确 SQL 格式与工具指令兼容性。
4. 建立一个合成数据的命令受理/查询纵向模块，检查 Go 包/函数中文说明、SQL 事务、中文 API/错误、React Hook/组件、命令状态、审计及行为测试；此时不提前接入目标执行能力。
5. 在 CI 中执行真实格式、lint、类型和变更相关测试；必需依赖缺失应失败，自动检查与人工中文语义/业务合同审查分别记录。
6. 抽查领域包、Hook/组件、SQL/migration、接口、关键测试及 PR 的代表样本，更新 README/ADR/审查清单，再推广到各模块。后续每阶段把实际约定及缺陷预防回写 spec。

验收：模板覆盖上述样本，手写说明中文且准确，代码满足统一命名/格式；命令和生成在新工程真实运行，CI 配置与实跑结果分别记录，审查能定位事务/幂等/取消/未知/证据边界。统一格式不能替代功能验收；草稿切片的实际命令和证据见 [README](../../../README.md) 与 [验收记录](../../../docs/validation/p0-command-foundation.md)，未实现能力不以规划检查代替行为验收。

## P0：基础设施与领域闭环

1. 建立 Go 模块化控制面与 React/TypeScript/Vite + React Router 静态 Web；首个草稿切片已采用 Tailwind/shadcn、TanStack Query 与 React Hook Form/Zod，后续列表再引入 TanStack Table，持续完善 token、状态/错误、密度和组合组件规范。REST/OpenAPI 生成前端 client，定义领域错误及 Action/Evidence schema；关键命令发送前生成 ID/幂等键，服务端同事务保存受理/业务/outbox，结果查询重验权限，绑定参数防同键改写。新工程当前采用 Vite，不复制 ARTEX 页面；后续能力延续本方案的职责边界。
2. 建立 PostgreSQL、pgx/sqlc 与编号 migration；模块为 Identity、Project、Authorization、Asset、Task、Evidence、Finding、Audit/Eventing。DDL 使用独立 migration Job，运行账号无 DDL 权限。
3. 接入 Go OIDC 授权码流程及 PKCE/state/nonce、HttpOnly/Secure/SameSite session cookie、CSRF、同源 SSE cookie。SAML/SCIM 后置；不使用 localStorage token、URL token 或不必要的 SSE ticket。
4. 实现授权书版本、资产 canonicalization、默认拒绝 PDP、当前紧急 deny/撤销，以及绑定真实 actor、Action hash/目标/适配器/授权/策略/TTL 的审批记录。全局资产与授权分离，支持手动/API 导入和显式历史上下文引用。
5. 实现全局资产图、任务探索图与 anchor 投影，任务模板/分类/历史和六个 Web 工作区。Finding 为权威对象，图节点由 outbox 更新；优先一种图引擎，按需子图、来源跳转、视口保持和列表替代视图。表格统一服务端筛选/分页、稳定 ID、URL 筛选、返回位置、批量范围与导出快照。
6. 接入 S3 兼容证据存储，建立 staging/hash、独立快照、绑定 Action/Attempt/Runner/授权/覆盖的 manifest、版本化/Object Lock、KMS、访问审计、保留/法律保全及期满处置。实现候选/verified/人工确认、独立复测去重与三态结论。ReportVersion 固定 finding/evidence/范围/模板版本，冻结 Markdown/HTML 与 manifest 导出，变更后旧报告标记过期。
7. 原子写业务对象、活动和 durable outbox；SSE 广播只唤醒，再按持久化 seq/cursor 查询，防提交次序/溢出跳过缺口；超过保留窗口重快照。站内通知投递失败保留领域事件。审计签名 checkpoint 写入独立权限域 WORM。
8. 建立基础 Helm 包、备份恢复路径、任务/策略/存储最小指标和临时依赖 CI，避免必需集成测试静默 skip。

验收：合成场景从资产和授权书到 Finding、复测、报告闭环；重复事件不重复变更，断线 SSE 无遗漏，证据内容可独立校验，授权版本不因资产变化而扩大。此阶段尚不声称高自主 V1 已交付。

## P1：持久化与隔离执行

1. 集成 Temporal + Go SDK，使用稳定 Workflow ID 幂等启动/signal；PG outbox 与 reconciliation 处理 DB 提交和 Workflow 启动的跨系统故障。不再实现第二套生产 scheduler。Workflow history 保存引用、小状态，Activity 执行外部 I/O，工具默认不自动重试。
2. 实现 Action/Attempt（awaiting_approval、in_flight/outcome_unknown）、lease、单调 fencing generation、心跳、超时、暂停/取消、迟到结果隔离和适配器 replay policy。暂停与取消均撤销新目标请求，记录 stop_requested 与出口/Runner 确认；配置并验证撤销传播上限。Task 状态是聚合投影，终止调度不抹除未知，单 Action 待审不冻结其他允许工作。
3. 定义 Runner registration、工作负载身份/mTLS、单次 jti 原子兑换 lease、短期 lease/fencing/目标/预算执行 capability 与机密、镜像 digest、结果 manifest 签名合约。签名不单独保证被攻陷 Runner 的陈述真实。
4. Runner 使用非 root Kubernetes Job、只读根文件系统、seccomp/capability 收缩、PID/CPU/内存/磁盘配额和临时工作区；禁止 privileged、hostNetwork 与宿主机挂载。浏览器 Runner 使用独立 profile/namespace。
5. 部署目标 egress gateway 和强制 CNI/NetworkPolicy：默认拒绝，只开放精确控制/证据基础设施通道及受控 DNS/目标出口。V1 完整覆盖所选 DNS/TCP/HTTP/TLS、IPv4/IPv6；UDP/ICMP/raw probe 未有专用适配器前关闭。解析、实际 IP、端口、重定向、attempt、速率均校验；允许已授权私网，另禁平台/metadata/loopback/link-local。TLS 应用语义由可信适配器负责，L4 gateway 不承诺可检查加密正文。
6. 建立 ToolCapability 和少量审核模板，固定 schema、参数、协议/端口、操作等级、请求/并发上限、凭据和可重放属性。以受限 DNS、TCP connect、服务/TLS/HTTP 元信息和经验证只读检查开始，不开放自由 Bash/任意 payload。
7. 将结构化结果、独立证据快照和工具/策略版本归入统一账本；实现 full/partial/unavailable 覆盖、独立验证器、执行时间线、审批队列、未知结果展示和真实停止确认。策略、关键审计或必需证据采集/持久化故障时停止新动作。

验收：模拟 Worker 丢失、重复投递、lease 到期、网络超时、取消、策略撤销与审批超时。仅一个有效 lease，旧 generation 新连接被拒，迟到结果不污染状态；不承诺已发请求可撤回或外部 exactly-once，未知 attempt 可见，重试符合目录。隔离测试必须证明目标出口旁路失败，不能只检查配置存在。

## P2：高自主 Agent 与首版验收

1. 实现统一模型网关：每角色主/fallback profile、健康与熔断、typed error、限流/费用、prompt/schema 版本、结构化输出。显式链耗尽停止，chain revision/CAS 拒绝旧请求覆盖，流式 committed 后不当次 failover。项目限制模型目的地和可发送数据级别，端点变化不沿用旧秘密；本地兼容端点与外部供应商均通过质量评估。
2. Planner 从只读任务/证据投影产生 PlanRevision/Intent/Action 提案，Observer 判断证据充分性与未覆盖原因。将 todo 持久化，Worker trace 按 task/intent/run 分页检索，模型输入使用有来源的摘要。
3. 将需人审且仍满足授权的 Action 转入待审批；未知能力、范围变化、首版禁止操作直接拒绝。side question 默认只读，要求执行时创建同样受控的新 Action。
4. 建立轮次、时间、Token/费用、请求数和并发预算、无新证据停机阈值；区分工具完成、验证完成、候选发现和确认发现。
5. 覆盖固定授权范围版本和检查维度，显示未测/完成/失败/拒绝/不适用/结果未知及原因。会话、todo、双图、时间线、trace、覆盖与费用聚合在任务详情。概况可下钻到阻塞/待审/缺口/未知，分开展示模型提案、策略决定和真实 Attempt；增量更新保留工作位置，断线/超时/冲突遵循统一数据与命令状态。
6. 在合成场景中完成多轮自主执行、模型故障降级、提示注入、预算耗尽、人工暂停/终止、复测和报告回归。每个结论记录 profile/model/prompt、输入摘要、Action 和 Evidence 关系。
7. 完成 V1 Helm/升级兼容矩阵、备份恢复演练和操作手册，实测计划负载下的工作流、Runner、图查询、证据及 SSE。使用同一批合成任务/角色对照 ARTEX，记录任务完成率、查证步骤、操作时间、误操作、列表/图响应和断线恢复表现；基于基线确定效率/容量门槛。仅在证明必要时增加额外组件。

验收：批准内部目标上的完整自主枚举/非破坏性验证链路通过 PRD A1–A12；恶意响应/越界建议不改变策略，切换模型和重启后 todo/trace 保留，不能用事实锚点虚报覆盖。分析员能完成状态判断与证据查证，关键操作具有键盘路径，断线与未知结果明确可见。完成 P0–P2 才可交付 V1。

## P3：企业增强与规模化

1. 客户需要时增加 SAML/SCIM、高级报告/复测模板、工单联动、外部通知的签名/脱敏/重试和完整归档恢复；恢复包不继承执行许可。
2. 建设私有扩展注册表：manifest、来源、schema、镜像 digest、依赖、网络目的地、等级和重放属性；所有扩展仍经过同一 Policy/Runner 契约。
3. 增加资产源同步、覆盖热图、跨任务语义推理；新增知识不自动成为授权范围。
4. 按 SLA/压测增加 PostgreSQL HA、隔离节点池、gVisor/Kata/microVM、KEDA、多 Runner 池与完整 Loki/Tempo。按外部消费/查询负载决定 NATS JetStream 或检索引擎。

验收：增强能力不绕过授权/审批，不改变 Action/证据契约，升级/恢复不重放未知结果操作。

## 验证门槛

- 有意义的单元/集成测试覆盖状态、默认拒绝、资产规范化、图投影、审批绑定、证据/复测、outbox、权限、模型降级和预算停止。
- 合约与故障注入覆盖控制面/Temporal/Runner/gateway：重复消息、DB 提交后 Workflow 尚未启动、过期 lease、迟到结果、未知 outcome、暂停/取消和继续运行。
- 网络验证覆盖 IPv4/IPv6、DNS rebinding、CNAME/重定向、平台保留地址、已授权私网、直接连接/代理旁路、TLS 下适配器语义。
- 身份/证据验证覆盖 cookie/CSRF、SSE 权限/溢出/重连、无 URL/localStorage token、对象授权/保留锁、签名 checkpoint、审批 actor/版本绑定与不能扩大授权、报告冻结/并发编辑/过期及必需采集故障。
- E2E 只使用模拟服务、录制响应和批准内部靶场；工具实际只读语义与模型质量需独立回归，不能用 schema 合规替代效果测试。
- 前端交互验证覆盖统一列表/图/证据联动、返回位置、服务端分页及准确批量范围、loading/empty/error/stale/reconnecting、命令受理/确认、受理响应丢失后按预先 ID 查询、同键去重/异参数拒绝、冲突输入保留与无敏感草稿。Vitest/Testing Library 验证关键组件状态，Playwright 验证完整流程，axe-core 辅助检查后仍需实际键盘/读屏验证。
- CI 运行 go test / race（适用包）、Go lint、前端 lint/typecheck/build 和真实临时依赖集成套件；按最终骨架填入准确命令。发布包含镜像 digest/签名、SBOM 与 migration 兼容校验。
- 按中文规范检查注释、命名、格式、生成漂移、接口/迁移说明、错误/日志、测试意图及 README/ADR/PR；自动检查格式/结构，人工验证中文含义与真实业务约束，不用字符扫描证明注释完整。
- 每阶段随机抽取 Finding，从界面/API 回溯授权、Action/attempt、策略、工具、对象 hash；实际演练数据库/对象/Workflow 恢复，缺失证据明确报告。

## 上线与回滚

政策包、能力目录、适配器、镜像、prompt/schema 独立版本化，在隔离池回归。工具风险出现时撤销版本/凭证、停止新调度并隔离 Runner，保留在途/未知结果和原始证据。Migration 采用 expand/contract，Workflow 采用兼容版本和 continue-as-new，证据永不就地改写；恢复与业务投影 reconciliation 不自动重放目标请求。

## 规划验证与实施门槛

方案阶段使用 `python ./.trellis/scripts/task.py validate 10-11-artex-rebuild-plan`，检查链接、要求/验收映射和跨文件一致性。用户的实施批准已获得，首个子任务已启动；无需再次索取同一范围的开始许可。ARTEX 源码继续只读，实际产品检查按根 README 和子任务验收执行，不用规划文件校验代替行为测试。
