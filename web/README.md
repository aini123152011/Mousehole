# Mousehole 中文工作台

本模块提供任务草稿、资产登记、授权版本和静态策略预检查。所有数据来自真实控制面 API；“已受理”表示记录已保存，“静态规则匹配”仍有运行时条件尚未检查。

## 技术与职责

采用 React 19、TypeScript、Vite、Tailwind CSS 4 和自持 shadcn/ui 基础控件。TanStack Query 保存服务器 session、项目投影、列表、版本和原命令回执；React 局部状态保存编辑输入、交互阶段和项目内搜索位置；React Hook Form/Zod 校验表单，React Router 提供详情和返回导航。浏览器不把身份、CSRF、目标、输入草稿或回执写入 Web Storage、URL 或路由 history state。

| 路径                          | 职责                                                               |
| ----------------------------- | ------------------------------------------------------------------ |
| `src/app/`                    | 页面外壳、会话、项目选择、退出及保护缓存取消/移除                  |
| `src/features/commands/`      | 任务草稿、原 ID 恢复及原有账本交互                                 |
| `src/features/assets/`        | 手动/API 来源、服务端预览、项目列表及来源详情                      |
| `src/features/authorization/` | 候选表单、不可变版本、复制及当前策略只读展示                       |
| `src/features/prechecks/`     | 静态预检查输入、历史决策和运行时待检查项                           |
| `src/features/scope/`         | 跨页面命令锁、原 ID 查询、分页和查询状态                           |
| `src/api/client.ts`           | 生成 HTTP client、Cookie/CSRF、超时及响应对象绑定                  |
| `src/api/generated/`          | 从 [OpenAPI](../api/openapi.yaml) 生成的类型和 JSON Schema         |
| `src/components/ui/`          | 自持基础控件，来源及升级规则见 [说明](src/components/ui/README.md) |
| `src/styles/globals.css`      | 统一 token、中文排版、焦点和响应式布局                             |
| `e2e/`                        | 真实 API/PostgreSQL/OIDC 的关键浏览器路径                          |

## 路由与业务边界

`/` 创建任务草稿；`/assets` 查询/导入，`/assets/:assetId` 查看当前列表页中资产的本项目来源；`/authorization` 查看版本，`/authorization/new?copy=:versionId` 可复制为新候选，`/authorization/:versionId` 查看不可变正文；`/prechecks` 创建静态检查，`/prechecks/:decisionId` 查看持久化解释。地址栏仅使用对象 ID、非敏感类型和签名分页 cursor。

资产输入最多 100 条，由服务端统一规范化。预览含错误行时整批不导入；变更输入或预览失败后需要再次确认预览。手动和 API 来源都是登记声明，资产存在不获得授权。列表采用 25 条 seek 分页，明确区分页内数量与未知总量。敏感搜索按 React Router 位置键保存在项目内存，详情返回和浏览器前进/后退恢复对应搜索与 cursor；退出、换项目或刷新后内存丢弃。

候选保存后只读，复制仅创建新版本。预算以十进制字符串保存，时间输入使用本机时区并转为 UTC。激活、撤销、紧急禁止仅由受控管理员 CLI 完成；Web 没有管理员写入按钮。预检查只校验本地静态语义，不请求目标 DNS/TCP/HTTP。历史快照保留原时刻、hash、版本及原因，当前规则变化会提示；页面不提供执行令牌或扫描入口。

## 命令与缓存边界

提交前生成 UUID、克隆原参数并同步锁定。PUT 只发送一次，15 秒超时；断线、5xx、无效响应或身份/参数不匹配时显示“受理待确认”，保留原 ID/参数，只 GET 原 ID。GET 404 仍不证明未受理。范围命令锁由项目路由祖先持有，在资产、授权、预检查间导航保持有效；明确拒绝或已确认后可显式结束本次命令。

刷新会丢弃页面内状态和缓存，通过 Cookie 重新读取 session。用户需保存原命令 ID，在原项目查询；纯查询不产生 PUT，不重新计算历史规则。已确认回执再次查询失败保留原记录并显示未更新。Query key 包含 actor、项目、筛选、cursor 和对象；退出/401/项目403/项目切换取消相应请求并移除领域缓存，选择代次阻止迟到写响应复活旧缓存。

任务标题与 Go `strings.TrimSpace` 使用同一 Unicode `White_Space` 集合，按 Unicode 字符计数，接受 1–120 个字符；U+FEFF 不作为空白裁剪。

## 本地运行与检查

依赖 Node.js **24.19.0**、pnpm **12.4.2**。按 [根说明](../README.md) 准备并显式启动 PostgreSQL 和 API，在 `web/` 执行：

```powershell
pnpm install --frozen-lockfile
pnpm run dev
```

Linux 使用同一命令。浏览器地址为 `http://127.0.0.1:5173`，Vite 严格绑定 loopback。`/api` 代理至 `http://127.0.0.1:8080` 并改写 Host；后端校验 Host/Origin。管理员须先预置企业身份和项目角色，登录回调由服务端设置 HttpOnly Cookie，CSRF 只保存在内存。Vite 开发服务不是生产交付形态。

新依赖遵循 `pnpm-workspace.yaml` 的 24 小时发布窗口；精确版本和唯一 lockfile 纳入审查。契约生成同时输出 TS 类型与 Ajv 2020 Schema，不能手改生成文件。API 边界验证未知字段、枚举、状态、嵌套项目、命令和对象身份、日期及分页一致性；手写注释、测试场景和错误说明使用中文。

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

格式采用根 `.prettierrc.json`，仅精确忽略生成文件、许可证、lockfile 和构建产物。自持 UI 源码参加全部检查。

真实浏览器验收由 [验收脚本](../scripts/verify_authorization_e2e.py) 配置随机身份、两个项目和管理员二进制，并显式传入环境。Playwright 自动启动 5173；全部主项目场景串行，权限撤销项目依赖主验收完成后最后执行，避免撤销夹具污染其他测试。缺少真实依赖会失败，不跳过或伪造业务响应。

```powershell
pnpm exec playwright install chromium
pnpm run e2e
```

验收只使用随机 ID 和合成保留目标，覆盖 OIDC/Cookie、资产错误行/去重/来源、筛选分页与返回、候选只读、CLI 激活/禁止、默认拒绝/运行时待检、原 ID 回放、真实提交后的浏览器丢包恢复、权限撤销及窄屏键盘操作。诊断截图在忽略的 `test-results/`；四张新切片合成截图写入 `docs/validation/assets/` 供文档核验。请求 trace 关闭，避免记录认证秘密。

SAML、SCIM、Web 成员管理、Runner、Agent 执行、SSE 和完整读屏器认证仍属于后续切片。
