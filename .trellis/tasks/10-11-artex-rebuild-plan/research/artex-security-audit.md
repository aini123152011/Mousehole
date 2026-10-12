# ARTEX 安全边界、执行隔离与证据链专项梳理

审查日期：2026-10-12。范围：`E:/Crazywhite/ARTEX` 当前源码与随仓部署文件。方法为只读静态审查；没有启动平台、执行目标探测、运行测试或验证利用。以下“事实”指可直接从源码确认的实现；“推断/待验证”指部署或运行时结论，不能作为实测结果。

重建前提已确认：单企业私有化部署，高自主 Agent，自主操作上限为主动枚举与非破坏性验证。目标写入、账户/配置变更、驻留和影响可用性的操作不进入首版；审批不能突破产品上限，也不能扩大授权范围。

## 1. 结论与优先级

ARTEX 已经具备有价值的安全治理和证据工程：资产闸门、工具拦截与审批、平台拒绝原因提示、审查快照与工具结果关联、独立证据副本、哈希校验、版本冻结，以及较细致的通知凭据保护。重建应复用这些业务契约和成熟细节。

主要不足是这些机制尚未组合成高自主执行的强制边界：资产闸门位于知识入库/意图下发路径，执行依赖本地进程与可选代理，通用鉴权只有一个共享身份，部分规则/模型/日志故障不会阻止执行。不能将“有审批页面、有资产规则、有代理”直接等同于“不可越权、不能直连、可完整审计”。

| 重建优先级 | 具体事项                                                              | 为什么影响当前定位                                                           |
| ---------- | --------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| P0         | 独立、不可被 Agent 修改的授权快照；在每次执行和最终网络连接前强制校验 | 高自主 Agent 必须在既定范围内持续工作，发现新资产不能自动获得权限            |
| P0         | 控制面与 Runner 分离；Runner 默认拒绝出网，按任务授予最小能力         | ARTEX 工具运行于平台本地主机/容器，代理配置不是 OS 网络隔离                  |
| P0         | 将首版操作上限做成不可由普通 allow/审批覆盖的硬规则                   | 现有破坏/外传限制是普通可编辑规则，且工具拦截默认并非覆盖所有工具            |
| P0         | 独立用户身份、任务权限和审批者身份；秘密按需注入                      | 私有化部署仍然需要区分管理员、评估人员、审核人员及只读用户                   |
| P0         | 持久执行状态、审批快照、不可重复执行契约；安全依赖故障时暂停新执行    | DB 中的 pending 记录不等于重启后能恢复原执行；进程内 waiter 不能作为持久编排 |
| P0         | 证据覆盖状态和执行前记录；缺证据不得自动标成“已验证”                  | ARTEX 可关闭捕获，MITM 失败后可透传；哈希只能证明保存后内容一致              |
| P1         | 签名证据清单、独立审计锚、保留策略和受控导出                          | 高自主任务需要证明谁授权、谁执行、如何得到结论，而不只是保存 HTTP 文本       |
| P1         | 固定工具/MCP/浏览器版本，镜像 digest，离线安装与恢复演练              | 随仓 Docker 采用 latest，控制面与全部工具共用运行镜像                        |

## 2. 身份认证与授权

### 2.1 实现事实

