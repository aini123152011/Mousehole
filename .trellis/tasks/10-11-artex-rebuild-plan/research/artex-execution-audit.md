# ARTEX 执行运行时、恢复、模型路由与证据专项梳理

审查日期：2026-10-12。对象为 `E:/Crazywhite/ARTEX` 当前源码，方法为只读静态审查。没有启动平台、运行目标扫描或执行测试；引用测试仅表示读过相应测试源码。本文区分可直接确认的实现事实、由调用链推导的风险及重建建议。

重建前提：单企业私有化部署；高自主 Agent；自主操作上限为主动枚举与非破坏性验证。修改目标状态、创建账户、驻留和影响可用性的操作不进入首版。工具审批不能突破这个产品上限，也不能扩大任务授权范围。

## 1. 核心判断：保留领域设计，重建可靠执行边界

ARTEX 不是只有聊天与工具调用的原型。它已有资产图、探索图、Intent→Fact 血缘、Worker trace 按需检索、Finding 与复测闭环、任务模型链、安全流式降级、任务重启恢复，以及独立、内容寻址的证据副本。这些设计应当继承，而不是在新平台里重新做成简单聊天历史。

执行运行时则主要围绕单进程 goroutine、内存事件/控制信号、SDK session 与主机工具组织。它支持恢复，但恢复到“任务/意图/对话继续运行”，还不是“每个外部动作具有持久租约、尝试身份、幂等和不确定结果处理”。高自主首版的主要投入应放在这一层，而不是单纯更换 Agent 框架。

| 能力     | ARTEX 已有实现                                                  | 重建取舍                                                                     |
| -------- | --------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| 探索规划 | Planner 根据图与事件产 Intent，Worker 执行一个 Intent 并写 Fact | 保留职责边界；计划必须先成为结构化 Action 才能执行                           |
| 任务恢复 | DB 恢复任务、意图、deadline；Worker transcript 有固定身份       | 保留领域恢复；补 Action attempt、lease、heartbeat、fencing 与 reconciliation |
| 模型链   | 任务/角色绑定、quota cursor、chain revision、提交前流式降级     | 保留契约；补 typed error、跨实例健康共享与动作级重复保护                     |
| 上下文   | 固定 mandate、todo、trace drilldown、历史来源                   | 保留按需检索；todo/continuation 持久化，旧观察带时效与验证状态               |
| 执行能力 | Bash、shell、custom、MCP、skill 等可组合                        | 收敛到统一 admission/Runner 入口；能力发现不能当作授权                       |
| 证据     | SHA-256 blob、写入校验、真实流量副本、报告版本冻结              | 继承；补 Action 身份、签名清单、审计锚、留存与加密                           |
| 活动记录 | 先持久 activity，再 SSE；失败重试后仍可丢弃                     | UI 通知可降级；授权、执行、证据事件须可靠记录                                |

## 2. Planner / Worker / Engine：设计合理，但语义仍偏进程内

### 2.1 Planner 的强项与局限

[planner.go:19](E:/Crazywhite/ARTEX/agent/planner.go:19) 明确 Planner 是事件驱动的意图生成者；[planner.go:382](E:/Crazywhite/ARTEX/agent/planner.go:382) 同时预取触发增量和 graph overview。这比每轮只看最近聊天更有利于跨 Worker 协作和长时间探索，应保留。

[planner.go:43](E:/Crazywhite/ARTEX/agent/planner.go:43) 的跨唤醒 todo 是对象内 mutex/map；[planner.go:432](E:/Crazywhite/ARTEX/agent/planner.go:432) 每轮创建新 session，并复用该内存 todo。[planner.go:441](E:/Crazywhite/ARTEX/agent/planner.go:441) 使用稳定 transcript ID，但并不 Resume 旧 Planner 对话。因此“有稳定 transcript”不等于“规划状态跨重启完整恢复”。模型配置变更也会重建 Agent bundle，见 [task_llm.go:546](E:/Crazywhite/ARTEX/server/task_llm.go:546) 与 [server_mgmt.go:449](E:/Crazywhite/ARTEX/server/server_mgmt.go:449)，内存 todo 可能随之丢失。

