# P0 授权、资产与静态预检查独立复核

本记录由已派遣的独立 `trellis-check` 代理维护。审查完成，已发现问题均由代码 owner 修复并经本代理复核，未发现未修复的阻塞问题。实现目录仍由主会话和实施代理拥有，本代理仅修改本报告，未修改 backend/api/web，未提交、推送或归档。完整验收状态由主会话在交付文档登记，以下分别记录独立检查与主会话运行证据。

## 读取与边界

已读取本任务 PRD、设计、实施计划、check.jsonl 引用资料；后端数据库/错误/日志/工具/质量规范，前端状态/组件/类型/Hook/质量规范，以及父任务中文编码约定。ARTEX 仅沿既有只读研究证据核对，没有运行、构建、测试或目标扫描。

审查覆盖 target、asset、authorization、policy、access、scopecommand，旧 command/identity 共享权限变更，HTTP/CLI，00003/00004、SQL 查询，以及 Web client、项目路由、表单、分页、unknown 恢复与测试。公共接口和跨层修复由主会话分配后实施，本代理核对落盘结果，不越过代码所有权。

## 已发现并核对修复的问题

| 问题                                  | 可观察反例、最终修复与证据                                                                                                                                                                                                                                                                        |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CLI 操作引用受理/查询规范化不一致     | 带首尾空格的 operator_ref 经受理保存为规范值，原 AdminGet 却以原值比较。现在 AdminGet 使用 Text 返回值，生命周期真实 PG 测试已补带空格查询；完整后端门禁通过由主会话确认。                                                                                                                        |
| 旧草稿 unknown 被跨页面导航丢失       | 原 CommandWorkspace 仅挂载在 `/`，unknown 后访问资产再返回会丢原 ID 并解锁新命令。现在旧/新命令 Hook 均提升至 WorkspaceRoutes 项目祖先，原标题与原 ID/参数保留；定向 Vitest 与真实浏览器导航往返覆盖。                                                                                            |
| 授权保存丢失字段定位与 CIDR 建议      | HTTP 原只返回通用 invalid_request，吞掉领域层中文 CIDR masked 建议。现在受控 internal/validation 经 HTTP `validation_errors`、OpenAPI/client 到表单传播；仅接受合法 400/invalid_request 并映射已知字段，中文 alert/aria 描述保留输入，不公开 URL query/SQL。定向 Go/Vitest 和真实浏览器场景覆盖。 |
| 资产搜索返回时 cursor 与条件错配      | 新条件 B 会与旧页面 A 的 cursor 混用。现在 useAssetSearchState 以 location.key 在项目祖先内存恢复 snapshot，并在子查询渲染前恢复；不写敏感地址栏或 history state。单测和真实浏览器 A→B→后退 A→前进 B 覆盖。                                                                                       |
| 00004 预算数据库 CHECK 不完整         | 初稿 19 位正整数 regex 接受超过 int64 上限，且未拒绝 concurrency 大于 requests。现在补 numeric 上限与相对大小 CHECK，真实 PG 反例已补；主会话确认完整后端门禁通过。                                                                                                                               |
| 时窗策略反例不足                      | 初稿缺 active 未来/过期与 exact `[from,expires)` 决策断言。现在五例 Evaluate 时窗边界，以及未来/过期激活拒绝且状态/账本不变的 PG 回归均已补。精确 policy 测试由本代理定向执行通过，PG 来源为主会话完整门禁。                                                                                      |
| 幂等隔离与失败回滚测试有遗漏          | 原 otherActor 仅 viewer，必需事件故障仅验证资产导入。现在三种 Web kind × audit/outbox 回滚矩阵比较业务/账本增量；另一 analyst 的 GET/同参 PUT/异参 PUT 均 404、无回执/结果且计数不变；同 actor 同 ID 改 kind 为 409 且无额外业务。真实 PG 来源为主会话完整门禁。                                  |
| 授权与预检查缺真实多页证据            | 初稿 PG 仅遍历资产。现在 PG 创建 29 个版本/决策逐页核对，并覆盖 cursor 资源/筛选绑定；浏览器各创建 26 条，核对详情返回与首尾页完整性。PG 门禁与最终完整 E2E 通过来源为主会话。                                                                                                                    |
| 已有开发库 namespace 保护影响前向迁移 | 同名函数/trigger 已存在时不能修改已执行 00003 或删除历史。00004 使用 CREATE OR REPLACE FUNCTION，并核对已有 trigger 的函数绑定、tgtype=7 与 tgenabled='O'，不匹配则失败；随机 schema fixture 预装同保护后升级并核对旧回执保留。完整 PG 门禁与开发库前向迁移/status 均由主会话确认通过。           |
| E2E 在 Node 24 下未收集               | JSON import 缺 import attribute，首轮没有执行任何业务场景。support.ts 现使用 `with { type: 'json' }`；lint/typecheck/12 场景列表与后续真实运行确认收集问题已解决。                                                                                                                                |
| 规范签名与中文说明小错误              | error-handling.md 的 NewHandler 参数顺序与接口名已改为实际 variadic Scopes 签名；validation.Error() 中文说明已补。                                                                                                                                                                                |