- [auth.go:20](E:/Crazywhite/ARTEX/server/auth.go:20) 使用一个 `auth.password_hash`，JWT 有效期七天。[auth.go:87](E:/Crazywhite/ARTEX/server/auth.go:87) 签发 HS256，`sub` 固定为 `ARTEX`，没有不同用户、角色、会话 ID、撤销状态、issuer/audience 的业务模型。
- [auth.go:108](E:/Crazywhite/ARTEX/server/auth.go:108) 可从 Bearer、cookie 或 URL 查询参数取得完整 JWT。[auth.go:120](E:/Crazywhite/ARTEX/server/auth.go:120) 对 API 校验 token 后直接调用 handler，没有将真实用户/任务权限写入请求上下文。`/api/auth/*` 与 health 豁免通用中间件，其中改密接口单独验证 token 与当前密码。
- [auth.go:211](E:/Crazywhite/ARTEX/server/auth.go:211) 改密成功仅更新密码 hash，已签发 JWT 没有随之撤销。[auth.ts:21](E:/Crazywhite/ARTEX/web/src/lib/auth.ts:21) 浏览器在 localStorage 和 JS 可读 cookie 中保存 token；cookie 为 SameSite=Lax，未设置 HttpOnly/Secure。[auth.ts:41](E:/Crazywhite/ARTEX/web/src/lib/auth.ts:41) 展示用用户 ID 固定为 `1`、角色固定为 `operator`。
- [api.ts:189](E:/Crazywhite/ARTEX/web/src/lib/api.ts:189) SSE 将完整 token 放入 URL。这会增加 token 出现在反向代理/access log 等位置的机会；本次没有检查实际日志配置，不声称已经发生泄露。
- [proxy.ts:11](E:/Crazywhite/ARTEX/web/src/proxy.ts:11) 前端页面路由只检查 cookie 存在；真实 API 验证仍在 Go 后端，不能据此宣称 API 可被任意绕过。
- [server.go:938](E:/Crazywhite/ARTEX/server/server.go:938) API 统一使用 CORS 和 requireAuth。[server.go:3880](E:/Crazywhite/ARTEX/server/server.go:3880) CORS 使用 `*`；这是配置事实，不代表本次已经验证跨站读取或 CSRF 利用。涉及 cookie 的写接口仍应结合具体部署、浏览器行为和 Origin/CSRF 机制评估。
- 本次在认证实现与 `server` 的相关搜索中未发现登录限速/失败尝试策略，不将此扩大为“全仓绝无任何限速”。

### 2.2 应继承的细节

[auth.go:48](E:/Crazywhite/ARTEX/server/auth.go:48) 已将 JWT key 移出可浏览的工作区，使用安全随机源并以 0600 写入；旧 key 迁移时保留会话兼容。[auth.go:140](E:/Crazywhite/ARTEX/server/auth.go:140) 认证数据源失败返回 503。[auth.go:157](E:/Crazywhite/ARTEX/server/auth.go:157) 首次设密借助数据库原子“只插入不存在值”，避免并发覆盖。这些是应保留的失败处理与存储边界。

### 2.3 重建建议

优先对接企业 OIDC/LDAP 身份提供方；离线交付可以提供本地用户，但仍必须有真实 user/session/role。API 根据身份与任务成员授权，区分配置修改、任务发起、审批、原始证据查看、导出与审计读取。签名身份不能只有共享 `ARTEX`。

浏览器会话由服务端设置 HttpOnly、Secure cookie，配套 Origin/CSRF 防护；SSE 用同站会话或短时、单用途订阅票据，避免把长期访问 token 放在 URL。改密、禁用用户、管理员撤销、会话登出应能令相关会话失效。鉴权与对象权限验证留在 API，前端守卫只承担体验。

## 3. 工具拦截与授权范围

### 3.1 工具拦截不等于不可突破的能力上限

- [guard.go:1](E:/Crazywhite/ARTEX/guard/guard.go:1) 明确说明旧 RoE 机制已移除；破坏/外传限制通过普通 DB 拦截规则实现，可以被禁用或删除。注意：这不代表整个项目没有资产规则，见下一节。
- [guard.go:83](E:/Crazywhite/ARTEX/guard/guard.go:83) interceptor 为空、工具未启用拦截、无规则命中且 judge 未参与时返回允许；未知 action 也最终返回允许。[intercept.go:141](E:/Crazywhite/ARTEX/intercept/intercept.go:141) 默认拦截工具列表是 Bash、WebFetch、web_search、shell_open/send、Write/Edit/MultiEdit，MCP 等工具不在该默认列表中。
- [intercept.go:1](E:/Crazywhite/ARTEX/intercept/intercept.go:1) 规则按优先级从高到低首命中，不是所有 deny 汇总；因此高优先级 allow 可以先于较低优先级 deny 生效。
- [intercept.go:165](E:/Crazywhite/ARTEX/intercept/intercept.go:165) 无效 regex 被跳过；enabled-tools JSON 解析失败变为空 map。[intercept.go:508](E:/Crazywhite/ARTEX/intercept/intercept.go:508) 规则读取失败与无匹配使用相同返回形态。结合 Guard 默认分支，部分配置/读取失败会失去预期拦截。
- [intercept.go:313](E:/Crazywhite/ARTEX/intercept/intercept.go:313) LLM judge 默认失败策略是 `allow`，人工 ask 默认超时 `deny`。[intercept.go:423](E:/Crazywhite/ARTEX/intercept/intercept.go:423) 模型失败或结果无法解析使用配置的 FailAction；上下文不完整则转 ask。不能笼统说“所有审批失败都放行”。
- [intercept.go:561](E:/Crazywhite/ARTEX/intercept/intercept.go:561) allow/deny 决策持久日志是 best-effort，写 DB 失败不改变执行结果。[guard.go:77](E:/Crazywhite/ARTEX/guard/guard.go:77) 内存日志在最终判断前先写 allow，[guard.go:181](E:/Crazywhite/ARTEX/guard/guard.go:181) 只保留最近 2000 条，不能直接作为最终、完整的执行审计。