[planner.go:394](E:/Crazywhite/ARTEX/agent/planner.go:394) 将操作约束注入 prompt；[planner.go:373](E:/Crazywhite/ARTEX/agent/planner.go:373) Planner 仍可使用 SDK 默认工具；[planner.go:412](E:/Crazywhite/ARTEX/agent/planner.go:412) 使用 `ModeBypass`。这不代表项目完全没有权限或拦截系统，见第 5 节；但 Planner 的“生成意图”职责和其底层“可以执行主机工具”的能力边界并未完全分开。

[planner.go:457](E:/Crazywhite/ARTEX/agent/planner.go:457) 在没有 active intent 且目标未完成时要求继续至少产一个意图。从静态逻辑推导，证据不足或当前能力已经穷尽时，需要明确的 `inconclusive / capability_exhausted / budget_exhausted` 停机语义，否则容易反复产出低价值探索；本次没有运行验证该情况。

### 2.2 Worker 的强项与局限

[worker.go:23](E:/Crazywhite/ARTEX/agent/worker.go:23) 将 Worker 定义为“一次执行一个 Intent，写 Fact 后结束”；[worker.go:94](E:/Crazywhite/ARTEX/agent/worker.go:94) 的 session ID 固定为 `exp%d-worker-i%d`。[worker.go:384](E:/Crazywhite/ARTEX/agent/worker.go:384) 把 Intent mandate 与目标原始资产固定在 system prompt，防止 compaction 丢失。这些是应继承的上下文稳定性设计。

[worker.go:373](E:/Crazywhite/ARTEX/agent/worker.go:373) 的默认工具仍有 Read/Write/Edit/LS/Bash/Sleep；[worker.go:397](E:/Crazywhite/ARTEX/agent/worker.go:397) 的操作约束仍主要作为 prompt 指令。[worker.go:408](E:/Crazywhite/ARTEX/agent/worker.go:408) 把 Intent 资产加入 coverage scope。这里的覆盖范围是探索上下文，不能直接当作企业授权范围；发现或关联一个资产不应自动授权对它执行。

[worker.go:467](E:/Crazywhite/ARTEX/agent/worker.go:467) 每次 run 创建临时 TodoStore；[worker.go:497](E:/Crazywhite/ARTEX/agent/worker.go:497) 创建 session 后调用 Resume，但 [worker.go:513](E:/Crazywhite/ARTEX/agent/worker.go:513) 忽略其错误。因此对话恢复失败时缺少显式恢复分支。`requestID` marker 防止人工消息重复注入，见 [worker.go:507](E:/Crazywhite/ARTEX/agent/worker.go:507)，它不是外部动作幂等身份。

[worker.go:425](E:/Crazywhite/ARTEX/agent/worker.go:425) 的 settlement 隐藏名为 `Bash` 的工具。下文会说明，这个机制是模型可见 schema 的缩减，并不是统一的底层执行禁止；不能据此保证“任务收尾后绝不再产生外部动作”。

### 2.3 Engine 与 Worker pool

[engine.go:93](E:/Crazywhite/ARTEX/server/engine.go:93) 的运行句柄、控制信号与部分状态位于进程内 map；[engine.go:748](E:/Crazywhite/ARTEX/server/engine.go:748) 为每个任务创建 Planner loop 与 N 个 Worker loop。该结构简单、适合单实例，但多实例和独立 Runner 的所有权需要新契约。

[engine.go:800](E:/Crazywhite/ARTEX/server/engine.go:800) 的事件/heartbeat/debounce 与 pending trigger 主要在内存；[engine.go:781](E:/Crazywhite/ARTEX/server/engine.go:781) Planner heartbeat 最低 600 秒。实时更新不宜仅依赖进程内通知，重建需用 durable event/outbox 再派生唤醒与 SSE。

[engine.go:845](E:/Crazywhite/ARTEX/server/engine.go:845) 无 goal 任务根据 Go frontier 判断结束；[engine.go:880](E:/Crazywhite/ARTEX/server/engine.go:880) goal met 后进入终态并取消 Worker。新平台应保留确定性生命周期，并在 goal met 之前加入独立证据验证器，而不是只由 Planner 宣布完成。

[engine.go:1135](E:/Crazywhite/ARTEX/server/engine.go:1135) 已区分 done、blocked、exhausted 等 Intent 终态，优于统一“失败”。但是 `WriteCounts` 只进入日志，不参与验收；“Worker 正常返回”不能自动等同于“已获得可复核证据”。

