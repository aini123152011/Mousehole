# 前端质量与中文注释

统一 UTF-8 无 BOM、LF、单个结尾换行。根 Prettier 配置：2 空格、分号、单引号、尾逗号、100 目标行宽；JSON 服从 parser。ESLint 检查 TS、Hooks、热更新与禁止 any/非空断言，Prettier 单独处理排版。生成目录仅有明确最小豁免，不扩大到整个 API 或测试。

业务组件、对外类型与 Hook 写中文 TSDoc，复杂状态说明 ID/参数绑定、失败与取消语义。简单函数一两句即可，避免每行翻译。基础控件保留许可证/生成标记，中文补充说明行为。测试说明中文，稳定机器状态英文。

```typescript
/** 按原命令查询回执；404 仍是待确认，不自动重新提交。 */
```

从根运行 `python scripts/check.py --scope web`，依次执行根/Web Prettier、ESLint、tsc、生成漂移、Vitest 和生产构建。前端依赖通过唯一 pnpm-lock.yaml 和 frozen-lockfile 安装，版本见 package.json 与根工具清单。

真实 API 已迁移并启动后运行 `pnpm --dir web e2e`。mock/组件检查证明交互逻辑，真实 E2E 证明 Vite 代理、Go、数据库贯通；二者准确区分。浏览器验收覆盖创建/恢复、响应丢失、明确冲突、键盘与窄屏。只测试风险与可观察行为，不为格式或注释新增镜像测试。

禁止提交控制台调试、关闭 Hooks 规则、把未知结果写成失败、自动重发未知 PUT、UI 乐观伪造受理、敏感 localStorage、复制手写 OpenAPI 类型。已执行与未执行检查分别登记。
