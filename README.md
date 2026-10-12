# Mousehole

面向单企业私有化的自主安全评估平台。本仓库正在落实基于 ARTEX 源码审查形成的重建方案：Go 控制面、PostgreSQL 业务权威、React 中文工作台，以及后续的 Temporal 编排与隔离 Runner。

当前交付的是第一个可运行切片：**创建任务草稿 → 事务受理 → 查询原回执**。包含真实数据库并发与回滚验证、响应丢失后的查询恢复、中文注释和统一检查工具。页面显示“已受理”时，草稿已经持久化，尚未执行目标操作。

完整路线和取舍见 [设计方案](.trellis/tasks/10-11-artex-rebuild-plan/design.md)、[统一中文代码规范](.trellis/tasks/10-11-artex-rebuild-plan/coding-standards.md) 和 [注释与文档模板](.trellis/tasks/10-11-artex-rebuild-plan/coding-templates.md)。这些是总体规划；已有实现以本 README、源码及 [本切片验收记录](docs/validation/p0-command-foundation.md) 为准。

## 开发环境

| 依赖       | 锁定版本            | 说明                                             |
| ---------- | ------------------- | ------------------------------------------------ |
| Go         | 1.26.9              | `backend/go.mod` 与检查入口核对实际工具链。      |
| Node.js    | 24.19.0             | 检查入口精确核对版本。                           |
| pnpm       | 12.4.2              | 前端唯一包管理器，提交 lockfile。                |
| PostgreSQL | 17，固定镜像 digest | Compose 配置只绑定 `127.0.0.1:55432`。           |
| Python     | 3.11+               | 根脚本只使用标准库；本地使用 3.14。              |
| Docker     | 支持 Compose v2     | 仅本地数据库使用 Docker，API 和 Web 在主机运行。 |

生成器精确版本见 [toolchain.json](scripts/toolchain.json)。工具准备在当前终端或工作区内进行，项目脚本不安装全局工具、不修改全局 PATH。可将校验过的 Go 解压至 `.tools/go1.26.9/go/`，脚本会优先使用该目录；Linux CI 直接使用精确版本 Go。

Windows 官方归档：[go1.26.9.windows-amd64.zip](https://dl.google.com/go/go1.26.9.windows-amd64.zip)，SHA-256：`d722201a9c0c086d1610e111c48203009af690892ed708072bd5ae20160e7a59`。其他平台从 [Go 官方下载页](https://go.dev/dl/) 获取对应版本并核对官方校验和。

## 启动

在仓库根目录先启动本工程数据库，再安装前端锁定依赖：

```powershell
python scripts/dev.py database
pnpm --dir web install --frozen-lockfile
```

`database` 会等待 PostgreSQL 健康，并在不存在时创建固定的 `mousehole_test` 测试库。已有卷和数据保留；不会连接 ARTEX 数据库。公开开发占位密码见 [Compose](deploy/compose.yaml)。

在 API 终端显式配置环境并迁移，然后启动服务：

```powershell
$env:MOUSEHOLE_MODE = 'development'
$env:MOUSEHOLE_DATABASE_URL = 'postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole?sslmode=disable'
python scripts/dev.py migrate
python scripts/dev.py api
```

另一终端运行前端：

```powershell
python scripts/dev.py web
```

打开 [中文工作台](http://127.0.0.1:5173/)。API 默认在 `127.0.0.1:8080`，Vite 代理 `/api`。固定本地身份和项目只是开发适配器；所有业务请求仍检查开发标记、Host、Origin、身份和项目范围。[.env.example](.env.example) 只展示配置，脚本不自动读取它。

Linux/macOS 使用相同 Python 入口，将 PowerShell 环境赋值替换为：

```bash
export MOUSEHOLE_MODE=development
export MOUSEHOLE_DATABASE_URL='postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole?sslmode=disable'
python scripts/dev.py migrate
python scripts/dev.py api
```

API 启动不执行迁移；查看迁移用 `python scripts/dev.py migration-status`。Ctrl+C 停止 API 或 Web；数据库可用 `docker compose -f deploy/compose.yaml stop` 停止并保留数据。`/healthz` 只代表进程存活。

## 验证

在配置了专用数据库的终端运行静态、格式、生成漂移、单元及真实数据库检查：

```powershell
$env:MOUSEHOLE_TEST_DATABASE_URL = 'postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole_test?sslmode=disable'
python scripts/check.py --integration
python scripts/verify_restart.py
```

`verify_restart.py` 要求同终端设置 `MOUSEHOLE_MODE`、`MOUSEHOLE_DATABASE_URL` 且已迁移。它在随机 loopback 端口启动独立 API，创建一条合成草稿、终止进程、重新启动，并逐字段核对原回执；保留该条验收数据。集成测试则使用随机 schema，只清理自己创建的 schema。

保持开发 API 已启动，安装浏览器并运行端到端检查；Playwright 自动启动 Vite：

```powershell
pnpm --dir web exec playwright install chromium
pnpm --dir web e2e
```

`python scripts/check.py --integration --e2e` 合并上述格式与行为检查，但不替代独立重启检查。`--scope backend|web|text` 可缩小范围；`--race` 增加 Go 竞态检测。Linux CI 执行竞态检查；Windows 需要可用的 C 编译工具链。缺数据库、浏览器或指定工具时检查失败，不将跳过当成通过。CI 配置见 [check.yml](.github/workflows/check.yml)，配置已落盘与云端实际运行成功是两个不同状态。

前端统一格式化：`pnpm --dir web format`；根文档使用同一 Prettier 配置。Go 和 sqlc 生成命令见 [后端说明](backend/README.md)，前端状态、来源与测试见 [前端说明](web/README.md)。生成目录只允许通过生成器更新。

默认 Go 模块代理为官方配置。本地网络若无法访问，可只在当前终端设置 `$env:GOPROXY = 'https://goproxy.cn,direct'`（Linux 使用 `export GOPROXY=...`），仍保留 Go 的校验和校验，不修改全局设置。

## 代码组织与后续工作

| 路径                                        | 责任                                 |
| ------------------------------------------- | ------------------------------------ |
| `api/openapi.yaml`                          | HTTP 请求、回执和错误契约。          |
| `backend/cmd`                               | API 启动和显式迁移。                 |
| `backend/internal/command`                  | 规范化、访问边界与事务幂等服务。     |
| `backend/internal/httpapi`                  | 严格解码、HTTP 边界和中文错误映射。  |
| `backend/db`、`backend/internal/store/sqlc` | 编号迁移、参数化查询和生成访问代码。 |
| `web/src/features/commands`                 | 中文表单、查询恢复和回执展示。       |
| `web/src/components/ui`                     | 自持 shadcn 基础控件及来源记录。     |
| `scripts`、`deploy`、`.github`              | 本地入口、数据库依赖及 CI。          |

当前没有生产 OIDC、Temporal、Runner、模型调用、SSE 或目标探测能力。后续沿已批准路线建设真实授权、持久化编排、隔离执行、证据和发现闭环；首版自主能力上限仍为主动枚举与非破坏性验证。具体代码约定见 [后端 spec](.trellis/spec/backend/index.md) 与 [前端 spec](.trellis/spec/frontend/index.md)。