[engine.go:1159](E:/Crazywhite/ARTEX/server/engine.go:1159) 对人工继续的 paused Intent 启动 pool 外专用 goroutine。从代码推导，这条路径可能超过配置的 Worker pool 并发；需统一资源 admission，而不是不同入口各自启动执行。

## 3. 恢复与调度：已有能力不能低估，仍需动作级恢复

### 3.1 已有恢复能力

[server.go:264](E:/Crazywhite/ARTEX/server/server.go:264) 在启动时恢复 paused/非终态任务，重建 deadline coordinator，并启动已 admission 任务 loops。不能将 ARTEX 说成“完全没有重启恢复”。

deadline 也并非全在内存：[engine_timeout.go:67](E:/Crazywhite/ARTEX/server/engine_timeout.go:67) 优先使用进程内缓存、回落 DB-loaded `DeadlineAt`；[engine_timeout.go:94](E:/Crazywhite/ARTEX/server/engine_timeout.go:94) 通过 `StampTaskFirstRun` 持久首跑时间和 deadline；[engine_timeout.go:131](E:/Crazywhite/ARTEX/server/engine_timeout.go:131) 协调器同时用于首次运行与重启 reload。[server.go:278](E:/Crazywhite/ARTEX/server/server.go:278) 重启时恢复截止协调，已过期任务可收尾。这些都应保留。

[db/exploration.go:322](E:/Crazywhite/ARTEX/db/exploration.go:322) 可重开 blocked/exhausted/stopped 意图并保留图与 transcript；[manager.go:1067](E:/Crazywhite/ARTEX/server/manager.go:1067) 修复模型链后可重开 quota-blocked Intent，终态任务不自动重开。这个终态控制是有意的产品语义。

### 3.2 缺少动作租约与不确定结果

[db/exploration.go:1172](E:/Crazywhite/ARTEX/db/exploration.go:1172) 的 open→running 认领使用 CAS 并保存 owner，这是有价值的原子性。但没有执行 lease、attempt ID、heartbeat 或 fencing token。[db/exploration.go:312](E:/Crazywhite/ARTEX/db/exploration.go:312) 启动时把所有 running 重置成 open，不检查实例所有权。在单实例崩溃恢复时合理；若直接扩成多实例，可能重新认领仍由其他实例执行的意图。该风险为调用链推导，未做多实例复现。

[engine.go:1237](E:/Crazywhite/ARTEX/server/engine.go:1237) 读取 Frontier 并 ClaimIntent 的查询/claim 错误被忽略；[engine.go:1031](E:/Crazywhite/ARTEX/server/engine.go:1031) 对 model_error 额外重跑整个 Worker。即使 transcript 可 Resume，也不能证明上一次已发出的扫描/请求不会再次运行。LLM 请求重试、Intent 重跑和外部动作重试必须分开处理。

[scheduler.go:144](E:/Crazywhite/ARTEX/server/scheduler.go:144) 先 `TouchTriggerFire`，再 `StartTriggeredRun`。从顺序推导，两步间崩溃可能使一次触发已登记但实际 run 未创建，建议事务 outbox/触发唯一键。初始化 watermark 有意跳过旧事件，见 [scheduler.go:59](E:/Crazywhite/ARTEX/server/scheduler.go:59)，不应误判为所有历史事件都应该重放。

[engine.go:640](E:/Crazywhite/ARTEX/server/engine.go:640) activity 持久化重试仍失败可 drop 并继续；[engine.go:671](E:/Crazywhite/ARTEX/server/engine.go:671) 成功持久化后才 SSE Publish。此设计适合部分 UI 动态，但关键授权/执行/证据事件不能 best-effort。UI 是否实时刷新与事件是否可靠落库是两个不同保证。

### 3.3 重建契约

任务、Intent、Action 和 ActionAttempt 分开建模。Intent 表示“要证明什么”，Action 表示一个已规范化、可审核的工具操作，Attempt 表示某个 Runner 的一次尝试。Action 至少保存输入摘要、工具版本、授权/策略版本、预算、证据要求与幂等键；Attempt 保存 lease、Runner 身份、fencing、时间、退出码、回执与证据覆盖状态。

每个动作经 `planned → admitted → queued → leased → running → completed/failed/cancelled/unknown`。若需要复核，插入 durable `awaiting_approval`，审批绑定确切输入与版本。控制面/Runner 重启后先查询回执、进程与 evidence，再 reconcile；不能把不知道结果的动作直接当失败重跑。

