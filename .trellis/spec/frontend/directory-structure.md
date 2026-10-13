# 前端目录与依赖方向

```text
web/src/
  app/                       # 路由、QueryClient 与保护缓存边界
  api/client.ts              # 唯一 HTTP 与运行时校验边界
  api/generated/             # OpenAPI 生成类型和 JSON Schema
  features/commands/         # 草稿表单、状态 Hook、回执
  features/assets/           # 导入预览、来源、项目列表与内存搜索
  features/authorization/    # 不可变版本、复制表单与当前策略
  features/prechecks/        # 静态解释与历史决策，不发起目标连接
  features/scope/            # 跨页面范围命令、分页和共享交互
  components/ui/             # Button/Input/Card/Badge 及来源记录
  lib/utils.ts               # className 合并这一职责
  styles/globals.css         # Tailwind 与统一设计 token
  test/                      # 测试环境
web/e2e/                     # Playwright 真实浏览器路径
```

文件和目录 kebab-case，组件/类型 PascalCase，函数/变量 camelCase，Hook 以 use 开头。工具入口如 main.tsx、配置名按工具要求保留。组件直接使用 API 导出的生成类型，不另维护字段副本。

依赖方向 app → features → api/ui；ui 不请求业务接口、不判断项目权限。client 不读取组件交互状态。优先特性内部实现；存在真实跨特性复用再提取业务组件，不预建空目录、万能 store 或巨大 utils。

生成目录只改输入与生成器；[生成脚本](../../../web/scripts/generate-api.mjs) 负责复现。[工作台](../../../web/src/features/commands/command-workspace.tsx) 与 [状态 Hook](../../../web/src/features/commands/use-command-workspace.ts) 是当前组合实例。
