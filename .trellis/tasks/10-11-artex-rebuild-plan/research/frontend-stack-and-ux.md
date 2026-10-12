# 前后端选型与工作台改进研究

本文件回应用户对“比 ARTEX 更强大、更好用”及 shadcn/ui 的补充要求。用户已确认单企业私有化、高自主 Agent、主动枚举与非破坏性验证上限；具体技术栈仍为建议。研究日期为 2026-10-12，ARTEX 结论来自静态源码审查，shadcn 能力来自官方文档；未运行 ARTEX、性能测试或目标扫描。需求以 [PRD](../prd.md) 为准，完整架构以 [design.md](../design.md) 为准。

## 1. ARTEX 前端的真实基础

| 事实                                                                 | 源码依据                                                                                                                                                                                                                                                                                                                                                                                                                                                    | 对新平台的含义                                                                                            |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| 已实际采用 shadcn/ui 与 Radix                                        | [components.json](E:/Crazywhite/ARTEX/web/components.json:2) 声明官方 schema 与 radix-nova；[globals.css](E:/Crazywhite/ARTEX/web/src/app/globals.css:3) 导入 shadcn/tailwind.css；[Button](E:/Crazywhite/ARTEX/web/src/components/ui/button.tsx:3)、[Dialog](E:/Crazywhite/ARTEX/web/src/components/ui/dialog.tsx:4) 使用 Radix；[发现列表](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/findings/_components/findings-table.tsx:38>) 使用本地 UI 表格 | 继续采用是延续既有基础；能力与易用性提升须来自业务流程、组件组合和状态语义。                              |
| React/TS/Next/Tailwind 已构成当前框架基础                            | [package.json](E:/Crazywhite/ARTEX/web/package.json:23)                                                                                                                                                                                                                                                                                                                                                                                                     | 不因追求不同而换生态；复用代码须另核对许可证和具体行为。                                                  |
| Next.js 已支持静态导出                                               | [next.config.mjs](E:/Crazywhite/ARTEX/web/next.config.mjs:24)                                                                                                                                                                                                                                                                                                                                                                                               | Vite 简化新建工作台的工具链，不能把无需生产 Node SSR 称为新增能力。大量迁移既有页面可继续 Next 静态模式。 |
| TanStack Table、React Hook Form、Zod、Graphin、XYFlow 是已有依赖声明 | [package.json](E:/Crazywhite/ARTEX/web/package.json:30)；本次定向搜索 web/src 未确认这些库的直接导入                                                                                                                                                                                                                                                                                                                                                        | 不将依赖存在当作全面应用证据。推荐统一落地 Table/Form/Schema，不声称三套图引擎同时在运行。                |
| G6 和 Zustand 有实际使用入口                                         | [coverage graph](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/_tabs/coverage-graph-tab.tsx:539>)、[preferences store](E:/Crazywhite/ARTEX/web/src/stores/preferences/preferences-store.ts:1)                                                                                                                                                                                                                                               | 图能力和客户端偏好值得保留；图选型根据交互/负载验证，服务端缓存另有职责。                                 |

## 2. shadcn/ui 的官方能力与取舍

