# 页面状态与服务器权威

TanStack Query 保存 session、命令回执及资产/授权/策略/预检查服务器状态；React state 保存输入、选择、phase 与 notice。ref 只负责同步交互锁与请求代次，不保存第二份服务器快照。分页 cursor/类型可进入 URL；目标搜索按 location.key 只保留在项目祖先内存。当前没有全局 Zustand 或持久化草稿需求。新增约束详见 [范围工作台合同](scope-workspace-contract.md)。

| phase    | 展示含义                | 创建下一条         |
| -------- | ----------------------- | ------------------ |
| idle     | 可填写草稿              | 可提交             |
| sending  | 正在提交，ID/参数已锁定 | 禁止               |
| unknown  | 受理待确认，查询原 ID   | 同项目提交来源禁止 |
| accepted | 回执已确认，任务为草稿  | 显式开始新草稿     |
| rejected | 已校验明确拒绝          | 显式开始新草稿     |

selection 记录 commandId、source=submission/lookup 与绑定标题或规范请求。旧/新 command Hook 由 actor/project 路由祖先持有，跨特性导航不能卸载后绕过 unknown 锁。GET 不存在不是未创建证明；前端既不清除未知 ID，也不自动 PUT。查询已有 ID 无待发送业务写入，可显式退出纯查询。

命令 Query key 必须包含 actorId、projectId 和 commandId；不能用全局 `['command']` 覆盖不同身份或对象。上次已确认回执在更新失败后仍可见，说明是上次确认记录而非实时成功。查询返回的标题在本页提交恢复时还需匹配原绑定参数。

退出、会话查询 401 和业务请求 401 清除全部保护 roots（commands/assets/authorization/prechecks/scope-commands）及内存 session；项目 `403 project_forbidden/forbidden` 清除当前 actor/project 的保护缓存并重新查询 session。同步发起取消请求、移除缓存，再等待取消完成；迟到结果不得恢复旧缓存。用户切换项目时清除保护缓存，工作台通过 `actorId:projectId` key 重新挂载，旧项目的 selection、unknown、search 和 notice 不得进入新项目。

不保存 token、Cookie、CSRF、服务器回执或敏感输入到 localStorage/sessionStorage。CSRF 只存在 session Query 与 API 客户端内存中；刷新后重新读取服务端会话，命令回执凭人工输入既有 command_id 恢复。ID 本身不表示访问授权，服务端每次重新检查项目与身份。

基准见 [Hook](hook-guidelines.md)；测试围绕单次提交、锁定、缓存隔离、未知恢复与迟到响应，不按内部 setState 调用次数编写断言。
