# 中文资产与范围工作台合同

## 1. 范围与触发

适用于 React Router 项目工作台、`features/scope`、OpenAPI/Ajv 客户端和全部受保护业务缓存。基础控件沿用自持 shadcn；机器字段与状态英文，业务文案、TSDoc、公开错误简体中文。实际结果见 [授权与资产验收](../../../docs/validation/p0-authorization-assets.md)。

## 2. 签名

[client.ts](../../../web/src/api/client.ts) 的边界签名：

```typescript
previewAssets(projectId: string, body: AssetImportPayload, signal: AbortSignal);
acceptScopeCommand(projectId: string, commandId: string, body: ScopeCommandRequest, signal: AbortSignal);
getScopeCommandReceipt(projectId: string, commandId: string, signal: AbortSignal, expected?: ScopeCommandRequest | null);
listAssets(projectId: string, filters: AssetFilters, signal: AbortSignal);
listAuthorizationVersions(projectId: string, filters: PageFilters, signal: AbortSignal);
getAuthorizationVersion(projectId: string, versionId: string, signal: AbortSignal);
getProjectPolicy(projectId: string, signal: AbortSignal);
listPrechecks(projectId: string, filters: PageFilters, signal: AbortSignal);
getPrecheck(projectId: string, decisionId: string, signal: AbortSignal);
```

以上为签名摘要，返回值均由 OpenAPI 生成类型及运行时校验约束；写请求使用内存会话 CSRF，credentials include。

旧 `useCommandWorkspace` 与新 `useScopeCommand` 都由 actor/project 项目路由祖先持有；草稿及范围未知命令在跨特性导航后继续保留 ID、绑定输入和锁。

`useScopeCommand(actorId,projectId)` 的 `submit(body)`、`lookup(commandId)`、`reset()` 跨三个业务页面共享交互；回执只存在 Query。Query roots 为 commands/assets/authorization/prechecks/scope-commands，key 前三项始终 root/actorId/projectId，再附 filter/cursor/object/command。

`clearProtectedCache(client,actorId?,projectId?)` 同步发起 cancelQueries，立即 removeQueries，随后 await 取消完成；指定范围只清该 actor/project，未指定清全部保护数据。路由项目树以 actor/project 身份重挂载，清输入、selection、unknown 与通知。

## 3. 请求、响应与状态合同

请求仅使用生成的 ScopeCommandRequest 判别联合，不复制手写 DTO；表单 RHF/Zod 提示不替代服务端规范化/权限。已校验 invalid_request 的 validation_errors 按固定字段映射展示并关联输入；CIDR host bits 的规范建议只用于提示，不自动替换并扩大范围。API 边界由 Ajv2020 验证 schema，再核对 project/command/kind/result.type 和嵌套对象所属项目；version 的 entry_ids 与 entries 同长度，时间 Date.parse 有限，整数字符串不转 number。

提交前同步 ref 锁定 crypto.randomUUID 与 structuredClone 的参数，然后发单次 PUT，显式 retry:false。sending/unknown 不能新建命令或改变绑定参数；unknown 可按当前 ID 查询。查询 404 只表示暂未确认，不自动 PUT，不自动生成新 ID；已确认回执在更新失败后保留，明确为历史。迟到/取消响应不能写回新身份或新项目。

| phase    | 中文含义            | 交互                       |
| -------- | ------------------- | -------------------------- |
| idle     | 可编辑              | 可提交。                   |
| sending  | 提交中，ID/参数锁定 | 不再写入。                 |
| unknown  | 受理待确认          | 原 ID 查询。               |
| accepted | 控制面原命令已受理  | 显式结束后编辑下一条。     |
| rejected | 已校验明确拒绝      | 保留输入，显式结束后修改。 |

手输旧 ID 为 lookup，不带自动写入，可显式结束。刷新后只能人工输入保留的 ID；目标、回执、token/CSRF 都不进入 localStorage/sessionStorage。切换路由不能绕开 unknown 锁；切换项目/会话则清理原敏感状态。

