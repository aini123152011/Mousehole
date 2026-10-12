# shadcn/ui 基础控件来源

Button、Input、Card、Badge 源自 shadcn/ui 官方 `new-york-v4` registry，于 2026-10-12 引入。精确下载地址和 registry JSON 的 SHA-256 保存在 [provenance.json](provenance.json)，MIT 许可原文保存在 [LICENSE.shadcn](LICENSE.shadcn)。这些文件是纳入本项目维护的源码，参加格式、lint 和类型检查。

本地调整：将 `@/lib/utils` 改为相对路径；Slot 使用精确锁定的 `@radix-ui/react-slot`；不导出当前未使用的样式变体，满足 React Refresh 导出约束；增加中文职责说明并按根 Prettier 格式排版。设计 token 和页面密度统一放在 `src/styles/globals.css`。

基础控件只负责组合和展示，不发业务请求、不判断身份或授权。标签、错误说明、忙碌状态与受理语义由业务组件提供；新增业务属性时必须写中文 JSDoc。

升级时先取得官方 registry 文件，核对来源与许可证，对比本地差异，再运行 `pnpm run lint`、`pnpm run typecheck` 和相关交互测试。更新此说明和 provenance；不能用“生成源码”理由跳过自持改动审查。