不能承诺任意外部工具的严格 exactly-once。平台可以保证认领/记录幂等、禁止过期 Runner 回写、限制重复下发；真正的外部副作用还依赖工具幂等或目标回执。无法判定时保留 `unknown` 并复核。即便首版仅非破坏性动作，重复扫描仍会消耗预算、增加目标负载和污染验证时间窗口。

## 4. 模型路由与上下文：值得直接继承的细节

### 4.1 模型链不是简单 fallback 数组

[task_llm.go:104](E:/Crazywhite/ARTEX/server/task_llm.go:104) 按角色绑定→任务链→全局解析；[engine.go:713](E:/Crazywhite/ARTEX/server/engine.go:713) 权威 task provider resolver 出错不偷偷回落全局。[task_llm.go:129](E:/Crazywhite/ARTEX/server/task_llm.go:129) 显式任务链耗尽后 hard stop。它避免了成本/能力约定被静默突破，应保留。

[task_llm.go:199](E:/Crazywhite/ARTEX/server/task_llm.go:199) quota 推进持久 task cursor；[task_context.go:464](E:/Crazywhite/ARTEX/db/task_context.go:464) 使用 `FOR UPDATE` 和 chain revision，避免旧 in-flight 请求错误改变新模型链。[task_llm.go:81](E:/Crazywhite/ARTEX/server/task_llm.go:81) 严格识别 quota，普通 429、认证错误、5xx 不一概推进 quota 链。这些细节应该成为新平台验收条件。

[task_llm.go:295](E:/Crazywhite/ARTEX/server/task_llm.go:295) 在未 committed 前缓冲事件；[task_llm.go:376](E:/Crazywhite/ARTEX/server/task_llm.go:376) 与 [task_llm.go:386](E:/Crazywhite/ARTEX/server/task_llm.go:386) 在已交付文本/思考/工具输入后不在当前调用直接换模型重放。[llmpool/pool.go:73](E:/Crazywhite/ARTEX/llmpool/pool.go:73) 也明确事件已交 caller 则不 failover。它防止半段输出被第二模型接续/重复交付，但并不能代替第 3 节的 Action 幂等。

[task_llm.go:237](E:/Crazywhite/ARTEX/server/task_llm.go:237) 的非流式 Complete 可以重试未交付结果；[task_llm.go:456](E:/Crazywhite/ARTEX/server/task_llm.go:456) compaction window 使用任务链最小上下文窗口。已阅读 [task_llm_test.go:375](E:/Crazywhite/ARTEX/server/task_llm_test.go:375) 附近的 midstream quota 测试源码，未运行测试。

[llmpool/pool.go:163](E:/Crazywhite/ARTEX/llmpool/pool.go:163) 有熔断、窗口过滤与同优先级 round robin；[health.go:119](E:/Crazywhite/ARTEX/llmpool/health.go:119) 软错误多次 trip、硬错误立即 trip，并采用退避 ladder。所有候选被过滤后仍尝试 head，见 [pool.go:180](E:/Crazywhite/ARTEX/llmpool/pool.go:180)，重建应明确这是恢复探针还是能力违反。

局限是错误分类仍依赖错误字符串/HTTP status 正则，见 [task_llm.go:431](E:/Crazywhite/ARTEX/server/task_llm.go:431) 与 [pool.go:210](E:/Crazywhite/ARTEX/llmpool/pool.go:210)；健康状态以内存为权威、PG 异步 best-effort，见 [server/llmpool.go:19](E:/Crazywhite/ARTEX/server/llmpool.go:19)。建议 provider adapter 返回 typed error（quota/rate_limit/auth/transient/context/cancelled），以独立模型网关管理调用预算和健康；多实例时明确共享状态一致性。

### 4.2 Todo、trace 与历史来源

[norma/tool/todo.go:28](E:/Crazywhite/ARTEX/norma/tool/todo.go:28) 的 TodoStore 是内存 slice/mutex，TodoWrite 全量替换。todo 不宜成为唯一 checkpoint；重建需持久计划版本、完成条件、当前已知事实和未决问题，聊天 todo 仅作为呈现。

