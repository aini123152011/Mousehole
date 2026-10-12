# HTTP 错误与未知结果合同

## 1. 范围与触发

适用于 OpenAPI、httpapi、command 与 Web 的公开错误边界。权威接口为 [OpenAPI](../../../api/openapi.yaml)，映射见 [server.go](../../../backend/internal/httpapi/server.go)。

## 2. 签名

`NewHandler(cfg Config, commands Commands, logger *slog.Logger) (http.Handler, error)`。公开错误固定 `{code, message, request_id}`；Go 用 `errors.Is/As` 分类，不匹配中文消息。

## 3. 合同

PUT `/api/v1/projects/{project_id}/commands/{command_id}` 请求 `kind=create_task_draft` 与 `payload.title`。UUID 必须标准小写带连字符；标题 Unicode trim 后 1–120 字，禁止 NUL。拒绝未知、重复、缺失及大小写错误字段；正文最大 16 KiB。成功新受理 201，回放 200；GET 原回执 200。

业务请求需唯一 `X-Mousehole-Development: true`；它不是凭据。来源与项目校验在进入业务前完成。公开消息中文，不透传 DSN、SQL、完整参数或堆栈。请求 ID 由服务生成并同时写响应头。

## 4. 校验与错误矩阵

| HTTP    | code                               | 含义与前端处理                   |
| ------- | ---------------------------------- | -------------------------------- |
| 400     | invalid_request                    | 参数无效，明确拒绝               |
| 403     | project_forbidden                  | 固定身份无权访问项目             |
| 403     | development_only                   | 模式、来源或标记不符合边界       |
| 404     | command_not_found                  | 暂未查到，仍待确认，禁止自动重发 |
| 409     | idempotency_conflict               | 参数冲突，查询原命令             |
| 413     | request_too_large                  | 明确拒绝，限制 16 KiB            |
| 415     | unsupported_media_type             | 明确拒绝，要求 application/json  |
| 500     | internal_error                     | 结果可能未知，保留 ID 查询       |
| 503     | service_unavailable                | 依赖或提交不可确认，保留 ID 查询 |
| 404/405 | route_not_found/method_not_allowed | 路由或方法错误，统一公开结构     |

## 5. 好、基准与坏例

好例：Commit 失败包装底层原因，公开 503 并提示保留原 ID。基准：空标题得到 400 中文消息。坏例：把数据库错误字符串发给用户，或把 404/网络超时显示为“没有创建”。

## 6. 测试断言

HTTP 单测断言状态、code、中文消息、request_id/响应头、严格 JSON、来源/标记、项目、大小上限与媒体类型；真实集成断言业务错误不修改记录。Web 验证状态与 code 成对匹配，未校验响应统一视为未知。

## 7. 错误与正确

```go
// 错误：用用户可变文案判断业务分支。
// if err.Error() == "命令参数无效" { ... }
// 正确：稳定类别判断，公开消息由传输边界统一选择。
if errors.Is(err, command.ErrInvalidRequest) {
    // 返回 invalid_request 与中文说明。
}
```

代码示例说明分类方式；实际映射集中在 `domainError`，避免各 handler 各自维护状态语义。
