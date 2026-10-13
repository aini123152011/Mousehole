# P0 授权版本、资产规范化与策略预检查验收

本切片于 2026-10-12 获准实施，2026-10-13 完成本地验收。检查仅使用 Mousehole 控制面、本工程 PostgreSQL 与合成 OIDC；ARTEX 仅作源码研究，未运行、构建、测试或探测。后端、真实数据库、Web、浏览器与重启检查已通过；最终文本门禁和独立复核已通过，不替代完整平台验收。

## 环境与历史

Windows 本地锁定 Go 1.26.9、Node 24.19.0、pnpm 12.4.2；Compose PostgreSQL 17 绑定 loopback 55432。配置从显式进程环境或 Git 忽略的 UTF-8 JSON 映射注入，不自动读取 .env，不公开 DSN、Cookie、CSRF 或认证秘密。集成测试只清理自身随机 schema；浏览器每轮更换合成身份、主项目与独立撤销项目，不恢复 revoked 成员；重启测试使用随机端口。只停止检查自有进程，保留业务记录和数据库卷。

## 实际检查

| 检查                                                              | 状态   | 证据与限制                                                                                                                                                                                                                            |
| ----------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `python scripts/check.py --scope backend --integration`           | 通过   | 官方命令 exit 0；gofmt/goimports、sqlc 漂移、mod verify、vet、全部单元及真实 PG。实现代理会话 33462、完成输出 e633c2，未另存日志，不虚构路径。PG command/identity/scopecommand 为 5.060s/7.204s/22.440s；非 verbose，未统计测试总数。 |
| `python scripts/check.py --scope web`                             | 通过   | 最后 UI 修复后 exit 0；根/Web Prettier、ESLint、类型、生成一致、8 文件 56 项 Vitest、build。主 JS 698.79KB/gzip 209.27KB 超默认 500KB 提示线，留待后续拆分。                                                                          |
| `python scripts/verify_restart.py --environment-file …`           | 通过   | 合成 OIDC 登录；旧草稿/资产/候选/预检查在 API 重启后 GET/PUT 回执逐字段一致；CLI 激活/撤销原回执重放和查询一致。日志 `.local/validation/restart-1bbc57f02efa4294ae8aad7ea0aef559.log`。                                               |
| `python scripts/verify_authorization_e2e.py --environment-file …` | 通过   | 真实 PG/API/OIDC/Web：12 passed (32.9s)，含手机键盘双项目切换、三列表分页与最后独立项目撤销。日志 `.local/validation/authorization-e2e-32152f0a097943868eabf2c1d01abeb4.log`。                                                        |
| 00004 前向迁移与 status                                           | 通过   | 00001–00004 全 applied，旧历史保留；已有 namespace 函数/trigger 兼容另有 PG 回归。日志 `.local/validation/authorization-migration-00004-final.log`。                                                                                  |
| 最终 text / diff                                                  | 通过   | 最后 UI 修复及交付文档定稿后，官方 text 161 文件与 git diff --check 通过；SQL 换行修复后的 sqlc 漂移检查通过。                                                                                                                        |
| 独立 Trellis check                                                | 通过   | 独立报告核验最新 PRD/design/implement、全层 spec、最终 E2E 和截图，没有未修复阻塞；路径为本任务 research/independent-check.md。                                                                                                       |
| Windows Go race / Linux 云 CI / 真实企业 IdP                      | 未执行 | 未确认 Windows C 工具链；CI 配置不等于云运行通过；合成 issuer 不替代企业联调。                                                                                                                                                        |

首次浏览器检查因 Node 24 JSON import attribute 缺失在收集阶段失败，修复后收集成功。第二轮 10 passed、手机项目入口 1 failed、依赖撤销 1 did not run，未作完整通过。修复隐藏选择器并增加真实键盘双项目往返后，用新身份和独立项目跑最终 12 场景全部通过。复制按钮文字被全局链接色覆盖，普通链接规则排除 `[data-slot='button']` 后恢复。

## PA-A1–A9