[agent/tools.go:1800](E:/Crazywhite/ARTEX/agent/tools.go:1800) trace 摘要限制长度；[tools.go:1818](E:/Crazywhite/ARTEX/agent/tools.go:1818) 支持摘要列表、关键字搜索与最多五条 detail drilldown，并排除 thinking；[tools.go:1927](E:/Crazywhite/ARTEX/agent/tools.go:1927) 可查其他 Worker 尚未写 Fact 的观察。[tools.go:1973](E:/Crazywhite/ARTEX/agent/tools.go:1973) 的轻量索引降低全历史灌入成本。该思路优于默认把所有日志塞入 prompt，应继承。

[db/exploration.go:1808](E:/Crazywhite/ARTEX/db/exploration.go:1808) 当前使用 ILIKE summary/detail 搜索，不是语义搜索。本次未测大规模性能，不能说已发生性能问题；重建可以先用 PostgreSQL 全文/trigram 索引，只有检索质量与规模证明需要时才加入向量检索。

[exploration_sources.go:208](E:/Crazywhite/ARTEX/db/exploration_sources.go:208) 与 [exploration_sources.go:317](E:/Crazywhite/ARTEX/db/exploration_sources.go:317) 只继承直接 source，一层、不递归；[db/exploration.go:1788](E:/Crazywhite/ARTEX/db/exploration.go:1788) 查询来源 Intent 要求 terminal；[exploration_sources.go:270](E:/Crazywhite/ARTEX/db/exploration_sources.go:270) 不暴露 source Planner/Main 行，并标注 source_task_id/inherited。应保留来源标注，再增加观察时间、环境指纹、有效期、confidence 与 revalidation 状态；历史观察不自动成为本轮事实。

## 5. 工具、MCP 与 skill：编排便利不等于强制边界

### 5.1 现有权限系统与拦截应准确评价

[norma/permission/permission.go:86](E:/Crazywhite/ARTEX/norma/permission/permission.go:86) 显式 deny/工具 hard deny 优先，随后 ModeBypass allow，所以不能说“完全没有权限”。工具顶层调用经 permission Evaluate，见 [norma/harness/tools.go:57](E:/Crazywhite/ARTEX/norma/harness/tools.go:57)。ARTEX 还有 intercept/Guard 的规则、LLM reviewer 与人工审批；其默认范围、故障策略和范围闸门详见同目录 `artex-security-audit.md`。

但权限覆盖依赖实际调用路径。[permission.go:138](E:/Crazywhite/ARTEX/norma/permission/permission.go:138) `Name(...)` 按工具名匹配而非参数；[norma/harness/query.go:594](E:/Crazywhite/ARTEX/norma/harness/query.go:594) settlement 的 DisabledTools 只过滤给模型的 schemas；[norma/harness/tools.go:28](E:/Crazywhite/ARTEX/norma/harness/tools.go:28) 实际 dispatch 从 Registry.Get(name) 取工具，未检查 settlement disabled set。

[norma/tool/deferred.go:217](E:/Crazywhite/ARTEX/norma/tool/deferred.go:217) 的 ExecuteExtraTool 检查 unlock/schema 后直接调用内层 `t.Call(...)`，未对真实内层工具重新走 CheckPermissions/Evaluate/PreToolUse。[norma/tool/tool.go:161](E:/Crazywhite/ARTEX/norma/tool/tool.go:161) builtTool.Call 直接 Run，本身不执行上述权限流程。

[server/customtool.go:289](E:/Crazywhite/ARTEX/server/customtool.go:289) custom tool Permissions 默认 Allowed；[customtool.go:331](E:/Crazywhite/ARTEX/server/customtool.go:331) command custom tool 内部调用 `NewBash().Call`。从调用链确认，这不会执行 Bash 的 CheckPermissions，而少量灾难性命令 hard deny 正则位于 [bash.go:61](E:/Crazywhite/ARTEX/norma/tool/bash.go:61) 的 bashPermissions。即使注释称继承 Bash safety floor，也不能把注释当作实际权限执行。这是静态调用链差异，未进行恶意输入或绕过复现。

安全审查同时确认 Guard 对未启用拦截工具直接允许，默认启用列表并不覆盖 ExecuteExtraTool/MCP/custom 等所有入口。结论应是“已有治理机制，但未统一覆盖所有真实执行能力”，而不是“没有任何审查”。重建应把策略放在不可绕过的 Action admission 与 Runner capability 入口，覆盖内层真实动作。

### 5.2 Catalog 与组装