### 3.2 确实存在资产闸门，需要保留并重新定位

[asset_intercept_match.go:45](E:/Crazywhite/ARTEX/db/asset_intercept_match.go:45) 支持 domain/IP/URL 精确、模糊字符串和 CIDR 匹配。[asset_intercept_match.go:178](E:/Crazywhite/ARTEX/db/asset_intercept_match.go:178) 资产闸门 **block 优先**；存在启用 allow 时需要命中 allow，无 allow 时默认允许。它与上面的工具首命中规则是两套不同机制。

[asset_intercept_match.go:214](E:/Crazywhite/ARTEX/db/asset_intercept_match.go:214) 按资产 ID 取得候选并组合全局 block、任务 block 和任务 allow。[tools.go:1051](E:/Crazywhite/ARTEX/agent/tools.go:1051) `add_intent` 在存在 asset anchors 时调用该闸门，规则/资产读取错误会拒绝创建；没有 anchor 不走这条校验。[tools_insert.go:230](E:/Crazywhite/ARTEX/agent/tools_insert.go:230) `insert_assets` 也执行闸门，但规则读取失败会跳过判定。

因此应准确描述为：**ARTEX 有资产入库和意图下发的范围治理，但这些检查不等于执行时的目的地址强制授权。** 当前代理的选择拦截函数没有接入上述资产闸门；任意命令、脚本、自定义 HTTP/MCP 的最终连接也没有共享的不可绕过目的地址校验。被发现、被存库、被关联到意图，都不能天然获得测试权限。

[worker.go:408](E:/Crazywhite/ARTEX/agent/worker.go:408) 在资产覆盖度功能开启时，将意图锚定资产 `AddAutoScope` 到任务测试范围；注释明确该范围用于覆盖度分母。应将它理解为覆盖统计范围，不能据此宣称已经证明自动扩权。重建应分开保存“发现资产”“统计覆盖集合”和“不可变授权范围”，避免名称相似而混用。

### 3.3 应继承与应重写的契约

应继承资产 block 优先，以及 [tools.go:1028](E:/Crazywhite/ARTEX/agent/tools.go:1028) 意图需要追溯到 fact/finding 的约束。资产范围与发现血缘是两种独立数据，重建时不得合并。

应继承 [guard.go:126](E:/Crazywhite/ARTEX/guard/guard.go:126) “平台管控·非目标防御”的 Agent 反馈：模型明确知道它遇到的是平台政策，不能作为 WAF 绕过问题继续尝试。还应加入稳定 reason code 和被禁止的能力类别，避免单靠自然语言反馈。

重建采用三层交集：`企业硬规则 ∩ 任务授权快照 ∩ 当前执行能力许可`。人类审批只能批准交集内需要确认的动作。任务授权快照包含资产表达、排除范围、允许动作、有效期、并发/速率/请求/时间预算及授权版本；Agent、目标返回内容、普通工具规则均不能修改它。发现新域名、跳转地址或 IP 时先登记候选，不能立即测试。

默认仅允许注册、带类型参数与副作用声明的能力。LLM 可以解释风险或建议 ask，最终硬约束由确定性策略决定。未知工具、无效配置、审批过期、策略服务不可用时暂停执行；策略拒绝没有更高优先级的普通 allow 覆盖通道。非破坏性也不能仅根据 HTTP method 判断，GET 可能改变状态，需要结合具体工具行为与业务前提。

### 3.4 SDK 工具可见性与嵌套调用也需要统一执行边界

