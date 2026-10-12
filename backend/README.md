# 本地草稿命令后端

本模块实现 Go/chi → 命令服务 → pgx/sqlc → PostgreSQL 的最小闭环。命令回执、任务草稿、业务审计和 outbox 同事务提交；本切片不消费 outbox，没有 Runner、扫描、模型或工作流执行能力。`accepted` 表示草稿已持久化，不能解释为目标操作已执行。

## 运行依赖与配置

Go 工具链固定为 1.26.9（`go.mod` 指定最低 1.26.0 与 toolchain），PostgreSQL 使用 17 分支；开发数据库版本及镜像 digest 见根部署配置。依赖在 `go.mod`/`go.sum` 固定，禁止省略校验和验证。sqlc 固定为 1.31.1，goimports 固定为 0.52.0；生成工具不属于 API 运行依赖。

| 环境变量 | 用途与默认值 |
| --- | --- |
| `MOUSEHOLE_MODE` | 无默认值，API 和迁移必须显式设为 `development`。 |
| `MOUSEHOLE_DATABASE_URL` | API/迁移数据库 DSN，无默认值；仅使用专用本地开发库，配置秘密不写日志。 |
| `MOUSEHOLE_LISTEN_ADDR` | 默认 `127.0.0.1:8080`，只接受 loopback IP 与有效端口。 |
| `MOUSEHOLE_WEB_ORIGIN` | 默认 `http://127.0.0.1:5173`，只接受一个明确 HTTP loopback IP 来源，不含路径、查询或凭据。 |
| `MOUSEHOLE_TEST_DATABASE_URL` | 必需集成测试专用 DSN，无默认值；测试创建随机 schema 并只清理其自身 schema。 |

数据库用户在测试库需要建 schema 权限；日常 API 使用者不应获得生产 DDL 权限。本样本不是企业 OIDC，也不提供生产会话或生产权限保证。业务请求须带 `X-Mousehole-Development: true`，该字段只是显式开发协议，**不是凭据**。固定身份仅可访问固定演示项目；配置、Host 和 Origin 都经过默认拒绝校验，不信任转发头。Vite 代理必须以 `changeOrigin: true` 转发 API Host。

在 `backend/` 执行命令。PowerShell 配置示例中的 DSN 是专用开发库占位值，必须按根 `.env.example` 与部署配置替换：

```powershell
$env:MOUSEHOLE_MODE = 'development'
$env:MOUSEHOLE_DATABASE_URL = 'postgres://<开发用户>:<开发密码>@127.0.0.1:55432/<专用开发库>?sslmode=disable'
go run ./cmd/migrate up
go run ./cmd/api
```

Linux/macOS 使用同名环境变量：

```bash
export MOUSEHOLE_MODE=development
export MOUSEHOLE_DATABASE_URL='postgres://<开发用户>:<开发密码>@127.0.0.1:55432/<专用开发库>?sslmode=disable'
go run ./cmd/migrate up
go run ./cmd/api
```

`go run ./cmd/migrate status` 显式读取迁移状态。API 只连接数据库，**不在启动时执行迁移**。`GET /healthz` 仅报告进程存活，不表示数据库或 schema 可用。启动配置错误和无法连接数据库都明确失败，错误不输出完整 DSN。Ctrl+C/SIGTERM 给 HTTP 请求最多 10 秒宽限期，然后关闭服务；客户端未收到回执时仍须按原 ID 查询。

## 契约与代码结构

[OpenAPI](../api/openapi.yaml) 是 HTTP 请求、响应、状态与错误的权威描述。前端由此生成类型，服务端的传输验证与契约行为通过测试验证；这里未使用生成的 Go handler，不应误称所有 Go 类型均自动生成。

| 路径 | 职责 |
| --- | --- |
| `cmd/api` | 配置、连接池、HTTP 入口与优雅停机。 |
| `cmd/migrate` | 显式 `up`/`status` 迁移入口，不提供自动删除数据命令。 |
| `internal/httpapi` | 严格 JSON、请求 ID、来源/身份/项目边界和公开错误映射。 |
| `internal/command` | 参数规范化、幂等受理、事务协调及原回执查询。 |
| `db/migrations` | goose 编号 SQL 迁移与嵌入资源。 |
| `db/queries` | 参数化 sqlc 查询输入及中文并发说明。 |
| `internal/store/sqlc` | sqlc 生成访问代码，不手工修改。 |

首尾 Unicode 空白从标题剔除，长度按 Unicode 字符计数为 1–120；正文最大 16 KiB，拒绝未知、缺失、重复和大小写错误的字段。命令 ID 必须是带连字符的标准小写 UUID。数据库唯一键为 `(project_id, command_id)`，身份、命令类型、规范化标题共同绑定该键。不同参数返回 `409 idempotency_conflict`；同参数返回原始时间、草稿 ID 和回执，不新增事件。

PostgreSQL `READ COMMITTED` 下，`INSERT ... ON CONFLICT DO NOTHING` 等待同键事务结束，下一条查询读取获胜记录。该保证不依赖进程锁、缓存或单实例。受理记录与草稿有延迟外键，提交前草稿必须存在；审计和 outbox 也必须成功写入才能提交。事务内没有外部调用。

提交错误可能表示结果未知，对外使用 `service_unavailable` 并要求查询原命令；不能因网络错误或查询暂未找到而生成新 ID 重发。回执版本使用十进制字符串，受理时间使用 UTC RFC3339。任何查询都会重验身份和项目。运行日志不记录标题、完整请求、凭据或内部 SQL 异常，业务审计与草稿保持同事务。

## 迁移与数据回滚边界

使用 goose 3.28.0 的 Provider 和编号文件 `00001_command_foundation.sql`。首次建表迁移在事务中执行；重复 `up` 不重复执行，后续版本必须新增编号文件，不能更改已发布文件。开发数据库应独立于测试数据库。

SQL 内保留 `Down` 以明确建表依赖的反向顺序，但其含义是**永久删除本切片数据**，不恢复用户数据。日常命令不暴露 down；实际数据问题优先备份、核对回执并前向修复。禁止在现有用户库运行测试或通过关闭唯一/外键约束使测试通过。

## 生成与验证

所有以下命令均从 `backend/` 执行，工具版本使用明确版本参数：

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go run golang.org/x/tools/cmd/goimports@v0.52.0 -w ./cmd ./internal ./db
go vet ./...
go test ./...
go test -tags=integration ./...
```

仅单元测试不需要数据库。集成测试带独立 `integration` 构建标签；缺少 `MOUSEHOLE_TEST_DATABASE_URL` 或 PostgreSQL 时必须失败，不能跳过或报告为通过。Linux CI 在支持环境额外运行：

```bash
go test -race ./...
go test -race -tags=integration ./...
```

Windows 的 `-race` 需要支持的 C 编译工具链，环境缺失时如实报告，不关闭并发测试。生成漂移验证需比较生成前后文件内容，不能仅判断生成命令成功；根验证脚本负责此项。更改 OpenAPI 后还需运行前端生成检查。

真实 PostgreSQL 集成用例证明：24 个并发请求只产生一份四类记录，参数改变返回冲突，四类记录关联一致，延迟约束在提交时失败会全部回滚，关闭旧连接池后重新创建服务仍能恢复原回执。该重新加载验证持久化与无进程缓存，全进程重启及浏览器端到端路径由全栈验收补充。HTTP 单测验证中文参数、严格字段、项目拒绝、开发标记、Host/Origin、媒体类型、请求上限和公开错误语义。