[agent/toolcatalog.go:10](E:/Crazywhite/ARTEX/agent/toolcatalog.go:10) 由代码拥有 handler，DB 可以覆盖描述/schema/default；[toolcatalog.go:30](E:/Crazywhite/ARTEX/agent/toolcatalog.go:30) SDK 默认工具不入 DB catalog、不受绑定过滤；[toolcatalog.go:102](E:/Crazywhite/ARTEX/agent/toolcatalog.go:102) 没有 DB row 的 MCP/skill/host 工具原样通过。[server/assembly.go:239](E:/Crazywhite/ARTEX/server/assembly.go:239) tools-table 读取失败按默认全部放行；[assembly.go:308](E:/Crazywhite/ARTEX/server/assembly.go:308) interactive shell 由 Agent 开关控制，不受该 binding。

重建需要版本化 CapabilitySpec：真实实现身份、JSON Schema、允许目标类型、副作用分类、资源预算、证据要求、幂等/取消语义、镜像 digest、最小 secret 权限。description/schema 编辑可以改变提示与展示，但不能凭 DB 编辑为实现授予新能力；未知/加载失败的能力不自动执行。

[agent/assembly.go:64](E:/Crazywhite/ARTEX/agent/assembly.go:64) 最后统一 guardPanic，保护服务进程，应继续保留。但捕获 panic 与隔离恶意/失控工具是不同问题。

### 5.3 MCP / skills 已有较完整的能力发现

[server/assembly.go:33](E:/Crazywhite/ARTEX/server/assembly.go:33) skills 按角色可见；[assembly.go:54](E:/Crazywhite/ARTEX/server/assembly.go:54) skill 的 mcps 控制 unlock；[assembly.go:111](E:/Crazywhite/ARTEX/server/assembly.go:111) invoke 后解锁相应工具，恢复时可从历史重建 unlock。[norma/skill/skill.go:135](E:/Crazywhite/ARTEX/norma/skill/skill.go:135) guidance 是独立消息，compaction 后可重注入，且有 listing 预算。这些便利应该保留。

但 [assembly.go:86](E:/Crazywhite/ARTEX/server/assembly.go:86) 表明 skill 被 invoke 前已连接/启动 MCP server；deferred 主要是 schema/call gate，不是进程懒启动隔离。unlock 表示模型发现能力，不等于企业授权。

[server/mcpdiscover.go:24](E:/Crazywhite/ARTEX/server/mcpdiscover.go:24) 支持 stdio、Streamable HTTP、legacy SSE 三种传输；[mcpdiscover.go:47](E:/Crazywhite/ARTEX/server/mcpdiscover.go:47) 有 UI tool cache、启动串行发现与超时。不能仅根据 norma 包注释误说“只支持 stdio”。[norma/mcp/mcp.go:192](E:/Crazywhite/ARTEX/norma/mcp/mcp.go:192) 与 [mcp.go:228](E:/Crazywhite/ARTEX/norma/mcp/mcp.go:228) 处理 tools/list 和 tools/call；[mcp.go:233](E:/Crazywhite/ARTEX/norma/mcp/mcp.go:233) 仅拼接 text blocks，其他 rich content 未处理。本次未见完整 resources/prompts/subscriptions 运行链，重建应按真实需要支持，避免宣称已完整实现全部协议。

### 5.4 主机进程不是 sandbox

[norma/tool/task.go:140](E:/Crazywhite/ARTEX/norma/tool/task.go:140) 默认继承控制进程环境，再叠加 session env；[task.go:231](E:/Crazywhite/ARTEX/norma/tool/task.go:231) host shell 仅设置工作目录、env 与独立 process group。[task_unix.go:13](E:/Crazywhite/ARTEX/norma/tool/task_unix.go:13) 和 [task_windows.go:16](E:/Crazywhite/ARTEX/norma/tool/task_windows.go:16) 有进程组/taskkill 清理；[task.go:515](E:/Crazywhite/ARTEX/norma/tool/task.go:515) Session Cleanup 终止 tasks/shell 并清临时目录。这是已有清理能力，但不是网络/文件系统/凭据隔离，也不是跨崩溃的 durable process 管理。

[norma/tool/shell_session.go:302](E:/Crazywhite/ARTEX/norma/tool/shell_session.go:302) PTY 仍是 host shell；[norma/mcp/stdio.go:32](E:/Crazywhite/ARTEX/norma/mcp/stdio.go:32) 启动本地主机 MCP 子进程；[server/customtool.go:379](E:/Crazywhite/ARTEX/server/customtool.go:379) 的 Python 临时脚本继承 os.Environ/sessionEnv。[bash.go:140](E:/Crazywhite/ARTEX/norma/tool/bash.go:140) 前台工具超时还可能自动后台化，不能把超时当作已经停止。