## 最终界面修复核对

| 问题                   | 独立证据与处理状态                                                                                                                                                                                                                                   |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 绿色复制按钮文字不可见 | 失败轮真实授权截图中按钮背景与字色接近；普通链接规则覆盖 Button asChild foreground。现在 globals.css 的普通链接规则排除 `[data-slot='button']`；本代理已读实际 CSS，并独立查看最终授权桌面截图，复制按钮白色文字清晰。                               |
| 手机端不能选择项目     | 原手机规则隐藏 `.project-switcher`，真实 E2E combobox 不可见。现在手机布局为顶部两列加四个导航，保留真正项目 select、中文注释和宽度约束；本代理已核对 CSS/最终手机截图与真实 E2E，后者用 Home/ArrowDown 在双项目间往返，没有绕过隐藏控件或放宽断言。 |

## PA-A1–A9 审查证据索引

下表记录代码路径和已确认的运行来源；最终验收状态由主会话在交付文档登记，本代理不擅自改写 PA-A 状态。

| 验收  | 已核对代码与运行证据                                                                                                                                                                  | 剩余边界                                                         |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| PA-A1 | target 表驱动规范化与 asset 原子导入、稳定排序、并发去重；本代理纯函数测试通过，真实 PG 批次/项目关联/来源唯一由主会话完整门禁确认。                                                  | 导入是来源声明，不是已验证观测。                                 |
| PA-A2 | access 完整权限锁、项目关联入口、原 actor 回执、跨项目 FK；真实 PG viewer/analyst/失效成员/跨 actor 反例由主会话确认，最终独立项目成员撤销 E2E 已通过。                               | 合成 issuer 不替代真实企业 IdP 联调。                            |
| PA-A3 | 正文/条目不可变、延迟集合一致、candidate→active→superseded/revoked 与 CAS；生命周期/未来过期激活 PG 由主会话确认，精确 policy 边界本代理通过；最终复制按钮白字独立可见。              | 历史状态不等于当前许可。                                         |
| PA-A4 | PRIVATE 后缀、域名标签、CIDR/映射、URL scheme/port/path/query/尾 slash/空 query/前缀反例与私网匹配；本代理 target/policy 测试通过。                                                   | 不推定任意多重编码或运行时连接已经受控。                         |
| PA-A5 | policy 仅本地计算，拒绝 DNS/HTTP 夹具仍完成，只有 deny/requires_runtime_checks 与八类待检项；代码不存在目标 TCP 调用。                                                                | 夹具不声称拦截任意 TCP；真实 Runner/admission 后置。             |
| PA-A6 | access→global→project/letter→command 锁序与空 deny 稳定行；真实 row locks/advisory gate 控制变更和权限两种先后顺序。                                                                  | 完整 PG 运行来源为主会话，不以 sleep 或 mock 推定。              |
| PA-A7 | namespace 唯一、先权限/principal 再重放、旧 revision 原回执优先、业务/ledger/audit/outbox 一事务；三种 kind 回滚/隔离 PG 由主会话确认；原 ID GET/PUT 重启回执一致日志已核对。         | Windows race 与 Linux CI 未实跑。                                |
| PA-A8 | 项目祖先命令/搜索内存、全部 protected roots、Ajv 资源/身份/时刻校验；本代理定向 22 项 Vitest 通过；主会话最终 12 场景 E2E 包含真实分页/搜索/unknown/CIDR/手机键盘双项目往返，均通过。 | 敏感输入仅内存；刷新后须按原 ID 查询，不宣称自动恢复未保存表单。 |
| PA-A9 | 00003/00004 前向新增、旧历史保持、provision 同事务 scope 行、新旧共享 access；迁移/status 和重启日志已核对，最终完整后端/Web 门禁由主会话确认通过。                                   | 最终文本/diff 与交付文档汇总由主会话负责。                       |

## 本代理实际执行

2026-10-12 至 2026-10-13，backend 使用仓库锁定 Go 1.26.9，进程级 `GOTOOLCHAIN=local`：

