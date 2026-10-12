# Mousehole 中文工作台

此模块是已批准 P0 的开发纵向样本：在固定本地项目内创建任务草稿，读取服务端持久化回执。回执“已受理”只表示草稿已保存。页面没有目标访问、扫描、Runner 或 Agent 执行入口。

## 技术与职责

采用 React 19、TypeScript、Vite、Tailwind CSS 4 和自持 shadcn/ui 基础控件。TanStack Query 保存服务端 session、项目投影和命令回执；React 局部状态表达项目选择、命令选择、提交过程和提示；React Hook Form/Zod 校验表单。浏览器不保存 token、Cookie、CSRF 或服务器快照到 localStorage/sessionStorage。

| 路径                     | 职责                                                               |
| ------------------------ | ------------------------------------------------------------------ |
| `src/app/`               | 页面外壳、企业会话查询、项目选择、退出和 Query 缓存配置            |
| `src/features/commands/` | 草稿表单、原 ID 恢复、交互锁和回执展示                             |
| `src/api/client.ts`      | 生成 HTTP client、Cookie 凭据、CSRF、超时和边界校验                |
| `src/api/generated/`     | 从 [OpenAPI](../api/openapi.yaml) 生成的类型和 JSON Schema         |
| `src/components/ui/`     | 自持基础控件，来源及升级规则见 [说明](src/components/ui/README.md) |
| `src/styles/globals.css` | 统一 token、中文排版、焦点和响应式布局                             |
| `e2e/`                   | 使用真实 API/PostgreSQL 的关键浏览器路径                           |

## 本地运行

依赖 Node.js **24.19.0**、pnpm **12.4.2**。从仓库根目录按 [根说明](../README.md) 准备并显式启动 PostgreSQL 和 API，随后在 `web/` 执行：

```powershell
pnpm install --frozen-lockfile
pnpm run dev
```

Linux 使用相同 pnpm 命令。浏览器地址是 `http://127.0.0.1:5173`；Vite 严格绑定此 loopback 地址与端口。`/api` 代理至 `http://127.0.0.1:8080` 并改写 Host，后端校验 Host/Origin。首次访问首屏显示企业登录入口；管理员必须先预置 `(issuer, subject)` 和项目角色。回调成功后服务端设置 HttpOnly Cookie，前端只把 session 响应内的 CSRF token 保存在内存。该样本不能以 Vite 开发服务交付生产环境。

`pnpm-workspace.yaml` 要求新依赖发布满 24 小时，不设置包级豁免。React Hook Form 7.89.0、Zod 4.6.5 满足窗口；依赖精确版本及唯一 lockfile 纳入审查。`ajv-formats` 2.1.1 满足已锁定 resolver 的 peer 约束。

## 命令与恢复边界

提交前生成 UUID 并同步锁定标题/命令 ID。标题与 Go `strings.TrimSpace` 使用同一 Unicode `White_Space` 集合，规范化后按 Unicode 字符计数，接受 1–120 个字符；U+FEFF 不作为空白裁剪。

PUT 只发送一次，15 秒超时；网络中断、5xx、无效回执或参数失配均显示“受理待确认”。此时保持原 ID 和参数，禁用新建和其他 ID 查询，只允许 GET 原命令。GET 404 仍不证明未受理，用户可继续查询。明确匹配的 4xx 拒绝后才允许显式新建另一份草稿。

刷新页面会丢弃页面内命令交互和缓存，并通过 Cookie 重新读取服务端会话；用户需保存原 ID，再用“找回已有命令”输入它。查询本身不产生 PUT；纯查询失败允许返回新建。已确认回执再次查询失败时保留上次记录并显示未更新提示。选择代次和包含 actor/项目/命令的 Query key 隔离迟到响应；退出、401、项目 403 与项目切换按范围清理缓存。

## 契约生成与验证

生成器读取 [OpenAPI](../api/openapi.yaml)，同时生成 TS 类型和 Ajv 2020 校验 schema。不得手改生成文件；未知字段、状态、对象身份和绑定标题均在统一 API 边界检查。英文生成标记及固定工具语法保留，手写注释和测试场景使用中文。

```powershell
pnpm run generate
pnpm run check:generated
pnpm run format:check
pnpm run lint
pnpm run typecheck
pnpm run test
pnpm run build
pnpm peers check
```

格式采用根 `.prettierrc.json`，仅精确忽略生成文件、许可证、lockfile 和构建产物。自持 UI 源码仍参加格式、lint 和类型检查。单元/组件测试验证响应丢失、404、明确拒绝、迟到查询、Unicode 边界和无效 HTTP 响应。

浏览器测试要求真实 API 已监听 `127.0.0.1:8080` 并完成数据库迁移。Playwright 自动启动 5173，依赖缺失会失败，不跳过或返回假业务数据：

```powershell
pnpm exec playwright install chromium
pnpm run e2e
```

测试使用随机 ID 的合成草稿，覆盖真实 OIDC 登录、Cookie 会话刷新/退出、创建、同 ID 回放/冲突、刷新找回、提交成功后浏览器丢包恢复、Web Storage 无秘密、手机布局及键盘焦点。截图位于被忽略的 `test-results/`。本切片尚未提供 SAML、SCIM、JIT、Web 成员管理、执行权限、SSE、执行工作流或完整读屏器认证；相应能力按后续阶段建设。
