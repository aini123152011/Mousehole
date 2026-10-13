# 后端目录与依赖方向

当前源码以 `backend/go.mod` 为单独 Go 模块。只创建当前行为需要的模块，不为 Temporal、Runner 或身份服务预建空实现。

```text
backend/
  cmd/api/                   # 配置、连接池、服务生命周期
  cmd/admin/                 # 身份预置/撤销、范围变更与原管理回执查询
  cmd/migrate/               # 显式 up/status 迁移入口
  internal/httpapi/          # 严格 HTTP 边界，消费 Commands/Identities/Scopes 接口
  internal/command/          # 领域输入、访问限制、事务受理及查询
  internal/identity/         # OIDC、会话与受控身份管理
  internal/access/           # user/project/membership 统一权限行锁
  internal/target/           # target-v1 纯规范化与匹配，无目标 I/O
  internal/validation/       # 白名单字段路径及受控中文原因
  internal/asset/            # 导入、来源、项目关联与预览
  internal/authorization/    # 不可变授权正文、hash、状态与禁止规则
  internal/policy/           # 固定目录的纯静态解释，不发执行令牌
  internal/scopecommand/     # 范围账本、事务、当前策略与签名分页
  internal/store/sqlc/       # 生成数据库访问代码
  db/migrations/             # 编号 SQL 与嵌入资源
  db/queries/                # sqlc 查询输入
```

依赖方向为入口 → httpapi → command/identity/scopecommand → 领域模块/sqlc/pgx。最小业务接口由消费方 httpapi 定义，便于 HTTP 测试替身；领域不能导入 httpapi。事务协调在服务受理入口，handler 不直接执行 SQL。scopecommand 负责跨领域提交；target/policy 的规范化与判定不引用网络执行或 Runner。

包名小写英文，Go 文件使用 `snake_case.go`，测试邻近实现并以 `_test.go` 结尾。导出名遵循 `ID`、`HTTP` 等惯用缩写。核心手写包具有 `doc.go`，以 `Package 包名` 开头，用中文解释责任与不变量。生成包保留生成器注释。

例子：[事务服务](../../../backend/internal/command/service.go)、[传输接口](../../../backend/internal/httpapi/server.go)。通用 `utils` 包、进程全局数据库变量、为了测试扩展生产后门均禁止。集成测试用构建标签和自己的 schema 隔离，不改变生产服务签名来注入任意网络行为。