```text
go test ./internal/target ./internal/authorization ./internal/policy
go vet ./internal/target ./internal/authorization ./internal/policy
go test ./internal/scopecommand ./internal/policy ./internal/httpapi -run 'TestNormalizationFieldDetailsAndPrivacy|TestEvaluateAuthorizationTimeBoundaries|TestScopeErrorsReplayAndPreview' -count=1
go test ./internal/httpapi ./internal/scopecommand -run 'TestScopeValidationDetailsAreControlled|TestNormalizationFieldDetailsAndPrivacy' -count=1
go vet ./internal/httpapi ./internal/scopecommand ./internal/validation
pnpm exec eslint src/app/app.tsx src/api/client.ts src/features/assets/use-asset-search-state.ts src/features/scope/use-scope-command.ts src/features/commands/command-workspace.tsx src/features/authorization/authorization-form.tsx
pnpm typecheck
pnpm exec vitest run src/features/scope/workspaces.test.tsx src/features/commands/command-workspace.test.tsx src/api/scope-client.test.ts
```

以上定向 Go 测试/vet、六个路径 ESLint、全项目 TypeScript 和三个 Vitest 文件共 22 项均通过。本代理没有加载 `.env`、输出连接串或认证信息，也没有重复启动服务、真实 PG、全套门禁或 E2E。最后两处 CSS 与手机回归修复后的完整 Web 门禁与 E2E 均由主会话实际执行确认通过，不自动沿用先前通过结果。

## 主会话运行证据核对

以下明确来源于主会话或其实施代理，不声称本代理独立执行：

| 检查                  | 已确认结果与证据                                                                                                                                                                                                                                                              |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 后端完整门禁          | 主会话确认格式/sqlc/mod/vet/unit/真实 PG 全通过，包含约束、迁移兼容、并发、回滚、幂等和旧草稿回归。                                                                                                                                                                           |
| Web 完整门禁          | 主会话确认最后两处 UI 修复后的官方完整门禁 exit 0：格式、生成漂移、lint、类型、8 文件 56 项 Vitest 与 build 均通过。                                                                                                                                                          |
| 00004 前向迁移/status | 已读取 `.local/validation/authorization-migration-00004-final.log`，00001–00004 均 applied。                                                                                                                                                                                  |
| 最终 API 重启         | 已读取 `.local/validation/restart-1bbc57f02efa4294ae8aad7ea0aef559.log`；主会话确认旧草稿和新领域 GET/PUT 回执逐字段一致，CLI 激活/撤销重放与查询一致。                                                                                                                       |
| 完整浏览器验收        | 已读取最终 `.local/validation/authorization-e2e-32152f0a097943868eabf2c1d01abeb4.log`，12 passed（32.9s），含手机键盘双项目往返、三列表分页及最后独立项目权限撤销。历史首轮未收集、第二轮 10 passed / 1 failed / 1 did not run 均保留事实，但已由实际修复和最终完整重跑闭环。 |
| 文本/sqlc/diff        | 主会话报告 SQL 多末尾换行已修，161 文件文本、sqlc 漂移与 diff 检查通过；最后UI修复和交付文档定稿后，主会话再次执行官方text161文件及git diff --check，均exit0。                                                                                                                |

主会话与本代理均已实际查看最终四张截图：`docs/validation/assets/p0-authorization-assets-desktop.png`、`p0-authorization-version-desktop.png`、`p0-authorization-precheck-desktop.png`、`p0-authorization-precheck-mobile.png`。复制按钮白字可见，手机项目选择与四导航可见，hash/UUID 正确换行，来源声明、总量未知、八类运行时待检、历史回执和“不是执行许可”说明清楚。失败轮截图曾用于发现问题；本次视觉结论以最终更新截图为依据。

本代理自有报告已运行 Prettier，并独立检查 UTF-8 无 BOM、LF、单个末尾换行与无尾随空白，均通过。完整任务最终文本/diff已由主会话在交付文档定稿后汇总通过。

## 规范同步与后续边界

已独立读取新增 backend/authorization-assets-contract.md 与 frontend/scope-workspace-contract.md 七段合同，并核对 backend/frontend index、database/error/tooling/directory、frontend state/type/hooks/quality 同步差异。内容与最终路径一致：新旧 access 锁、global/project/letter/command 顺序、namespace 与原子账本；非敏感路由/cursor、敏感搜索 location.key 祖先内存 snapshot、跨页面 unknown 和所有 protected roots 已登记。

用户选择 `satnaing/shadcn-admin` 作为正式前端底座，由后续独立整合任务落实。本切片仍按实际 Vite/React/shadcn 实现验收，不宣称已引入完整开源底座。底座整合必须保留已验证的项目/会话隔离、缓存清理、unknown 恢复、分页返回和受控字段错误合同。

Windows Go race、Linux 云 CI、真实企业 IdP 未实际运行，不作为本地通过项。预检查仍是静态子集；实际目标 DNS/HTTP/TCP、ToolCapability、admission、lease/fencing、执行预算、Runner/出口网关、Agent 与证据闭环属后续切片，不消费历史预检查作为执行许可，也不宣称完整平台或 ARTEX 性能对照已完成。
