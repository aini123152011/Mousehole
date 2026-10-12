# P0 草稿命令闭环验收记录

## 范围与环境

用户已批准的首个实施切片，见 [PRD](../../.trellis/tasks/10-12-p0-command-foundation/prd.md) 和 [跨层合同](../../.trellis/tasks/10-12-p0-command-foundation/design.md)。本次只向本地合成项目写入任务草稿；未连接 ARTEX 数据库或访问评估目标。ARTEX 源码保持只读。

实跑环境为 Windows、Go 1.26.9、Node.js 24.19.0、pnpm 12.4.2、Python 3.14、Docker Compose 和 PostgreSQL 17。镜像及生成器版本见 [工具清单](../../scripts/toolchain.json)，启动与检查命令见 [根 README](../../README.md)。Go 置于工作区 `.tools/`，未全局安装工具或修改永久 PATH。

## 已通过检查

2026-10-12，独立检查代理完成最终全范围命令 `python scripts/check.py --integration --e2e`，退出码为 **0**。下表中的数据库、全进程重启及脚本验证也已实际执行；mock 组件测试与真实端到端结果分别记录。

| 检查                 | 命令或证据                                                                                          | 结果与含义                                                                                                                                                             |
| -------------------- | --------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 开发数据库           | `python scripts/dev.py database`                                                                    | 数据库健康；按需创建固定测试库，保留已有开发数据。                                                                                                                     |
| 文本与统一格式       | 最终全范围检查                                                                                      | 72 个工程文本文件通过编码/换行检查；根及 Web Prettier、Go gofmt/goimports 通过。                                                                                       |
| Go 静态、生成与行为  | 最终全范围检查                                                                                      | sqlc 完整文件集合及字节比较、模块校验、vet、6 个顶层单元测试和 3 个真实 PostgreSQL 集成测试通过。                                                                      |
| 事务与并发           | 真实 PostgreSQL 集成测试                                                                            | 24 个同 ID 并发请求只有一份命令、草稿、审计和 outbox；规范化回放保留原回执，异参数冲突，COMMIT 失败四表整体回滚，连接池重建后回执保留。                                |
| 显式迁移             | `python scripts/dev.py migrate`                                                                     | 开发库应用 1 个编号迁移；API 启动不执行迁移。                                                                                                                          |
| 全进程持久化恢复     | `python scripts/verify_restart.py`                                                                  | 创建后终止 API 进程，再启动新进程，GET 与原回执逐字段一致。验收命令 ID：`37743d46-a7a5-4a4e-9ef4-35801ee4ab41`。                                                       |
| Web 静态、生成与构建 | 最终全范围检查                                                                                      | ESLint、TypeScript strict、OpenAPI 完整文件集合及字节漂移检查、Vite 生产构建通过。                                                                                     |
| Web 单元与组件       | 最终全范围检查中的 Vitest                                                                           | 4 个文件、28 个测试通过，覆盖回执校验、命令状态、表单及未知结果恢复。                                                                                                  |
| 真实全栈浏览器       | 最终全范围检查中的 Playwright                                                                       | Chromium 4/4 通过；实际贯通 Vite 代理、Go 和 PostgreSQL。覆盖创建/回放/冲突/刷新恢复、真实提交后丢弃浏览器响应并按原 ID GET 恢复、404 保留待确认、手机布局和键盘焦点。 |
| 锁定依赖             | `pnpm --dir web install --frozen-lockfile` 及 peer 检查                                             | 锁定安装与 peer 检查通过，保留唯一 pnpm lockfile。                                                                                                                     |
| 脚本语法             | `python -m py_compile scripts/tooling.py scripts/check.py scripts/dev.py scripts/verify_restart.py` | 四个根脚本语法通过。                                                                                                                                                   |

## 独立审查与规范回写

审查完成后没有剩余功能发现。下列修复均在最终全范围检查中验证，相应边界已写入实际 [后端工具合同](../../.trellis/spec/backend/tooling-contract.md)、[前端类型合同](../../.trellis/spec/frontend/type-safety.md) 和 [组件规范](../../.trellis/spec/frontend/component-guidelines.md)。

- 来源配置：URL 解析器可能丢弃空片段。检查原始字符串，拒绝带查询、空查询、片段、空片段的 Origin；回归覆盖末尾 `?` 与 `#`。
- 回执日期：OpenAPI 要求 UTC `Z` 后缀；客户端继续确认 `Date.parse` 结果有限。非 UTC 偏移或浏览器无法表示的闰秒进入待确认处理，避免日期展示崩溃；增加四个非法响应测试。
- 生成漂移：比较完整文件集合及字节。实际加入嵌套遗留文件、移走生成目录时，检查均退出 1；正常输出退出 0。失败检查不创建目录、不覆盖源码，检查前后原生成文件 SHA-256 一致，临时路径已清理。
- 页面交互：新建草稿解锁渲染后恢复标题焦点；跳转链接脱离正常布局，键盘聚焦不推动导航；手写装饰标签改为中文。

## 视觉与使用证据

已目视检查桌面创建前、桌面恢复回执和 390 px 手机校验状态，中文显示正常，无遮挡和横向溢出。Chromium 实测跳转链接、错误反馈与焦点恢复；未执行屏幕阅读器测试，不能据此声明全面无障碍认证。

- [桌面创建前](assets/p0-desktop-empty.png)
- [桌面按原 ID 恢复回执](assets/p0-desktop-recovered.png)
- [手机校验状态](assets/p0-mobile-validation.png)

## 文档与任务复核

交付文档经另一代理只读核对，完成声明、测试计数、响应丢失与 404 语义、三个截图链接均一致。根 README、验收文档、实际前后端 spec 和父子任务 Markdown 共 30 份通过 UTF-8/LF 与本地目标检查，549 个本地链接均存在；这些文件通过 Prettier。父子任务 context validate 通过，分别为每个 manifest 7 条与 17 条。

## 保留的非阻塞提示

Vite 构建提示单个 JS 文件约 582.30 kB，gzip 177.32 kB，超过默认 500 kB 提示阈值；未提高阈值，本切片尚无性能预算，后续页面扩展时实测加载并决定拆包。Playwright 有 `NO_COLOR` 与 `FORCE_COLOR` 同时设置的提示，测试正常完成。

## 尚未执行或不在本次范围

- Windows Go race 尚未执行，当前未准备其要求的 C 编译工具链。
- Linux CI 已配置真实 PostgreSQL、race、生成漂移和 E2E，尚未在云端执行，不能视为已通过。
- 没有生产 OIDC、Temporal、Runner、模型调用、SSE 或目标执行。本地固定身份不是生产登录方案。
- 没有实际与 ARTEX 同场景的效率、容量或自主完成率对照测试；更强、更好用仍由父任务后续验收证明。
- 根目录尚非 Git 仓库，未初始化、提交或假造提交记录；任务提交及归档尚未完成。

## 后续交付边界

本切片验证了平台基础命令链的格式、持久化和恢复合同，不等于完整 P0 或高自主 V1 已完成。后续按 [总体实施路线](../../.trellis/tasks/10-11-artex-rebuild-plan/implement.md) 建设企业身份与授权、领域闭环、持久化编排和隔离执行，并保持首版主动枚举与非破坏性验证上限。
