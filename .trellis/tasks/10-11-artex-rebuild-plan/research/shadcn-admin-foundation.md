# shadcn-admin 前端底座研究与整合建议

## 决定与证据边界

用户于 2026-10-13 明确选择 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin)，替换此前问答中选择的 Next.js Dashboard Starter。后续前端基于该项目真实源码整合；当前授权与资产切片仍是 React/Vite + 官方 shadcn 基础控件，尚未采用完整后台项目。本研究与后续实施规划不能被理解为迁移已完成。

本次通过 GitHub API 固定上游 main 提交 `e16c87f213a5ba5e45964e9b67c792105ec74d26`，提交时间 `2026-06-11T10:08:35Z`；仓库未归档，许可证 MIT，版权声明为 `Copyright (c) 2024 Sat Naing`。元数据的仓库 pushed_at 为 `2026-09-10T22:30:37Z`，不把它误写成 main 提交日期。源码快照及 SHA-256 清单保存在 Git 忽略目录 `.tools/build/shadcn-admin-research/`；研究只下载、读取源码，未安装或运行上游代码。

下列链接均指向固定提交。当前工程事实来自 `web/package.json`、`web/src/app/app.tsx`、实际前端 spec 和授权切片验收；ARTEX 的历史取长补短仍参考 [原前端研究](frontend-stack-and-ux.md)，未运行 ARTEX。

## 可继承的源码与需适配的行为

| 上游源码                                                                                                                                                                                                                                                                                                           | 核验结果                                                                                                               | Mousehole 的整合方式                                                                                                                                                     |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [package.json](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/package.json)                                                                                                                                                                                                | React 19、Vite 8、Tailwind 4、Radix/shadcn、Query/Table、RHF/Zod；路由为 TanStack Router，还声明 Clerk、Axios、Zustand | 保留现有锁定 React/Vite/Query/RHF/Zod，按实际引入模块补 Radix/Table 等精确版本；不整份覆盖 package/lockfile，不为演示功能引入依赖                                        |
| [authenticated-layout.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/components/layout/authenticated-layout.tsx)、[app-sidebar.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/components/layout/app-sidebar.tsx) | SidebarProvider、SidebarInset、折叠/布局、跳转主内容、分组导航及账户区域已有真实实现                                   | 采用这些布局源码及依赖组件，适配 Mousehole 项目、账户和四个真实工作区；手机折叠菜单仍可选择项目、操作导航                                                                |
| [sidebar-data.ts](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/components/layout/data/sidebar-data.ts)                                                                                                                                                               | 团队、用户、导航使用模板数据                                                                                           | 用 Go session 中的用户和项目替换；项目切换触发完整保护缓存/输入清理，不把演示 TeamSwitcher 当成权限体系                                                                  |
| [theme-provider.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/context/theme-provider.tsx)                                                                                                                                                                        | light/dark/system、系统媒体查询和可读主题 Cookie                                                                       | 复用主题机制并校验枚举；主题偏好可持久保存，资产目标、授权输入、CSRF 和认证 token 不可借此持久化                                                                         |
| [auth-store.ts](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/stores/auth-store.ts)、[user-auth-form.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/features/auth/sign-in/components/user-auth-form.tsx)             | 表单用 sleep 模拟成功，创建 mock-user/mock-access-token；认证 store 使用可读 Cookie                                    | 排除演示认证和 Clerk 路由；登录、退出与 session 继续通过 Go OIDC、HttpOnly Cookie、Origin/CSRF，权限由 Go 复核                                                           |
| [main.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/main.tsx)、[vite.config.ts](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/vite.config.ts)                                                                           | TanStack Router 及自动路由插件；Query 全局错误按 AxiosError 处理，401 重定向、500 跳演示错误页                         | 本轮建议保留现有 React Router，替换布局中的 Link/Outlet/navigation 适配；保留 OpenAPI/Ajv/ApiError 及原 unknown 命令恢复，不再引入第二个路由器或 HTTP client             |
| [tasks-table.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/features/tasks/components/tasks-table.tsx)、[pagination.tsx](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/components/data-table/pagination.tsx)         | 使用客户端过滤/分页模型，分页展示 pageCount、页号和末页                                                                | 复用表格渲染、工具栏、列展示结构；改为现有服务端 seek cursor。未知总量显示本页数量和是否有下一页，不计算假总页/末页；不新增后端未支持的排序/导出                         |
| [styles/index.css](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/styles/index.css)、[theme.css](https://github.com/satnaing/shadcn-admin/blob/e16c87f213a5ba5e45964e9b67c792105ec74d26/src/styles/theme.css)                                                          | Tailwind token 与主题层已集中，组件大多直接使用 token                                                                  | 整合统一 light/dark token 和中文字体回退，删除冲突全局选择器；常规链接规则不能覆盖 Button 的前景色。检查实际引入资源和 index.html，确保运行与构建不依赖外部演示图片/字体 |

## 为什么沿用 React Router

用户选择的是完整后台项目作为前端基础。该目标要求实际继承布局、导航、主题及列表源码，保留许可与来源差异；路由库本身不是用户要求。现有 React Router 的项目祖先持有新旧 unknown 命令，资产搜索按 location.key 在内存恢复历史位置。保留它可以直接复用经过真实浏览器验证的语义，减少本轮不必要的迁移。上游依赖 TanStack Router 的布局和导航代码需要明确适配并记录本地差异，不能仅贴来源链接却重写一套相似外观。

未来若有统一 typed routes 的具体收益，可另行收敛 TanStack Router 迁移；本轮不同时运行两个 Router。该建议应纳入最终整合计划审阅，不提前写为已实现 spec。

## 推荐的交付范围

1. 建立上游许可证、固定提交、每个引入文件的原始 hash/目标路径/本地改动清单；现有官方 shadcn 组件来源记录保留并标清两个来源。
2. 整合 AppSidebar、Header/Main、Sidebar UI、账户与项目区域，加入中文浅/深/跟随系统主题及手机抽屉、键盘/焦点路径。
3. 四个既有工作区迁入真实底座：草稿命令、资产、授权范围、策略预检查；实际数据仍来自 Go，状态机、字段错误、原 ID 恢复和缓存隔离保持验收行为。
4. 资产、授权版本与预检查列表采用上游表格结构，按服务端 cursor 改写分页；敏感搜索不进入 URL、history.state 或浏览器持久存储。
5. 运行全部 Web 门禁、真实 Go/OIDC/PostgreSQL E2E，覆盖桌面/手机、浅/深主题、项目/身份切换、unknown 导航恢复、分页返回；保留当前切片为 Git 回滚点，不删除历史数据。

整合不新增任务执行、Agent、图、证据、真实管理 Web 或演示通知/计费。后端 API/数据库无需因换布局而修改。应先完成当前授权切片的验收、提交与归档，再创建独立子任务；完整 PRD/design/implement 和上下文收敛后按 Trellis 最终规划门槛开始实施。
