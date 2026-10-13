# 命令 Hook 与异步边界

## 1. 范围与触发

适用于 `useCommandWorkspace(actorId, projectId)` 的创建、原命令查询与新草稿交互。依据 [实现](../../../web/src/features/commands/use-command-workspace.ts)。

## 2. 签名

`createDraft(title: string)`、`lookup(commandId: string)`、`startNewDraft()`；返回 selection、phase、notice、receipt、isQuerying 及交互允许条件。Query key 为 `['commands', actorId, projectId, commandId]`，不得让不同登录身份共享同项目、同命令的回执缓存。

## 3. 合同

PUT 前生成 crypto.randomUUID，规范化标题并同步 ref 锁定 ID/参数，然后更新 React 展示状态；防止同一渲染前重复事件发送第二个 PUT。mutation/query 显式 retry:false。查询传播 AbortSignal 并设置 15 秒预算。查询代次 `generation` 使迟到响应不改变当前选择的 phase/notice；旧身份、项目或命令结果只能进入自己的 Query key。组件以 `actorId:projectId` 为 key，身份或项目切换会重置局部未知结果状态。

同项目已提交但未知时，不能换 ID 查询来绕过锁，不能新建草稿；只查询原 ID。Hook 挂在项目路由祖先，页面导航不会销毁 ID/绑定标题；重返表单恢复原标题，原 ID 查询核对原标题。新范围 Hook 同样跨三个领域页面持有原参数。手输恢复属于 lookup，只查询不产生写入。Hook 中文 TSDoc 解释所有权、同步锁与未知语义，不为返回异步状态写虚假的同步 @throws。详见 [范围工作台合同](scope-workspace-contract.md)。

## 4. 校验与错误矩阵

| 场景              | 行为                            |
| ----------------- | ------------------------------- |
| 首次创建          | 锁定 ID/参数，sending，单次 PUT |
| 正常回执          | Query 缓存原回执，accepted      |
| 网络/5xx/未知响应 | unknown，保留原选择             |
| GET 404           | 仍待确认，不解锁未知提交        |
| 旧选择响应迟到    | 不覆盖当前交互状态              |
| 已确认后查询失败  | 保留上次回执，显示更新失败      |

## 5. 好、基准与坏例

好例：响应丢失后只 GET 同一 ID。基准：受理后用户显式选择新草稿。坏例：依赖 setState 完成后才防双击；effect 自动重发 PUT；迟到请求更新当前 phase。

## 6. 测试断言

同一 tick 连续 createDraft 只发一次；未知时查询别的 ID 被阻止；404、5xx、断线保持原 ID；明确拒绝可开始新草稿；迟到查询不覆盖新选择；已有确认回执在查询失败后仍保留。

## 7. 错误与正确

错误：mutation `retry:3`，catch 新建 ID；正确：`retry:false`，提交前保存 ID，catch 标记 unknown，人工 lookup 原 ID。