因此首版应该把这些执行全部移到独立 Runner：独立身份、专用 workspace、资源限额、默认拒绝出网、短时目标/能力令牌、最小秘密注入与进程树回收。代理 env、cwd、context timeout 和 Docker 包装分别有用途，但不能代替这些强制边界。

## 6. 证据与复测：ARTEX 已有扎实实现，建议补身份与审计

### 6.1 已有证据能力

[evidence/store.go:33](E:/Crazywhite/ARTEX/evidence/store.go:33) 使用 SHA-256 blob path 并验证长度/hash；[store.go:62](E:/Crazywhite/ARTEX/evidence/store.go:62) staging 写入、校验、Sync、rename 和目录 Sync，SQL 失败残留文件由 GC 清理。这不是“只有一段文本证据链接”。

[store.go:146](E:/Crazywhite/ARTEX/evidence/store.go:146) 从真实 Traffic exchange 复制 request/response，snapshot 包含 URL、method、time、headers、status、body hash/length。[traffic/evidence.go:29](E:/Crazywhite/ARTEX/traffic/evidence.go:29) 复制时持有 writer lock，避免删除竞争。[agent/finding_recorder.go:9](E:/Crazywhite/ARTEX/agent/finding_recorder.go:9) 明确 Agent 不合成/复制证据正文，由注入 recorder 原子处理，traffic_refs 必须指向真实 exchange；HTTP evidence 可选，非 HTTP 可使用命令/日志证据。

[store.go:182](E:/Crazywhite/ARTEX/evidence/store.go:182) 锁 task evidence 后 prepare 并 RecordFindingTx；[store.go:197](E:/Crazywhite/ARTEX/evidence/store.go:197) Finding Bind 在事务内；[store.go:247](E:/Crazywhite/ARTEX/evidence/store.go:247) 读取再次验证 hash/length。这些原子性和真实性约束应保留。

[store.go:118](E:/Crazywhite/ARTEX/evidence/store.go:118) 导出前冻结 bindings/report version 并私有复制 bodies；[store.go:265](E:/Crazywhite/ARTEX/evidence/store.go:265) 报告/portable archive 使用真实副本，不依赖原 Traffic 持续存在；[store.go:316](E:/Crazywhite/ARTEX/evidence/store.go:316) 恢复验证 snapshot metadata ID hash 与 body hash。[store.go:347](E:/Crazywhite/ARTEX/evidence/store.go:347) 对未引用数据24小时 GC，因此它是内容完整性与快照存储，不是永久 WORM 或不可删除审计账本。

[agent/retester.go:4](E:/Crazywhite/ARTEX/agent/retester.go:4) 独立复测会话区分 reproduced/fixed/inconclusive；fixed 要求可比环境与正常对照，一次失败不能当作修复。这一验收规则尤其值得保留。

### 6.2 应补强而非重复建设

重建在现有证据契约上补 `action_id/attempt_id/runner_id`、工具/模型/策略/授权版本、采集时间与证据覆盖状态。内容哈希只能证明保存后数据未变，不能独立证明目标来源、执行授权或采集完整。对要求证据的验证动作，捕获不可用时暂停，不能生成 verified finding；该点与安全审查中的代理/透传事实配套。

对象存储可以使用企业 S3/MinIO 兼容服务，PG 保存 metadata、关联与冻结版本；轻量首版也可保留本地 blob store 并定义迁移接口。增加签名 manifest、独立审计锚/受控留存、加密、敏感字段 redaction、访问审计与受控导出。哈希、签名、留存、授权与秘密脱敏分别解决不同问题，不能互相替代。

Finding 自动进入“已验证”需同时满足：在授权/产品上限内、来自真实 Action、证据覆盖足够、验证器可复核、环境与时间窗口明确。Planner/Worker 的 confidence 或正常返回不能单独满足这些条件。

## 7. 面向重建的执行技术路线

### 7.1 推荐职责划分

采用模块化控制面加独立 Runner，先不按领域拆大量微服务。控制面负责身份、授权、任务/探索图、策略、持久编排、模型预算、证据与报告；Agent 负责生成/修订假设和 Action 建议；Runner 负责执行已经 admission 的能力并上传回执/证据。控制面与 Runner 至少是不同进程与权限/网络边界。

