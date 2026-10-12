# 本次实施顺序

## 分工与依赖

1. 主会话维护已批准范围、跨层合同和 Trellis 状态。本任务的 design.md 先冻结 HTTP/数据约定，前后端以此独立开发；接口变更先同步双方。
2. 后端子代理负责 backend/、api/openapi.yaml：Go/chi 服务、pgx/sqlc、迁移、事务幂等/查询、真实数据库测试、中文模块文档。
3. 前端子代理负责 web/：Vite/React/shadcn 中文界面、OpenAPI 生成 client、明确受理待确认与查询恢复、组件测试和构建。生成 client 依赖 OpenAPI 文件，其他组件可先开发。
4. 工具链子代理负责根开发配置、scripts/、deploy/、.github/、工具准备：统一格式、依赖环境、验证入口与 CI；不修改前后端业务文件。主会话维护根 README 与 .trellis/spec 实际规范。
5. 后端、前端完成后才进行真实全栈测试、浏览器检查和独立 trellis-check；检查失败先修复。完成后更新实际 spec、验证记录与父任务进度。

## 预期检查

实际命令以落地 package scripts/README 为准：Go format/vet/test、sqlc 生成漂移、PostgreSQL 事务并发/回滚/重启查询；Web format/lint/typecheck/test/build、OpenAPI 生成漂移；真实 API 与浏览器验证创建、查询、重复和冲突。生成器输入/输出比较不依赖已有 Git 仓库。

## 回滚与完成边界

本次仅新工程文件与明确修改的规范；保留既有 Trellis 文件及 .gitattributes journal 规则。不修改 ARTEX，不执行目标探测。不删除数据库数据来实现回滚，不全局修改 PATH。如验证受环境阻塞，记录未通过项，不声明该检查通过。

用户已批准本实施切片，不再次索取开始许可。整个父任务不在本轮标记完成；本切片只有验收及独立审查满足后才可完成。

## 实际进度（2026-10-12）

- [x] 落地 Go/chi、pgx/sqlc、PostgreSQL 迁移和四表原子受理/查询；同 ID 参数规范化回放与异参数冲突可验证。
- [x] 落地 React/TypeScript/Vite、shadcn 中文草稿工作台、OpenAPI 类型与运行时校验；未知结果锁定原 ID/参数，只查询不自动重发。
- [x] 落地本地工具、统一格式、开发/检查入口、锁定依赖、中文 README 和 PR 模板、Linux CI 配置。
- [x] 真实数据库验证 24 并发幂等、冲突、COMMIT 回滚和连接池重建恢复；独立 API 全进程重启验证回执逐字段一致。
- [x] 独立 trellis-check 修复 Origin 空片段、UTC/不可显示时间与生成文件集合缺口；最终 `python scripts/check.py --integration --e2e` 退出 0，Vitest 28/28、真实 Chromium 4/4 通过。
- [x] 桌面/手机目视复核，保存截图；实际中文 spec 已回写。具体证据与保留提示见 [验收记录](../../../../../docs/validation/p0-command-foundation.md)。
- [x] Git 提交与任务归档：工程 `3270443`、规范 `610d367`、归档 `73fec28`、日志 `8d59642` 已生成，远程 main 已核对同步至 `8d59642`。此切片直接落在 main，归档使用无 PR 分支校验例外；没有虚构合并或云端检查结果。

Windows race 和云端 CI 尚未执行，验收记录已分别标明；未宣称通过。此切片功能与独立验收已完成，后续继续父任务 P0 身份、授权和领域闭环，随后 P1 编排/隔离、P2 高自主 Agent。
