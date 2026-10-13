# Mousehole

面向单企业私有化的自主安全评估平台。本仓库正在落实基于 ARTEX 源码审查形成的重建方案：Go 控制面、PostgreSQL 业务权威、React 中文工作台，以及后续的 Temporal 编排与隔离 Runner。

当前分支已实现 **企业 OIDC 登录 → 服务端会话 → 项目成员权限 → 草稿受理与原回执查询**，以及规范化资产、不可变授权版本和静态策略预检查。浏览器不保存 OIDC token，只持有 HttpOnly 不透明 Cookie；写权限在命令事务内复核用户、项目与成员状态。页面显示“已受理”表示控制面记录已经持久化；预检查只解释静态规则与尚待验证的执行条件。

完整路线和取舍见 [设计方案](.trellis/tasks/10-11-artex-rebuild-plan/design.md)、[统一中文代码规范](.trellis/tasks/10-11-artex-rebuild-plan/coding-standards.md) 和 [注释与文档模板](.trellis/tasks/10-11-artex-rebuild-plan/coding-templates.md)。这些是总体规划；已有实现以源码、[身份验收](docs/validation/p0-identity-access.md) 和 [授权与资产验收](docs/validation/p0-authorization-assets.md) 为准。授权与资产切片已通过本地真实数据库、12 个浏览器场景与重启验收。用户已确定下一前端底座采用 [shadcn-admin](https://github.com/satnaing/shadcn-admin)，实际整合尚未实施；源码研究与兼容设计见 [底座研究](.trellis/tasks/10-11-artex-rebuild-plan/research/shadcn-admin-foundation.md)。

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

在管理终端显式配置数据库并迁移，然后预置测试身份与项目成员。issuer 与 subject 必须使用部署 IdP 的精确值，email 和显示名不参与授权：

```powershell
$env:MOUSEHOLE_MODE = 'development'
$env:MOUSEHOLE_DATABASE_URL = 'postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole?sslmode=disable'
python scripts/dev.py migrate
go -C backend run ./cmd/admin provision-user `
  --database-url $env:MOUSEHOLE_DATABASE_URL `
  --issuer http://127.0.0.1:9090 --subject mousehole-local-analyst `
  --user-id 10000000-0000-4000-8000-000000000001 --display-name 本地分析员 `
  --project-id 10000000-0000-4000-8000-000000000002 --project-name 本地评估项目 `
  --role analyst
```

会话和权限撤销使用构建后的管理员二进制，以便脚本获得稳定退出码。每条命令都要求显式 DSN 和标准小写 UUID，不读取 `.env`；重复执行成功且不重复写管理审计。禁用和撤销只更新状态并保留用户、成员、命令及历史审计：

```powershell
New-Item -ItemType Directory -Force .local/bin | Out-Null
go -C backend build -o ../.local/bin/mousehole-admin.exe ./cmd/admin
$admin = (Resolve-Path .local/bin/mousehole-admin.exe).Path
& $admin revoke-sessions --database-url $env:MOUSEHOLE_DATABASE_URL `
  --user-id 10000000-0000-4000-8000-000000000001
& $admin revoke-membership --database-url $env:MOUSEHOLE_DATABASE_URL `
  --user-id 10000000-0000-4000-8000-000000000001 `
  --project-id 10000000-0000-4000-8000-000000000002
& $admin disable-user --database-url $env:MOUSEHOLE_DATABASE_URL `
  --user-id 10000000-0000-4000-8000-000000000001
```

| 退出码 | 含义                                             |
| ------ | ------------------------------------------------ |
| `0`    | 成功，包括状态已经满足要求的幂等重放和帮助输出。 |
| `1`    | 未分类的内部失败。                               |
| `2`    | 用法错误、缺少参数、无效 DSN 或非标准 UUID。     |
| `3`    | 指定用户或项目成员关系不存在。                   |
| `4`    | 预置参数与既有稳定绑定冲突。                     |
| `5`    | 数据库连接或事务结果无法确认。                   |

在独立终端启动工程拥有的合成 OIDC issuer。它只用于本地开发和 E2E，不连接真实企业 IdP：

```powershell
$env:MOUSEHOLE_OIDC_CLIENT_ID = 'mousehole-local'
$env:MOUSEHOLE_OIDC_CLIENT_SECRET = 'mousehole-local-secret'
python scripts/dev.py oidc
```

在 API 终端设置身份与会话配置后启动服务。示例 auth key 是公开的本地占位值，生产必须使用秘密管理系统生成并注入独立的 32 字节随机值：

```powershell
$env:MOUSEHOLE_MODE = 'development'
$env:MOUSEHOLE_DATABASE_URL = 'postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole?sslmode=disable'
$env:MOUSEHOLE_PUBLIC_URL = 'http://127.0.0.1:8080'
$env:MOUSEHOLE_WEB_ORIGIN = 'http://127.0.0.1:5173'
$env:MOUSEHOLE_OIDC_ISSUER = 'http://127.0.0.1:9090'
$env:MOUSEHOLE_OIDC_CLIENT_ID = 'mousehole-local'
$env:MOUSEHOLE_OIDC_CLIENT_SECRET = 'mousehole-local-secret'
$env:MOUSEHOLE_AUTH_KEY = 'bW91c2Vob2xlLWxvY2FsLWF1dGgta2V5LTMyYnl0ZXM='
python scripts/dev.py api
```

另一终端运行前端：

```powershell
python scripts/dev.py web
```

打开 [中文工作台](http://127.0.0.1:5173/) 并选择“登录”。合成 issuer 会自动使用预置 subject 完成标准授权码 + PKCE 流程。API 默认在 `127.0.0.1:8080`，Vite 代理 `/api`；所有受保护请求都重新检查 Cookie 会话、用户状态和项目成员关系，写请求还检查 Origin 与 CSRF。[.env.example](.env.example) 只展示配置，脚本不自动读取它。

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

`verify_restart.py` 要求同终端设置 `MOUSEHOLE_MODE`、`MOUSEHOLE_DATABASE_URL` 且已迁移。它在随机 loopback 端口启动独立合成 OIDC issuer 与 API，预置随机身份和项目，完成登录、Cookie/CSRF 写入、API 重启及原回执逐字段核对；保留该条验收数据。集成测试则使用随机 schema，只清理自己创建的 schema。

安装浏览器后，用统一入口运行端到端检查。该入口构建二进制、执行迁移、预置独立合成身份及两个项目，并启动自己的 API 和 issuer；Playwright 自动启动 Vite：

```powershell
pnpm --dir web exec playwright install chromium
python scripts/verify_authorization_e2e.py
```

入口要求显式 `MOUSEHOLE_MODE=development`、`MOUSEHOLE_DATABASE_URL`，以及空闲的 8080、5173 和合成 issuer 端口（默认 19090）。每轮默认生成新的 actor、subject、主项目与成员撤销项目；撤销仅影响第二项目，不恢复历史成员。若显式指定这些夹具 ID，重跑须改用新的身份及项目。检查只停止自己启动的进程，业务历史保留。

两个验收脚本均接受 `--environment-file <UTF-8 JSON 文件>` 显式合并环境，不自动读取 `.env`；环境文件应放在 Git 忽略目录。诊断日志写入 `.local/validation/`，隐藏子进程参数并脱敏认证秘密。`python scripts/check.py --integration --e2e` 仍可针对已启动的 API/issuer 合并格式与行为检查，不替代独立重启检查。`--scope backend|web|text` 可缩小范围；`--race` 增加 Go 竞态检测。Linux CI 配置了竞态检查；Windows 需要可用的 C 编译工具链。缺数据库、浏览器或指定工具时检查失败，不将跳过当成通过。CI 配置见 [check.yml](.github/workflows/check.yml)，配置已落盘与云端实际运行成功是两个不同状态。

前端统一格式化：`pnpm --dir web format`；根文档使用同一 Prettier 配置。Go 和 sqlc 生成命令见 [后端说明](backend/README.md)，前端状态、来源与测试见 [前端说明](web/README.md)。生成目录只允许通过生成器更新。

默认 Go 模块代理为官方配置。本地网络若无法访问，可只在当前终端设置 `$env:GOPROXY = 'https://goproxy.cn,direct'`（Linux 使用 `export GOPROXY=...`），仍保留 Go 的校验和校验，不修改全局设置。

## 资产、授权与预检查

分析员在“资产”页整批预览及导入 domain/IP/HTTP(S) URL；任一行无效时须先修正，不会暗中跳过。企业资产按规范值去重，但来源只对关联项目可见；来源表示导入声明，资产登记不扩大授权。

在“授权范围”保存候选版本，填写依据引用、联系人、UTC 时窗、结构化范围、静态操作目录及预算上限。正文保存后只读；复制修改生成新版本。管理员通过受控 CLI 激活、替代或撤销，当前项目策略修订可在页面查看。候选、未到时窗、过期或撤销版本均不能静态匹配；撤销当前版本不会自动回退。

先构建管理员入口并从页面取得项目、候选版本及当前 `project_revision`。下例的 ID 和修订变量应使用实际对象；命令 ID 在发送前生成并保留：

```powershell
go -C backend build -o ../.local/bin/mousehole-admin.exe ./cmd/admin
$scopeAdmin = (Resolve-Path .local/bin/mousehole-admin.exe).Path
$scopeCommandId = [guid]::NewGuid().ToString()
& $scopeAdmin scope activate --database-url $env:MOUSEHOLE_DATABASE_URL `
  --project-id $projectId --version-id $versionId `
  --command-id $scopeCommandId --operator-ref change-2026-001 `
  --expected-policy-revision $policyRevision
& $scopeAdmin scope-command get --database-url $env:MOUSEHOLE_DATABASE_URL `
  --project-id $projectId --command-id $scopeCommandId --operator-ref change-2026-001
```

`scope revoke` 使用相同参数形式，但须生成新的命令 ID 并填写当前修订。首次变更核对 expected revision，冲突不覆盖；成功命令按原 ID、原参数及原操作引用重放时返回原回执，不重做业务。成功 stdout 为单个 JSON；`actor_id=null`、`operator_source=admin_cli` 明确表示受控管理操作，操作引用不冒充 OIDC 身份。

紧急禁止使用 `scope deny-add --payload-file <JSON 文件>`，正文只含 `kind`、`value`、`include_subdomains`、`reason`；`kind` 为 domain/ip/cidr/url_exact。项目规则用 `--project-id`，全局规则用 `--global`，两者只选一个，均提供命令 ID、操作引用和对应当前策略修订。撤销禁止用 `scope deny-revoke --deny-id <UUID>`；查询用 `scope-command get` 的相同命名空间及原操作引用。

“策略预检查”只提供 `dns_lookup`、`tcp_connect`、`http_metadata` 的本地静态判断，不连接目标。结果只有“静态规则拒绝”或“静态规则匹配，执行条件尚未验证”，包含原检查时间、版本/hash、策略修订、理由和运行时缺项。历史回执恢复不重新计算；要检查当前规则，显式结束原命令并重新提交。连接中断时保留原 ID 查询，不生成第二条未知命令。

## 代码组织与后续工作

| 路径                                                | 责任                                   |
| --------------------------------------------------- | -------------------------------------- |
| `api/openapi.yaml`                                  | HTTP 请求、回执和错误契约。            |
| `backend/cmd`                                       | API 启动和显式迁移。                   |
| `backend/internal/identity`                         | OIDC、会话、CSRF、预置身份和项目投影。 |
| `backend/internal/command`                          | 事务内成员权限、幂等受理与恢复查询。   |
| `backend/internal/access`                           | 用户、项目、成员的稳定锁与能力判断。   |
| `backend/internal/target`                           | 资产、范围与预检查共用的纯函数规范化。 |
| `backend/internal/scopecommand`                     | 独立范围命令受理、冻结回执与恢复。     |
| `backend/internal/asset`、`authorization`、`policy` | 项目资产、授权版本及纯本地静态判断。   |
| `backend/internal/httpapi`                          | Cookie 会话、严格来源和中文错误映射。  |
| `backend/db`、`backend/internal/store/sqlc`         | 编号迁移、参数化查询和生成访问代码。   |
| `web/src/features/commands`                         | 中文表单、查询恢复和回执展示。         |
| `web/src/features/scope`                            | 资产、授权范围、预检查与原命令恢复。   |
| `web/src/components/ui`                             | 自持 shadcn 基础控件及来源记录。       |
| `scripts`、`deploy`、`.github`                      | 本地入口、数据库依赖及 CI。            |

当前已实现标准 OIDC 合同和生产 Cookie 配置边界，但仍需针对实际企业 IdP 做部署联调；尚无 SAML、SCIM、JIT、Temporal、Runner、模型调用、SSE 或目标探测能力。后续沿已批准路线建设持久化编排、隔离执行、证据和发现闭环；首版自主能力上限仍为主动枚举与非破坏性验证。具体代码约定见 [后端 spec](.trellis/spec/backend/index.md) 与 [前端 spec](.trellis/spec/frontend/index.md)。