Go 适合继承现有控制面、Runner、代理、数据库事务与流式接口积累。若团队有成熟 Go Agent 开发能力，可以继续 Go；若希望更快接入模型与研究工具，Python 可承担独立 Agent service/工具 adapter。不要为了“全栈统一”把所有执行直接放回 Web API 进程，也不要只因为 Agent 生态丰富而重写已经成熟的证据与调度代码。

| 选择                                    | 适用条件                                                    | 建议                                                                                      |
| --------------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Temporal + Go workflow/activity         | 高自主、长任务、跨重启审批/取消、独立 Runner 是明确首版要求 | 推荐目标方案；业务状态与证据仍以 PG/对象存储保存，workflow history 不等于全部业务审计     |
| PostgreSQL durable job + outbox + lease | 团队规模很小、离线交付对组件数量极敏感                      | 可作缩减方案，但需真正实现 lease/fencing/attempt/reconcile/审批恢复；不能只加一个内存队列 |
| LangGraph 或其他 Agent graph            | 需要 checkpoint、复杂模型分支和 Agent 迭代                  | 可用于模型推理层，不能替代授权、Runner 隔离、业务事务和外部动作恢复                       |
| Redis/消息队列                          | 需要通知加速、缓存或更高吞吐                                | 作为可重建派生层；不要让唯一任务真相、审批或审计只存在这里                                |

Temporal activity 重试也不会让任意工具天然幂等；工具执行必须使用上面的 Action 契约。Temporal 是减少持久编排自研负担的选择，不是安全边界，也不免除网络隔离/证据工程。

### 7.2 首版必须先落实的契约

1. 任务授权快照和企业硬规则独立于 Agent 图；每个 Action 和最终网络连接校验范围/预算，候选资产发现不能扩大权限。
2. 单一执行 admission，覆盖 Bash/shell/custom/MCP/deferred 的真实内层动作；未经注册能力不能因名字或 unlock 自动执行。
3. 持久 Action/Attempt、Runner 租约与 fencing；模型故障只能重试尚未交付的模型调用，不隐式重放已执行动作。
4. 确定性预算和停止条件：模型 token/成本、时间、请求、并发、目标速率、无新事实次数；预算耗尽产生清晰终态。
5. 证据与动作绑定，区分 observed/verified/inconclusive；独立验证器与复测合同复用 ARTEX 的正确设计。
6. 能力版本与依赖固定，离线镜像与配置导出可复现；审计/策略/DB 不可用时暂停新的外部动作。

### 7.3 后续验收应测行为，不只测 API 成功

本次未执行以下测试；它们是新平台验收建议。

- Action 已提交、Runner 尚未回执时杀控制面/Runner，恢复后不得不加区分地重复执行，无法判定时保留 unknown。
- 两个调度实例同时领取、过期 Runner 迟到回写、多次人工继续，均应满足并发上限和 fencing。
- 调度触发登记与 run 创建之间故障，恢复后既不丢触发也不重复创建相同 run。
- 模型在工具输入提交前/后发生 quota、429、5xx，验证安全降级与动作不重放，旧 chain revision 不覆盖新配置。
- Bash、custom command、Python、MCP、deferred tool、浏览器、重定向等入口都受相同任务范围和停止策略约束。
- deadline/预算到期后撤销 capability、关闭出口并回收进程树；后台工具不能继续产生未授权请求。
- 原始 Traffic 删除、证据文件被改、对象缺失、捕获只覆盖部分流量、报告版本冻结与复测环境变化，均产生准确状态。
- 策略/证据/审计持久化不可用时停止新动作；SSE 断线只影响 UI，不改变任务真相。

## 8. 综合取长补短

保留 ARTEX 的图驱动探索、清晰 Planner/Worker 分工、固定 mandate、按需 trace、Intent/Fact/Finding 血缘、细分终态、模型链 revision 与流式提交边界、独立证据副本和严格复测标准。

将内存执行协调、对话级恢复、主机通用工具、名字/schema 层的能力过滤和 prompt 范围约束，提升为持久 Action、统一策略 admission、隔离 Runner、网络最终目的校验和可复核执行回执。前者是已有产品设计的长处；后者是高自主私有化平台必须承担的工程工作。