| 验收  | 状态 | 已观察行为                                                                                                                                                                                                    |
| ----- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PA-A1 | 通过 | target 表驱动 IDNA/尾点/IP/mapped/URL 默认端口及 path/query 差异；PG 无效批次原子拒绝、24 路同 ID 导入唯一、12 路不同命令同资产关联/来源计数；真实预览错误行和键盘导入。                                      |
| PA-A2 | 通过 | PG 跨项目来源/版本/决策/FK、viewer 写拒绝；第二 analyst 对三种 Web kind 的 GET/同参 PUT/异参 PUT 均 404、空回执且表计数不变；E2E 项目切换清输入、撤销后页面/请求/旧 Cookie 重载拒绝。                         |
| PA-A3 | 通过 | PG 正文/条目不可变及延迟集合一致、复制新候选、激活替代、撤销不回退、CAS；未来/过期激活拒绝且 revision 不变；policy 精确 `[from,expires)`；浏览器只读/激活/历史说明。                                          |
| PA-A4 | 通过 | 标签/PRIVATE 后缀、CIDR 主机位/家族/mapped、URL scheme/port/path/query/尾 slash/空 query、目录/参数/平台地址反例；完整私网返回 requires_runtime_checks；浏览器 CIDR 字段定位与修正。                          |
| PA-A5 | 通过 | policy 纯本地，DNS/HTTP 拒绝夹具目标尝试为零；夹具不声称拦截任意 TCP，实现无 TCP 目标调用。回执仅 deny/requires_runtime_checks，原时刻/版本/hash/revision/原因/八类待检齐全，无执行令牌。                     |
| PA-A6 | 通过 | 权限12种：user/project/member × 新旧命令 × 两种顺序；策略8种：activate/revoke/project deny/global deny × 两种顺序。pg_locks/pg_blocking_pids/advisory gate 真实观察先后，稳定行覆盖空 deny，不用 sleep 推定。 |
| PA-A7 | 通过 | 导入24路、candidate/precheck各12路同键唯一；跨 kind/异参数409且账本不变；三 Web kind × audit/outbox 共6种整体回滚。E2E 丢响应后仅一次 PUT、原 ID GET 跨导航恢复；重启/CLI 冻结回执一致。                      |
| PA-A8 | 通过 | PG29条版本/决策及 cursor 资源/项目/篡改反例；E2E 三领域多页无重漏、详情返回、敏感搜索 back/forward、新旧 unknown 跨导航、手机键盘双项目往返/跳转/焦点/错误/无横溢。                                           |
| PA-A9 | 通过 | PG旧历史升级、00003/00004与已有 namespace 兼容；旧草稿回归。生成/格式/lint/类型/build/PG/12 E2E/重启通过；中文抽查与实际 spec 同步。race/云CI/企业IdP仍未验证。                                               |

实现和测试索引见独立报告，测试存在与实跑分开核验。静态子集不替代父 A1/A6 的实际出口与 Runner。

## 中文与视觉抽查

人工抽查 target/doc.go 纯函数边界、normalize/match 的 IDNA/URL/CIDR 原因、access/access.go 权限锁、scopecommand/service.go 事务/重放/查询、00003/00004不可变与集合一致、use-scope-command.ts 原 ID、protected-cache.ts 取消/缓存所有权，以及受控字段错误与公开中文说明。保留英文符号、生成器声明和第三方许可证，不以字符比例替代语义审查。

最终四图已实际查看：复制按钮白字、手机项目及四导航可见，UUID/hash换行，来源声明/未知总量/历史结果/八类待检说明完整。当前浅色外观是既有工作台；完整 shadcn-admin 底座与深色主题属下一任务。

- [资产桌面](assets/p0-authorization-assets-desktop.png)
- [授权详情桌面](assets/p0-authorization-version-desktop.png)
- [预检查桌面](assets/p0-authorization-precheck-desktop.png)
- [预检查手机](assets/p0-authorization-precheck-mobile.png)

## 交付边界

资产来源是导入声明；历史决策不表示当前许可。实际 DNS/连接/重定向、可信适配器、lease/fencing、实际预算、Runner/出口、Agent与证据闭环后置；不宣称完整平台或 ARTEX 性能对照完成。用户于2026-10-13确定 shadcn-admin 底座，本切片保留为整合前可恢复成果；后续真实引入布局/导航/主题/列表源码，接入现有 Go/OIDC/OpenAPI/Ajv/命令恢复，排除演示认证。
