# 前端开发规范

适用于已实现 React/TypeScript/Vite 中文工作台。shadcn 是自持基础控件层，HTTP 契约由 OpenAPI 生成。手写文档、注释、状态和错误说明中文；英文字段、状态、类型与第三方许可证保留。

## 开发前检查

阅读任务 PRD、设计、执行计划，以及 [完整中文规范](../../tasks/10-11-artex-rebuild-plan/coding-standards.md)。按变更阅读下列实际规范：

| 规范                               | 已落实内容                     |
| ---------------------------------- | ------------------------------ |
| [目录结构](directory-structure.md) | 特性、API、UI 与生成边界       |
| [组件](component-guidelines.md)    | shadcn 来源、中文组合、无障碍  |
| [Hook](hook-guidelines.md)         | 稳定 ID、同步交互锁、迟到查询  |
| [状态](state-management.md)        | Query 权威回执与未知结果       |
| [类型安全](type-safety.md)         | strict、OpenAPI、Ajv/Zod       |
| [质量](quality-guidelines.md)      | 中文注释、格式、行为和真实 E2E |

## 质量检查

根目录执行 `python scripts/check.py --scope web`。接口变更需要生成与生成漂移对比。真实端到端须先启动已迁移 API 和合成 OIDC issuer，再运行 `pnpm --dir web e2e`；Playwright 自动启动 Web。结果登记在 [身份切片验收记录](../../../docs/validation/p0-identity-access.md)，不得把 mock 测试等同于真实数据库或全栈通过。
