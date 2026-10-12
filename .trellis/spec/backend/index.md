# 后端开发规范

适用于已实现的 Go 企业 OIDC、服务端会话、项目成员授权和草稿命令切片，依据 `backend/` 与 `api/openapi.yaml`。手写说明、注释和公开消息使用简体中文，英文符号及机器契约保持不变。父任务的完整规范位于 [中文编码规范](../../tasks/10-11-artex-rebuild-plan/coding-standards.md) 与 [注释模板](../../tasks/10-11-artex-rebuild-plan/coding-templates.md)。

## 开发前检查

阅读当前任务的 PRD、设计、执行计划，然后按变更阅读下列规范。跨 HTTP、数据库、页面时同时阅读前端类型与状态规范。

| 规范                               | 已落实内容                                 |
| ---------------------------------- | ------------------------------------------ |
| [目录结构](directory-structure.md) | 入口、传输、命令服务、SQL 输入与生成目录   |
| [数据库](database-guidelines.md)   | 四类记录原子提交、唯一约束、迁移与真实测试 |
| [错误](error-handling.md)          | 稳定错误类别、未知提交结果、HTTP 错误矩阵  |
| [质量](quality-guidelines.md)      | 中文注释、格式与可执行检查                 |
| [日志](logging-guidelines.md)      | slog 中文消息与脱敏责任                    |
| [开发工具链](tooling-contract.md)  | 环境、版本、启动及跨平台验证合同           |

## 质量检查

从根目录运行 `python scripts/check.py --scope backend --integration`，必须配置专用测试 DSN；运行 `python scripts/verify_restart.py` 验证合成 OIDC 登录、Cookie 会话和全进程恢复。接口变更同步运行 Web 生成漂移、前端检查和真实 E2E。实际检查结果登记在 [身份切片验收记录](../../../docs/validation/p0-identity-access.md)，配置 CI 不能替代实际运行。
