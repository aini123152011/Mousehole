# 页面状态与服务器权威

TanStack Query 保存 session 和命令回执；React state 保存输入、选择、phase 与 notice。ref 只负责同步交互锁与请求代次，不保存第二份服务器快照。当前没有全局 Zustand、URL 状态或持久化草稿需求，禁止为预测需求引入。

| phase    | 展示含义                | 创建下一条       |
| -------- | ----------------------- | ---------------- |
| idle     | 可填写草稿              | 可提交           |
| sending  | 正在提交，ID/参数已锁定 | 禁止             |
| unknown  | 受理待确认，查询原 ID   | 本页提交来源禁止 |
| accepted | 回执已确认，任务为草稿  | 显式开始新草稿   |
| rejected | 已校验明确拒绝          | 显式开始新草稿   |

selection 记录 commandId、source=submission/lookup 与本页绑定标题。GET 不存在不是未创建证明；前端既不清除未知 ID，也不自动 PUT。查询已有 ID 无待发送业务写入，可显式退出纯查询。

Query key 必须包含 projectId 和 commandId；不能用全局 ['command'] 覆盖不同对象。上次已确认回执在更新失败后仍可见，说明是上次确认记录而非实时成功。查询返回的标题在本页提交恢复时还需匹配原绑定参数。

不保存 token、凭据、服务器回执或敏感输入到 localStorage。刷新后凭人工输入既有 command_id 恢复；ID 本身不表示访问授权，服务端每次重新检查项目与身份。

基准见 [Hook](hook-guidelines.md)；测试围绕单次提交、锁定、缓存隔离、未知恢复与迟到响应，不按内部 setState 调用次数编写断言。
