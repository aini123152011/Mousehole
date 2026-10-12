# PostgreSQL 事务与迁移合同

## 1. 范围与触发

适用于身份预置、服务端会话、成员授权、草稿命令的受理与查询、表结构及 sqlc 生成。四类命令记录是当前原子边界，outbox 尚无消费者。依据 [命令服务](../../../backend/internal/command/service.go)、[身份存储](../../../backend/internal/identity/store.go)、[身份迁移](../../../backend/db/migrations/00002_identity_access.sql) 和数据库查询。

## 2. 签名

```go
func (s *Service) Accept(ctx context.Context, actorID, projectID, commandID string, request Request) (Receipt, bool, error)
func (s *Service) Get(ctx context.Context, actorID, projectID, commandID string) (Receipt, error)
```

`commands` 主键 `(project_id, command_id)`；`tasks` 同命令唯一；`audit_events`、`outbox_events` 各以同命令为主键。`commands.task_id` 到 `tasks.task_id` 为提交时检查的延迟外键。

## 3. 合同

`Accept` 先验证 ID 和参数，再开启 `READ COMMITTED` 事务。事务先以 `FOR SHARE` 锁定活动 user/project/membership 并检查 `task_draft:create`，随后才执行命令幂等逻辑。成员撤销更新同一 membership 行并取得排他锁：撤销先提交则命令拒绝，命令先提交则四类记录完整保留。`Get` 同样在事务内重验成员并要求命令 actor 等于当前 actor。

`INSERT ... ON CONFLICT DO NOTHING` 通过唯一键等待竞争者；下一条查询读取已提交记录，核对 actor、kind、规范化标题。同参数返回原回执及 `created=false`，新记录同事务写命令、草稿、审计、outbox，提交成功后返回 `created=true`。bootstrap 在一个事务内幂等创建或核对 user、external identity、project、membership 并写管理审计；异参数或唯一绑定冲突不得覆盖原记录。

管理员撤销必须先以 `FOR UPDATE` 锁定稳定用户或成员行，再在同一事务更新状态、撤销会话并写 `access_audit_events`。用户、成员、命令和历史审计都不物理删除；成员撤销递增 version。重复执行在目标已处于所需状态时成功返回，不重复写管理审计；目标从未存在与幂等重放必须使用不同错误。

版本存 BIGINT，API 返回十进制字符串；时间存 TIMESTAMPTZ，API 返回 UTC RFC3339。SQL 参数化，业务字段英文 snake_case，SQL 说明与 COMMENT 中文。sqlc 目录只通过固定版本生成器更新。

迁移使用 goose 编号文件；API 启动不自动迁移。`00002` 为旧固定 actor/project 建立 disabled/revoked 历史记录，再增加外键，不能改写既有命令归属。已发布编号不得覆盖。`cmd/migrate` 仅暴露 up/status。Down 会删除本切片数据，不能恢复已删除数据；优先备份与前向修复。测试要求 `MOUSEHOLE_TEST_DATABASE_URL`，创建随机 schema，只清理自身 schema。

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

真实 PG 集成断言旧命令迁移保留、bootstrap 幂等/冲突、摘要会话与登录尝试一次性消费、管理员全局撤销、用户禁用、成员撤销锁顺序、管理审计幂等和审计失败整体回滚，以及跨 actor 拒绝、并发唯一、规范化参数回放、冲突不变、关联一致、提交后段失败全回滚和连接池重建恢复。`verify_restart.py` 通过合成 OIDC 登录取得 Cookie/CSRF，并补全 API 进程重启后逐字段一致。生成检查在临时副本生成并比较文件集合与字节，缺 DSN 必须失败。

## 7. 错误与正确

错误：进程 mutex 防重、Commit 报错就换 ID 重发、删除库来让测试变绿。正确：数据库唯一键最终串行化；Commit 报错保留原 ID 查询；随机 schema 保留其他数据。