- [官方介绍](https://ui.shadcn.com/docs) 将其定位为可组合、可修改的组件代码基础。基础控件进入自己的仓库，适合定制紧凑工作台、证据抽屉、执行时间线和权限状态。
- [安装文档](https://ui.shadcn.com/docs/installation) 支持 Vite、Next.js、React Router 等方案，采用 shadcn 不要求 Next.js 或 Vercel。
- [Data Table 指南](https://ui.shadcn.com/docs/components/base/data-table) 通过 TanStack Table 组合表格。它提供构建方法，查询、跨页选择、导出和错误处理仍须根据产品实现；不将指南视为完整数据网格。
- 当前官方页面可选择 Base UI、React Aria、Radix UI 等 primitives。推荐延续 ARTEX 的 Radix 基础，同类控件保持一致，具体组合和稳定版本在实施前验证；不把网页示例版本当成已完成兼容测试。

自持源码也意味着自持维护责任：统一组件目录、设计 token、严重程度/执行状态词汇、密度和焦点规则；记录生成来源与版本，审查上游变更，避免 CLI 覆盖业务修改。可访问性基础来自 primitives，复杂表格、图与弹窗组装后仍需真实键盘、焦点恢复及读屏验证。

shadcn 是首选，因为双图、证据和执行过程需要较多定制。Ant Design 适合优先交付大量传统后台表单/表格的团队；两者都不能自动解决任务状态、完整分页、权限和证据一致性。本方案不承诺仅换组件就提高性能。

## 3. 前端建议

| 层             | 推荐                                                    | 负责的具体问题                                                                                                                 |
| -------------- | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| 应用与路由     | React + TypeScript + Vite + React Router                | 登录后的交互工作台，静态资源与 Go API 同源交付；分享筛选写 URL。Next 静态模式作为页面复用替代。                                |
| 组件体系       | Tailwind + shadcn/ui + Lucide                           | 控件、主题、布局和一致语义；进一步组合 TaskSummary、ActionTimeline、FindingTable、EvidenceViewer、ScopeEditor、ApprovalPanel。 |
| 服务端数据     | TanStack Query + OpenAPI 生成 client                    | Query key、分页、取消、缓存失效与错误统一；SSE 事件按持久 cursor 回补，批量更新或刷新缓存。                                    |
| 表格           | TanStack Table + shadcn Table；实测需要时加 Virtual     | 服务端筛选/排序/分页，稳定 ID，准确批量范围；虚拟化仅减少 DOM，不替代服务端查询。                                              |
| 表单           | React Hook Form + Zod                                   | 授权范围、模板、预算、凭据配置；前端校验改善输入体验，服务端仍重验全部业务约束。                                               |
| 图             | React Flow 优先用于任务 DAG/谱系与适度子图              | 保持视口与选中节点、按需展开、来源跳转与列表替代；超大资产网络图压测后决定是否需要 G6。                                        |
| 图表与本地状态 | Recharts/shadcn Chart；局部 React state，必要时 Zustand | 覆盖/预算有确定指标定义；偏好与布局是客户端状态，不能复制 Query 服务端缓存。                                                   |
| 验证           | Vitest/Testing Library + Playwright + axe-core          | 验证真实状态和完整流程、断线/超时/冲突、键盘与焦点；axe 不替代人工可访问性验证。                                               |

不将密钥写入本地草稿；原始目标 HTML/HTTP 采用转义文本或隔离预览，不运行目标脚本或自动请求响应中的远端资源。工作台性能以实际数据规模和用户流程验证，不提前编造节点/记录数或速度承诺。

## 4. 后端建议与替代

| 层             | 推荐                                                 | 作用与边界                                                                                                      |
| -------------- | ---------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| 控制 API       | Go + chi + REST/OpenAPI                              | 延续 ARTEX Go 领域经验，统一项目/授权/任务/发现业务与 SSE；模块化单体先行。                                     |
| 业务存储       | PostgreSQL + pgx/sqlc + 编号 migration               | 权威业务对象、Action/Attempt、lease、审计关联、双图元数据和查询投影。                                           |
| 持久化编排     | Temporal + Go SDK                                    | 工作流进度、timer、signal、等待/取消/恢复；PG outbox、稳定 ID 和 reconciliation 协调两种持久化边界。            |
| Agent          | Go 模型适配层 + 结构化计划/Action + Planner/Observer | 模型提案经过确定性校验，再交隔离 Runner；不另建一套拥有执行真相的 Agent 调度器。Python 适配器按既有库价值引入。 |
| 执行           | 隔离 Kubernetes Job + 强制目标出口                   | 非 root、资源配额、短期 capability、默认拒绝网络；容器声明本身不等于隔离效果已经验证。                          |
| 证据           | S3 兼容对象存储 + 版本/保留锁 + KMS                  | 大对象、不可变证据、hash、manifest、授权访问；证据引用与业务关联在 PG。                                         |
| 身份与机密     | 企业 OIDC + Go session；企业 secret broker/Vault     | HttpOnly cookie、CSRF 与同源 SSE；为 Runner 发短期机密。SAML/SCIM 按客户需求后置。                              |
| 事件/观测/交付 | PG outbox + SSE；OTel + Prometheus/Grafana；Helm     | 一致活动和断线恢复，任务/模型/Runner 可观测；开发环境 Compose，生产交付需备份/恢复与升级矩阵。                  |

Go 为首选，但团队熟悉程度会影响成本。Python/FastAPI 适合 Python 为主、模型实验频繁的团队；TypeScript/NestJS 适合全栈 TS 团队。它们仍须解决同样的工作流、隔离、证据与授权合同，语言切换本身不带来这些能力。

Temporal 和 Kubernetes 的运维成本是明确取舍：用户已确认高自主执行，长任务、等待和故障恢复是本方案推导的执行需求，隔离环境先验证真实恢复、出口和交付成本。API 与 Workflow worker 来自同一模块化代码库并独立部署，Runner 独立隔离。Kafka、Redis、Neo4j、向量库、OpenSearch 与更多微服务由实际负载或检索需求驱动，首版不预先堆叠。

## 5. 如何验证“取长补短”

保留 ARTEX 的全局资产图/任务探索图、Planner/Worker 分工、共享 todo、trace 检索、模型链与降级、独立证据、复测和冻结报告。这些已经是有价值的产品积累；源码详细依据见 [总体审查](artex-review.md) 和三个专项审查。

| ARTEX 源码观察                                         | 新平台应达到的可观察结果                                                                       |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| Manager/Engine 与进程内运行协调耦合，已有任务/意图恢复 | Action/Attempt 级持久化恢复与 fencing；Worker 丢失后不盲目重复未知操作，界面展示在途和未知。   |
| 任务多入口 polling 分散、局部错误静默                  | 工作台解释进度、阻塞、未覆盖与下一步；统一缓存与断线回补，显示最后同步时间。                   |
| 独立 findings 与图读取存在不同上限/投影路径            | 同一 finding 在全局/任务/复测使用同 ID/版本，完整列表可查且截断明确。                          |
| 双图、事实锚点、证据与报告闭环已有基础                 | finding → Action/Attempt → Evidence 能独立复核；覆盖分母固定范围版本，不用节点数代表验证完成。 |
| 某些接口先写状态再验证其他字段                         | 请求全量验证后原子提交；冲突保留输入，命令幂等且受理状态可查询。                               |
| 图全量查询和多页面状态需要人工定位                     | 图增量更新保留视口，返回恢复筛选与工作位置；核心流程具备键盘路径与列表替代。                   |

R13/A11–A12 将工作台和对照记录纳入 V1。使用相同合成数据、角色和脚本场景，完成创建任务、判断阻塞、复核发现、复测/报告、暂停/恢复六类流程，记录任务完成率、查证步骤、耗时、误操作、响应和故障恢复。取得 ARTEX 可比较基线后确定提升门槛；本次研究不等同于这些实验已经完成。

实施顺序维持 P0 基础领域/证据闭环、P1 持久且隔离执行、P2 高自主 Agent 与工作台联合验收，P0–P2 全部完成才是 V1。P3 再增加企业增强与规模化能力，详见 [路线图](../implement.md)。