[query.go:594](E:/Crazywhite/ARTEX/norma/harness/query.go:594) 进入 settlement 后，通过 `filterToolSchemas` 隐藏 DisabledTools；[tools.go:28](E:/Crazywhite/ARTEX/norma/harness/tools.go:28) 执行仍从完整 registry 取得工具，没有在该入口检查 settlement 的 DisabledTools。[worker.go:425](E:/Crazywhite/ARTEX/agent/worker.go:425) 的收尾阶段仅列出 Bash。隐藏给模型的 schema 能引导模型收尾，但不是执行能力撤销，也没有覆盖具有同类执行能力的自定义/MCP 工具。

[tools.go:57](E:/Crazywhite/ARTEX/norma/harness/tools.go:57) 对外层工具执行 permission 与 PreToolUse hook，再调用工具。[deferred.go:240](E:/Crazywhite/ARTEX/norma/tool/deferred.go:240) 的 `ExecuteExtraTool` 自身 permission 返回 Allowed；其内部会校验 registry、unlock 和参数 schema，却在 [deferred.go:269](E:/Crazywhite/ARTEX/norma/tool/deferred.go:269) 直接调用目标工具 `Call`，没有重复执行目标工具的 CheckPermissions/PreToolUse。[tool.go:161](E:/Crazywhite/ARTEX/norma/tool/tool.go:161) 的 `Call` 本身只是转发到 Run。这里仍可能存在 ARTEX 对外层工具的拦截，不能由此声称整个平台完全没有动作管控或本次已实测绕过。

类似地，[customtool.go:338](E:/Crazywhite/ARTEX/server/customtool.go:338) 注释声称自定义 command 复用 Bash 能继承安全 floor，但 [customtool.go:346](E:/Crazywhite/ARTEX/server/customtool.go:346) 直接 `NewBash().Call`，没有执行 Bash 的 CheckPermissions，因此没有通过这次调用继承位于 Bash permission 内的灾难性命令拒绝。这是 SDK 内层权限链缺口，不能扩大为自定义工具一定绕过了所有 ARTEX 规则。

重建统一 execution dispatcher：所有直接、deferred、MCP、自定义及工具内部调用都将“实际能力、展开后的参数、任务阶段”提交同一 admission，再交给 Runner。收尾/取消/预算耗尽应撤销执行能力，仅保留受限的记录、证据固定和总结能力。不能把 prompt、schema 隐藏、unlock 标记或某个外层 wrapper 的 allow 当作实际执行授权。

## 4. 审批持久性与状态真实性

### 4.1 值得继承

[intercept.go:284](E:/Crazywhite/ARTEX/intercept/intercept.go:284) 的 Decision 保存模型输入摘要、配置摘要、规则/模型 profile 与超时信息。[intercept_detail.go:19](E:/Crazywhite/ARTEX/db/intercept_detail.go:19) 审查时抓取上下文，明确 exact/ambiguous/unavailable；旧行不伪造重建快照。

[intercept.go:590](E:/Crazywhite/ARTEX/intercept/intercept.go:590) 创建 pending 后注册 waiter 并重新读取 DB，处理“刚 INSERT 就有人审批”的竞态。[intercept_detail.go:74](E:/Crazywhite/ARTEX/db/intercept_detail.go:74) `WHERE status='pending'` 原子决策避免超时覆盖人工结论。[intercept_detail.go:99](E:/Crazywhite/ARTEX/db/intercept_detail.go:99) 只对 exact run/tool、已允许且 awaiting_result 的记录追加执行结果，避免把“未执行的拦截”展示成“工具执行失败”。这些契约应纳入回归验收。

### 4.2 局限与重建建议

[intercept.go:85](E:/Crazywhite/ARTEX/intercept/intercept.go:85) 执行等待依赖进程内 channel；DB 中持久 pending 不等于 waiter/原始执行能够跨进程恢复。本次未找到审批与原执行的重启恢复协调，不宣称恢复已经实现。[server/intercept.go:353](E:/Crazywhite/ARTEX/server/intercept.go:353) 接口提交 decision，调用 `Decide(id, bool)`；[intercept.go:671](E:/Crazywhite/ARTEX/intercept/intercept.go:671) 与审查记录没有审批者真实身份参数。

