# ARTEX 领域与 UI 实施上下文

本文件是完整 [领域与 UI 审查](artex-domain-ui-audit.md) 的精简注入入口，避免后续上下文加载截断该审查。完整报告覆盖 21 个侧栏入口、10 个任务详情 Tab；具体页面、路由及取舍表以完整报告为准。最终选型以 [design.md](../design.md) 为准；本次只做静态研究，没有修改或运行 ARTEX。

## 保留的产品闭环

资产/范围 → 任务/探索 → 工具活动 → 事实/发现 → 证据 → 独立复测 → 版本化报告。全局资产图与任务探索图通过 anchor 关联；任务保留模板、类别、来源、历史、共享 todo 和 Worker trace。导航重组为项目与范围、任务、发现与复测、证据、审批、平台设置六个工作区，不按底层表增加菜单。[侧栏](E:/Crazywhite/ARTEX/web/src/navigation/sidebar/sidebar-items.ts:66) [任务 Tab](<E:/Crazywhite/ARTEX/web/src/app/(main)/function/tasks/detail/page.tsx:53>)

## 必须继承或修正的合同

1. Asset 是共享知识，Observation 保留来源/时间；AuthorizationVersion 独立锁定执行范围。旧任务事实可引用，授权、凭据和审批不继承；当前紧急 deny 与撤销生效。原资产 anchor 是谱系，不负责可见性授权。[assets](E:/Crazywhite/ARTEX/db/schema.sql:40) [anchor](E:/Crazywhite/ARTEX/db/exploration.go:245)
2. Intent 是计划语义，Action/Attempt 是真实执行。ARTEX 有任务持久化、admission CAS 与启动恢复；重建补 lease、heartbeat、fencing 和 outcome_unknown，不能将恢复 running intent 当成可无条件重放目标操作。[admission](E:/Crazywhite/ARTEX/server/manager.go:1141) [恢复](E:/Crazywhite/ARTEX/db/exploration.go:307)
3. Finding 为权威源。ARTEX 创建 Finding 与绑定证据已使用事务；更新后 best-effort 同步图节点仍可能造成双读差异。新平台业务、活动与 outbox 原子提交，图为可重建投影；全局/任务/复测查询同一权威对象。[创建](E:/Crazywhite/ARTEX/db/finding_traffic.go:342) [更新](E:/Crazywhite/ARTEX/db/findings.go:712)
4. PATCH 全部参数校验后一次事务修改；请求不能在返回非法 severity 的 400 时已提交 status。图发现读取每任务/直接来源最多 200，需要完整分页或显式截断说明；无图节点 Finding 也应能按任务查询。[状态写入](E:/Crazywhite/ARTEX/server/server.go:2278) [severity 校验](E:/Crazywhite/ARTEX/server/server.go:2299) [图限制](E:/Crazywhite/ARTEX/server/server.go:1951)
5. 继承 hash/staging、独立证据副本、版本/CAS、冻结导出与过期提示。ReportVersion 固定 finding/evidence/授权/覆盖/模板版本，不混入并发修改；原证据与脱敏视图分别保存。增加 Action/Attempt/Runner/工具/策略来源与 full/partial/unavailable 采集覆盖。[证据存储](E:/Crazywhite/ARTEX/evidence/store.go:60) [版本](E:/Crazywhite/ARTEX/evidence/store.go:118) [冻结](E:/Crazywhite/ARTEX/evidence/store.go:265)
6. 复测独立工作流，固定上下文，同 Finding 单活跃去重，reproduced/fixed/inconclusive 三态；只有完成且明确 fixed 改变状态。失败不能等同修复。[复测入口](E:/Crazywhite/ARTEX/server/finding_retests.go:20)
7. Tested 是 fact anchor，不能视为检查完成；覆盖固定授权版本×目标快照×检查模板，分别记录未测、完成、失败、拒绝、不适用、未知。新资产与 auto scope 更新知识/统计，不自动扩大授权。[粗略覆盖](E:/Crazywhite/ARTEX/db/task_scope.go:276) [来源合并](E:/Crazywhite/ARTEX/db/task_assets_context.go:24)
8. 继承快照 cursor、Last-Event-ID、seq 去重与先订阅后回补。原 256 缓冲满时丢弃，后续 cursor 可能跳过缺口；广播仅唤醒，按持久提交顺序查询。溢出重查/断开，超过保留窗口显式重快照，多实例使用相同事件真源。[缓冲](E:/Crazywhite/ARTEX/server/broadcast.go:9) [丢弃](E:/Crazywhite/ARTEX/server/broadcast.go:49) [回补](E:/Crazywhite/ARTEX/server/server.go:3019)
9. 通知已有 outbox/SKIP LOCKED/投递 lease，归档已有 DB 队列；但通知 savepoint 失败仍允许业务成功，不能据此保证每次业务变化都有 durable event。核心 domain event 必须原子提交，渠道失败在独立投递层恢复。历史恢复不自动启动 Runner。[通知](E:/Crazywhite/ARTEX/db/notification.go:219) [投递 lease](E:/Crazywhite/ARTEX/db/notification_delivery.go:211) [归档](E:/Crazywhite/ARTEX/db/task_archives.go:220)
10. 会话默认只读问答，需要执行则产生同样受控 Action。时间线关联真实执行/证据；失败或暂时请求异常显示 error/stale，不显示成没有发现/报告。暂停取消展示停止请求与出口/Runner 确认，已发请求的在途/未知情况保留。

## 已收敛的技术与交付建议

推荐 React/TypeScript/Vite 静态 Web，Tailwind/shadcn/Radix、React Hook Form/Zod、TanStack Query/Table、生成 OpenAPI client、React Flow。ARTEX 实际使用 shadcn/Radix、Zustand、G6；TanStack Table、React Hook Form/Zod、Graphin/XYFlow 是已有依赖声明，定向搜索未确认直接使用。以上统一数据与图交互方案是重建建议，不是现状或用户已确认选型。ARTEX Next.js 也支持静态导出；若大量复用页面可保留，认证由 Go OIDC/session 完成。统一数据 cache/分页/错误/取消请求，布局等客户端状态才使用 Zustand，需分享的筛选写 URL。[依赖](E:/Crazywhite/ARTEX/web/package.json:23) [静态模式](E:/Crazywhite/ARTEX/web/next.config.mjs:3) [前端选型研究](frontend-stack-and-ux.md)

P0–P2 一起为 V1：基础领域/证据闭环 → Temporal 持久化与隔离执行 → 高自主 Planner/Observer、todo/trace、模型治理和覆盖。V1 含基础 Markdown/HTML 报告、复测、站内通知、备份恢复和可验证导出。P3 才加入复杂报告/PDF、外部通知、完整在线归档恢复、资产同步、跨任务语义推理、SAML/SCIM 和私有扩展注册表。首版操作上限始终为已授权主动枚举及非破坏性验证，审批不解锁禁止操作。

验收至少覆盖：201+ Findings 与无图节点查询；合法 status+非法 severity 全部无修改；SSE 溢出/断线/多实例不遗漏；范围快照固定与当前撤销；fact anchor 不冒充检查完成；证据 CAS/报告冻结与过期；复测去重及 inconclusive 不改修复状态；通知失效保留 domain event；UI error/stale/真实停止状态。新增统一工作台与 ARTEX 对照验收包含列表/图/证据联动、返回位置、准确批量范围、命令查询和键盘路径。完整验收清单在原审查第 13 节及 PRD A1–A12。