页面清楚区分加载、空、错误、上次数据/stale、unknown。资产来源为声明；授权显示候选、当前、替代、撤销、尚未生效/过期与正文 hash；保存后只读，修改复制新版本。管理员动作提供受控 CLI 说明，不给 analyst 假激活按钮。

预检查仅显示“静态规则拒绝”或“静态规则匹配，执行条件尚未验证”，包含原 checked_at、版本/hash、revision、原因/条目与运行时缺项。历史结果不能显示为当前允许执行或验证完成。

服务端分页用 page_count/has_more/next_cursor，明确总量未知；默认 25 条。类型和 cursor 可进 URL，目标搜索按 React Router 的 location.key 存于项目祖先内存映射；筛选改变回首屏，详情与浏览器返回恢复原位置对应的搜索，不能拿新搜索配旧 cursor。actor/project 改变即清理，不把搜索写入 history.state。cursor 的筛选绑定不能含目标原文。

401 清全部保护缓存和会话内存，重新读取 session；403 forbidden/project_forbidden 清当前边界并刷新身份。会话/项目改变时取消请求，再清缓存，不能仅清旧 commands root。输入作为纯文本，禁止目标 HTML/URL 自动加载或执行。

## 4. 校验与错误矩阵

| 条件                                                          | 行为                                         |
| ------------------------------------------------------------- | -------------------------------------------- |
| schema 正确且对象绑定吻合                                     | 展示已验证服务器数据。                       |
| kind/result、project/command/嵌套对象错配，非法 UTC/数量/分页 | unconfirmed_response；写入结果保持 unknown。 |
| 已知 HTTP/code 成对明确拒绝                                   | rejected，保留编辑输入。                     |
| 5xx、断线、超时、未知码/响应                                  | unknown，保留原 ID/参数。                    |
| GET 404                                                       | 待确认，不推断未发生。                       |
| 401/项目拒绝                                                  | 清相应全部保护缓存与输入边界。               |
| 已确认后更新失败                                              | 保留上次回执并标明未更新。                   |

API 新字段、枚举与生成器输入同步变更，check:generated 比较完整文件集合与字节；不能先覆写生成目录再判漂移。

## 5. 好、基准与坏例

好例：预检查响应被丢弃，界面锁定原 ID；原 ID GET 恢复同 checked_at 的历史决策，用户用新命令明确重检当前规则。基准：键盘完成导入预览、候选保存、预检查与回执查询。坏例：切到另一页或自动重试生成第二条命令，或者保留旧身份 Query 让新用户看见。

## 6. 必需测试及断言

client 覆盖 schema、request object/嵌套对象、kind/result 和时间/字符串精度/分页一致；Hook 覆盖同 tick 只发一次、unknown 路由锁、404/5xx/迟到响应、历史保留、跨 actor/project 清理。组件围绕 label、错误关联、状态、输入保留与键盘可观察行为测试。

真实 Playwright 使用合成 OIDC、真实 Go/PG 与大于一页数据，覆盖列表翻查/筛选/返回、只读候选/复制、静态解释/未知恢复、主项目及独立成员撤销项目、窄屏无横溢。登录先等待稳定登录或工作台状态；trace 禁止收集 Cookie/CSRF，测试错误与日志脱敏。截图仅合成数据，无认证秘密；mock 通过不能替代此验收。

中文 TSDoc 说明原 ID/参数所有权、取消/迟到响应与历史有效性，不为每行语法写重复注释。shadcn 来源与许可证保留，业务无障碍仍需真实键盘/焦点和窄屏检查。

## 7. 错误与正确

错误：退出只 removeQueries(['commands'])；查询失败显示“未创建”；静态匹配显示绿色“允许执行”。

正确：所有 protected roots 按 actor/project 取消并清理；结果未确认保留原 ID 查询；静态匹配明确展示执行条件尚未验证，并保留原规则时刻。
