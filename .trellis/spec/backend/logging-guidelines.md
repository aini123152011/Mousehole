# 中文结构化日志与业务审计

Go 统一使用 slog，消息简体中文，event、code 和字段名英文。入口负责 logger 生命周期，HTTP 边界记录安全事实，存储层返回包装错误，不每层重复输出同一异常。

| 等级  | 当前场景                      | 安全字段                                           |
| ----- | ----------------------------- | -------------------------------------------------- |
| INFO  | 启动、停机、草稿受理          | event、request_id、project_id、command_id、created |
| WARN  | 响应传输中断                  | event=response_write_failed、request_id            |
| ERROR | 请求处理异常/未确认、启动失败 | event、request_id、稳定 code                       |

```go
s.logger.InfoContext(r.Context(), "任务草稿命令已受理",
    "event", "command_accepted", "request_id", requestID,
    "project_id", receipt.ProjectID, "command_id", receipt.CommandID, "created", created)
```

示例说明字段，实际实现见 [server.go](../../../backend/internal/httpapi/server.go)。禁止记录完整 DSN、标题、请求正文、令牌、Cookie、SQL 异常细节、目标证据或模型 prompt。开发模式也遵守此边界。调试需要具体证据才增加脱敏字段，不临时开启全部正文。

`audit_events` 属于业务账本，与命令和草稿同事务；INFO 不能代替它。响应写出失败不撤销已提交事务，日志与 UI 都要求按原 ID 查询。测试替身 logger 只写测试拥有的缓冲区，不扩大生产日志内容。