重建为持久状态机：planned → admitted → awaiting_approval（必要时）→ leased → running → completed/failed/cancelled/unknown。审批绑定 `operation_id + 输入摘要 + 授权版本 + 策略版本 + 工具版本 + 审批人 + 到期时间`，任一变化都需重新 admission。Runner 领取短时租约，执行前校验；已过期 approval 不得复用。

控制面重启、网络中断后先 reconcile，不可直接把未知结果当失败再重试。远端副作用无法靠数据库做到绝对 exactly-once，应使用 operation ID、幂等能力及执行回执；不确定时保留 unknown 并复核。任务取消需要撤销能力令牌、关闭代理出口并终止整棵进程树，不能只取消等待审批的 Go context。

首版审批的用途是授权范围内的歧义复核或运行暂停，不提供目标写入/驻留等产品上限之外的解锁入口。

## 5. 执行进程、网络与部署隔离

### 5.1 执行事实

[bash.go:17](E:/Crazywhite/ARTEX/norma/tool/bash.go:17) 自带少量灾难性命令正则硬拒绝，并明确说明不是完整 sandbox。[bash.go:155](E:/Crazywhite/ARTEX/norma/tool/bash.go:155) 直接 `exec.CommandContext` 执行 shell；[bash.go:140](E:/Crazywhite/ARTEX/norma/tool/bash.go:140) 带 TaskManager 的前台命令超时还可转后台，超时不一定代表进程停止。

[customtool.go:398](E:/Crazywhite/ARTEX/server/customtool.go:398) 自定义 Python 在平台本地启动，使用 workingDir 并继承 `os.Environ()` 加 sessionEnv。[stdio.go:32](E:/Crazywhite/ARTEX/norma/mcp/stdio.go:32) stdio MCP 也是本地子进程，env 在父环境上叠加；未显式设置 Cmd.Env 时 Go 仍继承父环境。结合 Compose 的 LLM key 环境变量，工具具有接触平台环境秘密的机会；这是进程与权限模型推断，本次没有执行秘密读取验证。

### 5.2 代理事实

- [manager.go:377](E:/Crazywhite/ARTEX/server/manager.go:377) 捕获代理初始化/停止失败会记录日志；[manager.go:412](E:/Crazywhite/ARTEX/server/manager.go:412) capture 默认 false。[manager.go:740](E:/Crazywhite/ARTEX/server/manager.go:740) capture 关闭后使用全局代理或直连。
- [traffic.go:164](E:/Crazywhite/ARTEX/traffic/traffic.go:164) MITM 因协议/代理问题失败的 host 加入透传集合，后续可继续连接且不录制；透传仍遵守已配置 global upstream，不能说它绕过 global proxy。[traffic.go:204](E:/Crazywhite/ARTEX/traffic/traffic.go:204) 使用 `SslInsecure: true`；[traffic.go:213](E:/Crazywhite/ARTEX/traffic/traffic.go:213) 无 global upstream 时强制 direct。
- [manager.go:635](E:/Crazywhite/ARTEX/server/manager.go:635) seeded browser MCP 同步代理与 CA，被删除/改名则 no-op。这种参数注入是集成便利，不是底层网络强制路由。
- [customtool.go:420](E:/Crazywhite/ARTEX/server/customtool.go:420) 自定义 HTTP 工具使用模板 URL、headers/body 和可选录制代理；[customtool.go:470](E:/Crazywhite/ARTEX/server/customtool.go:470) 未配 proxy 返回 nil，未见该路径做任务范围或最终连接目的地址校验。

### 5.3 随仓部署事实与推断

[Dockerfile:43](E:/Crazywhite/ARTEX/Dockerfile:43) 将网络/枚举工具装入运行镜像，[Dockerfile:54](E:/Crazywhite/ARTEX/Dockerfile:54) Playwright/MCP/CLI 使用 latest，控制面、执行工具、skill 共用该镜像。文件没有 USER，因此按该 Dockerfile 构建且无外部覆盖时使用基础镜像默认 root；这不证明所有真实客户都以 root 运行。

[docker-compose.yml:29](E:/Crazywhite/ARTEX/docker-compose.yml:29) PostgreSQL DSN 使用 sslmode=disable，平台环境包括 LLM keys；API 8787 对所有 host 接口暴露，8788 host 映射绑定 loopback。仅 data 持久化、skills 可写绑定；没有独立 Runner、cap_drop/read_only/资源限额等配置。

