# PostgreSQL 事务与迁移合同

## 1. 范围与触发

适用于草稿命令的受理、查询、表结构与 sqlc 生成。四类记录是当前原子边界，outbox 尚无消费者。依据 [服务](../../../backend/internal/command/service.go)、[迁移](../../../backend/db/migrations/00001_command_foundation.sql) 和 [查询](../../../backend/db/queries/commands.sql)。

## 2. 签名

```go
func (s *Service) Accept(ctx context.Context, actorID, projectID, commandID string, request Request) (Receipt, bool, error)
func (s *Service) Get(ctx context.Context, actorID, projectID, commandID string) (Receipt, error)
```

`commands` 主键 `(project_id, command_id)`；`tasks` 同命令唯一；`audit_events`、`outbox_events` 各以同命令为主键。`commands.task_id` 到 `tasks.task_id` 为提交时检查的延迟外键。

## 3. 合同

`Accept` 先验证身份、项目和参数，再开启 `READ COMMITTED` 事务。`INSERT ... ON CONFLICT DO NOTHING` 通过唯一键等待竞争者；下一条查询读取已提交记录，核对 actor、kind、规范化标题。同参数返回原回执及 `created=false`，新记录同事务写命令、草稿、审计、outbox，提交成功后返回 `created=true`。

版本存 BIGINT，API 返回十进制字符串；时间存 TIMESTAMPTZ，API 返回 UTC RFC3339。SQL 参数化，业务字段英文 snake_case，SQL 说明与 COMMENT 中文。sqlc 目录只通过固定版本生成器更新。

迁移使用 goose 编号文件；API 启动不自动迁移。已发布编号不得覆盖。`cmd/migrate` 仅暴露 up/status。Down 会删除本切片数据，不能恢复已删除数据；优先备份与前向修复。测试要求 `MOUSEHOLE_TEST_DATABASE_URL`，创建随机 schema，只清理自身 schema。

## 4. 校验与错误矩阵

| 情况                            | 结果                                       |
| ------------------------------- | ------------------------------------------ |
| 相同键、相同身份与参数          | 原回执，四类记录各一份                     |
| 相同键、不同参数                | `ErrConflict`，已有记录不变                |
| 查询不存在                      | `ErrNotFound`，不推断未受理                |
| Begin/插入/查询/Commit 依赖错误 | 包装 `ErrUnavailable`，保留内部原因        |
| 后段延迟约束失败                | 整体回滚，四类记录为零                     |
| 请求取消                        | 传播 context；资源清理使用有上限独立上下文 |

## 5. 好、基准与坏例

好例：24 个同键并发请求都获得同一 task_id 与 accepted_at，仅一个 `created=true`。基准：单次正常受理后 GET 原回执。坏例：先查不存在再无约束插入，或在事务内调用模型、目标、Runner。

## 6. 测试断言

真实 PG 集成断言并发唯一、规范化参数回放、冲突不变、关联一致、提交后段失败全回滚、连接池重建恢复。`verify_restart.py` 补全进程重启后逐字段一致。生成检查在临时副本生成并比较文件集合与字节，缺 DSN 必须失败。

## 7. 错误与正确

错误：进程 mutex 防重、Commit 报错就换 ID 重发、删除库来让测试变绿。正确：数据库唯一键最终串行化；Commit 报错保留原 ID 查询；随机 schema 保留其他数据。
