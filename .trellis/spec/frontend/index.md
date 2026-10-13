# 前端开发规范

适用于已实现 React/TypeScript/Vite 中文工作台。shadcn 是自持基础控件层，HTTP 契约由 OpenAPI 生成。手写文档、注释、状态和错误说明中文；英文字段、状态、类型与第三方许可证保留。

## 开发前检查

阅读任务 PRD、设计、执行计划，以及 [完整中文规范](../../tasks/10-11-artex-rebuild-plan/coding-standards.md)。按变更阅读下列实际规范：

| 规范                                      | 已落实内容                             |
| ----------------------------------------- | -------------------------------------- |
| [目录结构](directory-structure.md)        | 特性、API、UI 与生成边界               |
| [组件](component-guidelines.md)           | shadcn 来源、中文组合、无障碍          |
| [Hook](hook-guidelines.md)                | 稳定 ID、同步交互锁、迟到查询          |
| [状态](state-management.md)               | Query 权威回执与未知结果               |
| [类型安全](type-safety.md)                | strict、OpenAPI、Ajv/Zod               |
| [质量](quality-guidelines.md)             | 中文注释、格式、行为和真实 E2E         |
| [范围工作台](scope-workspace-contract.md) | 资产、授权、预检查、路由恢复与保护缓存 |

## 质量检查

根目录执行 `python scripts/check.py --scope web`。接口变更需要生成与生成漂移对比。真实端到端使用 `python scripts/verify_authorization_e2e.py --environment-file <显式环境映射>` 准备本轮合成身份、迁移、API 与 issuer；Playwright 自动启动 Web。结果分别登记在 [身份验收](../../../docs/validation/p0-identity-access.md) 和 [授权与资产验收](../../../docs/validation/p0-authorization-assets.md)，不得把 mock 测试等同于真实数据库或全栈通过。