`loadOrCreateJWTKey` 现在使用 executable 同级目录，Compose 仍把 data 注释成包含 jwt.key。在该默认布局下 `/app/jwt.key` 不在 `/app/data` 持久卷里，重建容器可能生成新 key、令会话失效；需核对实际部署是否另有持久挂载。CLI API 使用 ListenAndServe，随仓启动本身没有原生 TLS，客户反向代理是否补 TLS 未验证。

### 5.4 重建方案

控制面与 Runner 分开部署。控制面负责企业身份、任务、策略、编排、模型网关、证据与报告；Runner 使用独立用户与网络、短时任务能力，无法访问控制面 DB、secret store、宿主 Docker socket、其他任务文件或企业身份管理网络。Go 可承担控制面/Runner 调度与网络代理，Python 承担 Agent 及工具适配，但语言本身不提供隔离。

单企业首版可使用专用 Linux Runner 主机/VM 加 rootless 容器与内核权限/资源限制；对可运行任意命令的 Runner 预留 gVisor 或 microVM 隔离。Compose 适合交付轻量控制面，不能以“整个产品放一个容器”代替执行隔离。Kubernetes 是后续调度选项，不是首版必需，也不自动解决隔离。

Runner 的底层出网策略默认 deny：HTTP/browser 经受控网关；允许的原生网络工具经独立出口策略，并产生连接元数据。HTTP 代理环境变量可以被程序忽略，必须在 network namespace/firewall/网关层阻断直连、替代 DNS、IPv6/UDP 等旁路。规则对规范化 host、解析所得 IP、实际 dial 地址、redirect 每跳、port/protocol、CONNECT 和可观测的 Host/SNI 做一致校验，并明确对域名/CDN/动态地址的授权含义。

同一个域名授权不自动授权其解析出的任意共享地址；范围校验需结合 DNS 策略并在 dial-time 检查，避免“校验时一个地址，连接时另一个地址”。企业评估允许私网资产，不能照搬通知端点过滤为“所有 RFC1918 拒绝”；控制面、DB、secret store、metadata 等管理地址则始终排除。

证据必须采集的验证动作遇到录制不可用时暂停，不能自动转 unrecorded 并产生 verified finding。并非所有流量都可 MITM：原生协议保存连接/命令/退出码/工具原始输出，HTTP 根据能力需要保存受控原始交换；明确 coverage=full/partial/unavailable 和原因，不承诺“全部协议完整正文”。

## 6. 秘密与通知

### 6.1 存储事实

[config.go:14](E:/Crazywhite/ARTEX/db/config.go:14) LLM APIKey 标记 `json:"-"`，UI 用 hint；[config.go:159](E:/Crazywhite/ARTEX/db/config.go:159) 应用直接把 APIKey 值绑定到 `llm_profiles.api_key` INSERT/UPDATE，[schema.sql:369](E:/Crazywhite/ARTEX/db/schema.sql:369) 字段为 TEXT，本路径未见应用层 envelope encryption。数据库磁盘或宿主机加密是否存在未验证。

[config.go:174](E:/Crazywhite/ARTEX/db/config.go:174) 空 key 更新保留旧值，是避免配置编辑误删凭据的好细节。必须同时保证“目的端点变化”不能沿用旧秘密。

### 6.2 通知模块有可继承的完整细节

[http.go:41](E:/Crazywhite/ARTEX/notify/http.go:41) 规范化 IPv4-mapped IPv6，默认拒绝 loopback/link-local/unspecified/multicast，允许 RFC1918；[http.go:59](E:/Crazywhite/ARTEX/notify/http.go:59) dial-time 检查最终地址，配合拒绝跨 Host redirect、15s timeout 与小响应体上限。环境变量显式允许本地时可放宽；DefaultTransport 的环境代理路径也须计入部署评估，不能把这一实现描述为对所有网络路径的绝对 SSRF 防护。

[http.go:142](E:/Crazywhite/ARTEX/notify/http.go:142) 对 URL 与 transport error 脱敏，避免把秘密写入 history/log/API。[notify_api.go:67](E:/Crazywhite/ARTEX/server/notify_api.go:67) API 按渠道 SecretKeys 回显掩码。

