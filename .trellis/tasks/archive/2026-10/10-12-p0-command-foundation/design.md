# 首个命令闭环合同

## 数据流与边界

React 表单 → OpenAPI client → Go/chi 传输层 → command 服务 → pgx/sqlc 事务 → PostgreSQL。回执、draft task、业务审计和 outbox 同事务；不启动 dispatcher，不消费 outbox，不产生外部目标行为。数据库是权威，前端仅展示查询结果。

后端目录 backend/internal/{command,httpapi,store/sqlc}，入口 backend/cmd/api 与 backend/cmd/migrate。迁移由独立命令显式执行，不在 API 启动时偷偷修改 schema。编号 SQL 迁移锁定一种工具。前端按 features/commands 分层，基础 shadcn 控件集中 components/ui；先实现需要的 Button、Input、Card、Badge 等，记录来源。

## HTTP 合同

统一 /api/v1 前缀，JSON snake_case，OpenAPI 文件 api/openapi.yaml 为权威。所有业务请求必须携带 `X-Mousehole-Development: true`；该字段不是凭据，只是显式开发协议的一部分。

- GET /healthz：存活检查。
- GET /api/v1/session：返回 `{ mode: "development", actor_id: "00000000-0000-4000-8000-000000000001", project_id: "00000000-0000-4000-8000-000000000002", project_name: "本地演示项目" }`。
- PUT /api/v1/projects/{project_id}/commands/{command_id}：命令 ID 必须为标准小写 UUID。正文 `{ kind: "create_task_draft", payload: { title: "中文任务标题" } }`。标题 trim 后 1–120 个 Unicode 字符；规范化后的相同请求视为同参数，未知字段及不支持 kind 拒绝。
- GET 同一 commands/{command_id} 路径：按项目和开发身份读取回执，不存在返回 404 `command_not_found`；不把暂未查询到当成未受理证明。
- 新受理返回 201、同请求回放返回 200，正文都是 `{ command_id, project_id, kind: "create_task_draft", state: "accepted", task_id, task_title, task_state: "draft", version: "1", accepted_at }`。时间为 UTC RFC3339，version 十进制字符串。返回相同原记录，不重新生成时间、任务或审计。
- 错误 `{ code, message, request_id }`，英文 code、中文 message。400 invalid_request，403 project_forbidden/development_only，404 command_not_found，409 idempotency_conflict，503 service_unavailable，500 internal_error。可在实际契约补充请求过大/媒体类型等标准失败。

API 服务只允许显式 `MOUSEHOLE_MODE=development`；监听 `127.0.0.1:8080`（配置只能 loopback），应用 Origin 默认 `http://127.0.0.1:5173`。校验 Host/Origin，拒绝非配置来源，不开启通配 CORS；本地 CLI 无 Origin 也需开发标记。开发身份仅访问上述固定项目，禁止参数切换为其他项目。无 OIDC、生产 session 或生产权限完成声明。Go 优雅停机，request_id 与 slog 中文消息，不输出数据库连接串或完整参数。

## 存储与幂等

命令唯一键 `(project_id, command_id)`；actor 与命令类型/规范化参数绑定。服务先经过身份/项目检查，再事务内插入命令占位（或等价 PostgreSQL 唯一约束串行化），同键竞争等待提交后核对参数；不得依赖进程 mutex 保证唯一。draft task 使用 UUID，审计/outbox 对同命令唯一。注入事务后段失败时四类记录全回滚；未知提交结果按原 ID 查询。

pgx/sqlc 生成访问代码，生成器版本固定；业务规则不写入 handler。migration 与 DB 集成使用专用测试数据库/隔离 schema，不操作已有用户库。测试不得通过关闭关键约束来变绿。

## 页面与状态

采用 React/TS/Vite、shadcn/Tailwind、TanStack Query、RHF/Zod；此切片不安装尚未使用的图和表格库。中文页面包含本地模式说明、标题表单、命令 ID、受理/查询状态和草稿回执。界面必须可键盘使用、焦点明确、错误可读。提交待确认时保留 ID/参数并阻止第二次操作；提供“查询原命令”，不自动重发 PUT。GET 404 显示未确认，仍可再次查询。明确查询已有 ID 与创建新草稿的交互边界。

OpenAPI types 生成至 web/src/api/generated/，用 openapi-fetch 或等效生成 client 请求；未知返回值在统一边界检查。Query 管服务器结果，局部状态只管理输入与提交过程。不使用 localStorage 保存凭据或第二份服务器状态。

## 工具与升级

UTF-8 无 BOM、LF；Go gofmt/goimports；Web Prettier/ESLint/TS strict。锁定实际可用版本，记录 Windows 与 Linux 命令。数据库、Go 等工具若主机缺失，可在忽略目录准备可校验的本地工具，禁止隐式全局安装。CI 使用真实 PostgreSQL，独立检查生成漂移和关键浏览器路径。
