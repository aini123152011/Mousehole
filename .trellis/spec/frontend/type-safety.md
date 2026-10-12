# TypeScript 与 HTTP 运行时合同

## 1. 范围与触发

适用于 API 客户端、OpenAPI 生成、表单与任何边界输入。依据 [client.ts](../../../web/src/api/client.ts)、[生成器](../../../web/scripts/generate-api.mjs)、[schema](../../../api/openapi.yaml)。

## 2. 签名

`getSession(signal)`、`acceptCommand(projectId, commandId, body)`、`getCommandReceipt(projectId, commandId, signal, expectedTitle?)` 返回生成的类型；`logout()` 撤销当前 Cookie 会话，`setSessionMemory(session)` 只维护写请求所需的内存 CSRF。`CommandAPIError` 保存已校验 status、code、requestId。

## 3. 合同

TS 开启 strict、noUncheckedIndexedAccess、exactOptionalPropertyTypes、noUnusedLocals/Parameters。HTTP 字段保留生成 snake_case；手写变量 camelCase。openapi-fetch 提供请求类型，Ajv2020 使用生成 JSON Schema 校验未知响应，二者都需要。受保护请求显式使用 `credentials: include`，写请求从内存 session 取得 `X-CSRF-Token`，不得从 Web Storage 或 URL 恢复。表单 RHF/Zod 校验输入，标题 Unicode trim/count 规则与 Go 一致，不依赖浏览器 maxlength 的 UTF-16 计数。

回执包含 command_id/project_id/kind/state/task_id/task_title/task_state/version/accepted_at。固定 accepted/draft/版本字符串与 UTC 时间必须校验；accepted_at 同时满足 date-time 和 Z 后缀约束，校验后还须确认 Date.parse 结果有限，防止闰秒等浏览器不可表示时间导致展示崩溃。项目和命令必须匹配请求；本页提交还核对原标题。客户端不把 version 转 number。生成器头与许可证可英文，手写说明中文。

## 4. 校验与错误矩阵

| 输入                                   | 行为                          |
| -------------------------------------- | ----------------------------- |
| schema 合法且请求对象匹配              | 返回生成类型                  |
| 缺字段、非法状态/版本/时间或对象不匹配 | unconfirmed_response，保留 ID |
| 合法错误且 status/code 配对为明确拒绝  | rejected                      |
| 未校验错误、未知码、5xx、超时          | unknown                       |
| 标题空/超长、UUID 非标准               | 表单拒绝，服务端仍验证        |

## 5. 好、基准与坏例

好例：先 `validateReceipt(value)` 再读取字段，并匹配请求 ID。基准：合法合成回执。坏例：`await response.json() as CommandReceipt`、any、非空断言或双重断言。

## 6. 测试断言

client 测试覆盖 schema、对象匹配、时间/状态/版本、错误码配对、超时/未知与标题绑定；UTC 偏移与浏览器无法表示的闰秒必须返回 unconfirmed_response。生成漂移比较完整文件集合和内容，额外文件或输出目录缺失均失败；检查前后原文件字节不变，不重新建立缺失目录。生成成功但未比较不能算通过。表单测试覆盖 Unicode 与 UUID。

## 7. 错误与正确

错误：断言未知 JSON 已符合类型，未知错误解锁重发。正确：未知值经 Ajv 校验成为生成类型；只有稳定码与 HTTP 状态共同匹配才属于明确拒绝。