[mask.go:78](E:/Crazywhite/ARTEX/notify/mask.go:78) 修改通知目的地址时，要求对每一个秘密字段显式提供新值或清空，拒绝“修改目标但隐式沿用旧凭据”；[mask.go:109](E:/Crazywhite/ARTEX/notify/mask.go:109) 拒绝把掩码哨兵藏在 JSON 容器内部。这些细节应作为全平台 provider/proxy/MCP/SMTP/Webhook 配置更新契约，价值高于单纯把输入框显示成星号。

### 6.3 重建建议

秘密以引用形式存配置，使用部署方 KMS/Vault 或本地独立加密主密钥保护；主密钥不能与密文位于同一可浏览工作区。平台给 Runner 只注入此次工具所需秘密，不能完整继承控制面环境。浏览器/目标凭据按任务及账号用途隔离；凭据轮换、读取、注入、目的端点变化有身份审计。

将原始证据与脱敏视图区分：原始交换可能包含 Cookie、Authorization、业务隐私，只有获准角色可访问；Agent/普通页面/通知和默认导出使用脱敏视图。原始证据受加密与保留策略保护，不能先改写原始正文再声称 hash 是原始数据 hash。

## 7. 流量证据、归档与审计

### 7.1 ARTEX 已经做得较好的工程机制

- [evidence/store.go:33](E:/Crazywhite/ARTEX/evidence/store.go:33) 严格校验 SHA256 路径格式，读取校验 hash 与长度。[store.go:60](E:/Crazywhite/ARTEX/evidence/store.go:60) staging 写入、校验、fsync、rename、目录 fsync；SQL 失败留下的未引用 blob 经宽限期 GC 处理。
- [store.go:146](E:/Crazywhite/ARTEX/evidence/store.go:146) 从真实 captured traffic 获取请求/响应，复制到独立证据区，并用规范化 metadata 创建快照 ID。[traffic/evidence.go:16](E:/Crazywhite/ARTEX/traffic/evidence.go:16) 读取时锁住 writer 保持 snapshot 一致。
- [store.go:118](E:/Crazywhite/ARTEX/evidence/store.go:118) 在 HTTP 导出响应开始前冻结 finding/report/evidence version 并复制正文；[store.go:265](E:/Crazywhite/ARTEX/evidence/store.go:265) 是独立真实副本，证据不依赖可随时删除的 traffic blob。[store.go:314](E:/Crazywhite/ARTEX/evidence/store.go:314) 恢复时同时验证 metadata ID 和 body hash/长度。
- [traffic/archive.go:44](E:/Crazywhite/ARTEX/traffic/archive.go:44) 归档流量有一致快照，[archive.go:130](E:/Crazywhite/ARTEX/traffic/archive.go:130) 导入做版本/hash 校验与幂等处理。
- [findings.go:371](E:/Crazywhite/ARTEX/db/findings.go:371) follow-up intent、finding 血缘与 audit activity 在同一事务内生成；写图关系与写审计的一致性值得继承。

### 7.2 必须准确界定的局限

内容寻址与哈希验证可证明“当前内容与保存摘要一致”，不能独立证明“确实来自目标”、时间真实性或管理员未把 metadata/hash/文件一起改掉。本次审查路径未见独立签名锚、WORM 或管理员不可覆写的审计边界；这不能表述为全仓已经验证无法实现防篡改。

[traffic/archive.go:95](E:/Crazywhite/ARTEX/traffic/archive.go:95) 保存 raw HTTP 请求/响应，没有在该保存路径做脱敏；原始流量本来有证据价值，问题是需要受控访问与脱敏派生版本。[archive.go:164](E:/Crazywhite/ARTEX/traffic/archive.go:164) 导入 traffic body 使用 0644，和证据私有目录策略不同。

审批 audit 是在 [intercept_detail.go:87](E:/Crazywhite/ARTEX/db/intercept_detail.go:87) 中 UPDATE JSON，具有原子状态保护，却不是 append-only/抗管理员篡改日志。[schema.sql:298](E:/Crazywhite/ARTEX/db/schema.sql:298) activity 随 exploration DELETE CASCADE；[intent_admission.go:8](E:/Crazywhite/ARTEX/db/intent_admission.go:8) 删除 intent 显式删除对应 activity，[exploration.go:540](E:/Crazywhite/ARTEX/db/exploration.go:540) 批删除也清理 activity（保留 token rollup）。所以 UI 运行轨迹和独立审计日志应分开建模，不能要求同一 activity 表同时满足可重置体验与不可删审计。

