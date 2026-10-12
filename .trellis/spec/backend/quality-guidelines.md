# 后端质量与中文注释

手写文本统一 UTF-8 无 BOM、LF、单个结尾换行与无尾空白。Go 用 gofmt/goimports，不人为对齐空格。英文标识符、生成器头、许可证与机器语法保留。手写导出符号注释以符号名开头，核心包写 doc.go；注释说明职责、输入约束、原子范围、未知结果和失败恢复，避免逐行翻译代码。

```go
// Accept 受理草稿命令，created 区分首次受理与原回执重放。
//
// 四类记录同事务提交。提交失败可能结果未知，调用方应保留原 ID 查询。
```

context 为第一参数并传播到数据库；请求路径不任意换为 Background。独立回滚清理上下文必须有超时并说明原因。领域错误保留因果链，责任边界记录一次安全日志。测试导出函数遵循 Go 工具命名，场景、意图与失败定位说明中文。

禁止手改 sqlc、依赖进程锁保证幂等、吞掉迁移失败、测试 skip 缺数据库、在事务中做目标网络调用、通过 TODO 代替已承诺行为。注释不能承诺外部 exactly-once 或生产鉴权已完成。

根目录运行：

```powershell
$env:MOUSEHOLE_TEST_DATABASE_URL = 'postgres://mousehole:mousehole-local-only@127.0.0.1:55432/mousehole_test?sslmode=disable'
python scripts/check.py --scope backend --integration
```

入口检查固定 Go 版本、gofmt/goimports、sqlc 文件集合与字节漂移、go mod verify、go vet、单元和真实 PG 集成。Linux CI 增加 `--race`；Windows 缺 C 工具链时如实登记。检查通过后不机械重复，只有代码修复或新风险才补跑相关检查。
