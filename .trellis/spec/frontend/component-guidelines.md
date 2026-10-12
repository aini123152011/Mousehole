# React 与 shadcn 组件规范

基础组件集中 `components/ui`，记录 [来源](../../../web/src/components/ui/provenance.json) 和 MIT 许可证。shadcn 控件是项目源码，升级必须审查差异；新增注释中文，不翻译许可证或假称本地组件与上游完全相同。

业务组件具有中文 TSDoc，说明可观察行为与状态含义。Props 明确类型，复杂互斥场景使用可判别联合；原生控件 props 从 React.ComponentProps 推导。样式采用 Tailwind 与 `globals.css` token，不在页面各自复制颜色和焦点规则。

`CommandWorkspace` 组合表单与恢复查询，`CommandResult` 展示已校验回执。accepted 显示已受理，task_state=draft 显示草稿；不得表示扫描或任务执行完成。标题与错误按纯文本渲染，禁止 dangerouslySetInnerHTML。

输入必须关联可见 label；错误有 aria-describedby/invalid，状态有适度 aria-live；按钮保留可见 focus，键盘顺序和窄屏布局可用。禁用状态与说明同时表达提交锁，颜色不是唯一状态渠道。长 UUID 在窄屏换行，复制与查询不要求用户横向滚动。

表单重新可编辑后再恢复焦点，不能对仍禁用的旧控件同步调用 setFocus。跳转链接获焦时保持脱离正常布局，避免移动侧栏或正文；真实键盘路径须验证跳转后的焦点和滚动位置。界面装饰标签同样使用中文，品牌及机器标识保留原文。

基准例：[回执组件](../../../web/src/features/commands/command-result.tsx)。组件测试验证 label、状态与锁定；Playwright 验证 Tab/键盘路径、窄屏无横溢和真实受理。不为纯排版或注释编写实现镜像测试。