通用 JWT 中间件没有不同用户角色，意味着目前不能靠展示用 `operator` 区分“可审批”“可删任务”“可看原始流量”等权限。实际各 API 还应逐对象验证，不把“已登录”当任务级权限。

### 7.3 重建证据模型

保存四类关联对象：`operation/attempt`（确定的操作与尝试）、`observation`（工具观测）、`evidence_snapshot`（不可变正文与元数据）、`finding_version`（论断及其支持证据）。Asset ID、task ID、run ID、tool_use ID、scope/policy/tool version、actor/runner identity、trace ID 全程连接。

Agent 输出先是 hypothesis/candidate，满足验证规则与证据覆盖条件后才能成为 verified；验证失败、受限、unknown 不得包装成 confirmed。报告固定 finding version 与 evidence manifest，后续证据更新生成新版本并标记旧报告过时。

沿用 ARTEX 的 immutable blob + 版本绑定 + 导出冻结；在此基础上增加 Runner 签名的执行/采集清单、控制面接收时间、独立保留的审计摘要与受控保留策略。签名证明某个身份对清单负责，不保证被攻陷 Runner 一定采集真实，因此仍需隔离、受控采集路径和必要的独立复核。原始正文宜进入对象存储，PostgreSQL 保存结构化索引/关系/状态；不可默认追加图数据库来解决完整性问题。

## 8. 应写入 MVP 验收的行为

这些是未来测试建议，本次没有新增或执行代码测试。

1. Agent 发现、重定向或请求未授权目的地：资产可以登记为候选，execution admission 与最终连接均拒绝，不产生实际目标请求。
2. shell/Python/browser/MCP 尝试绕过 HTTP_PROXY、IPv6/UDP/替代 DNS 或访问控制面：底层网络边界拒绝；task allow 不能覆盖管理地址禁区。
3. 任意工具声明包含目标写入、账户变更、驻留或可用性影响：硬规则拒绝；审批页面无法解锁。
4. 规则 DB、策略引擎、审计持久化或必要录制不可用：暂停新执行；已运行操作进入明确状态，不自动直连或伪造已验证结论。
5. approval 后修改参数、tool version、scope version 或到期：审批不能复用；两个人同时审批只产生一个有效决策且记录真实身份。
6. 控制面/Runner 重启或回执丢失：重新协调状态；无法确定的非幂等操作保持 unknown，不盲重试；取消后后台进程和出口令牌均停止。
7. 删除任务/清空聊天/归档：业务轨迹按产品语义处理，独立审计仍可追溯；已绑定证据不会因 disposable traffic 删除而消失。
8. 损坏归档正文、metadata 或版本：恢复拒绝；导出期间证据更新仍导出同一冻结版本；报告不混合不同版本。
9. 修改模型/Webhook/SMTP/MCP 目的端点且回传旧掩码：拒绝隐式沿用秘密；URL、错误消息、Agent 上下文和普通导出不泄露长期凭据。
10. 同一私有化企业的不同用户：只有有任务权限的用户能查看/操作对应对象，原始证据、审批和配置权限各自验证。

## 9. 对总体技术路线的约束

推荐的组合是 Go 控制面及 Runner 管理、Python Agent/工具适配、TypeScript 前端、PostgreSQL 主数据与持久状态、S3 兼容证据存储、企业 OIDC、本地隔离 Runner 与受控出口。确定性策略用版本化结构规则或经过评估的策略引擎；LLM judge 作为辅助审查，不承担最终授权。

长任务编排可选择 Temporal 或明确实现租约/状态机/恢复协调的 PostgreSQL 作业模型，具体由团队经验与离线运维成本决定。无论选哪一个，进程内 pending map 和临时 channel 都不能继续承担跨重启的可靠执行契约；也不能因增加框架就宣称远端操作 exactly-once。

首版复杂度应该投入到“授权范围—计划—受控执行—观测—验证—证据—报告—复测”的闭环。多租户、计费、复杂图数据库和大规模集群属于以后可演进的能力；控制面/执行面分离、证据真实性、身份与能力限制不能留到以后。
