# 本地开发与验证工具合同

## 1. 范围与触发

适用于根 scripts、deploy、格式配置和 CI。固定版本见 [toolchain.json](../../../scripts/toolchain.json)，实例见 [README](../../../README.md)。不全局安装、不自动读取 .env、不修改永久 PATH。

## 2. 签名

`python scripts/dev.py database|migrate|migration-status|api|web`；`python scripts/check.py --scope all|backend|web|text [--integration] [--race] [--e2e]`；`python scripts/verify_restart.py`。

## 3. 合同

API/迁移必须显式 `MOUSEHOLE_MODE=development` 与 `MOUSEHOLE_DATABASE_URL`。默认监听 127.0.0.1:8080，Web Origin 为 http://127.0.0.1:5173；只接受 loopback IP 和有效端口，Origin 不含路径、凭据、查询、片段或通配符。空查询 `?` 和空片段 `#` 也要拒绝；不能只依赖 URL 解析后字段，因为解析器可能丢失分隔符。Host 精确匹配配置，不信任转发头。健康接口只报告进程存活。

Compose 的 PostgreSQL 17 镜像固定 digest，端口只发布到 127.0.0.1:55432。database 保留已有卷，按需创建固定 mousehole_test。Go 从 `.tools/go版本/go/bin` 优先定位，再查 PATH，核对实际版本；Node/pnpm 同样精确核对。subprocess 使用参数数组，禁止字符串 shell 拼接。

check 不覆写生成目录：sqlc 临时副本生成、Web 临时生成对比。integration 缺 DSN 失败，e2e 需已迁移的 8080 API，Playwright 拥有自己的 5173 Web 进程。verify_restart 在随机端口启动其拥有的 API 两次，清理本检查进程并保留合成验收记录。

## 4. 校验与错误矩阵

| 情况                          | 结果                           |
| ----------------------------- | ------------------------------ |
| 模式未设、非 loopback、缺 DSN | 明确失败，不降级生产模式       |
| 工具缺失/版本不同             | 检查失败，不偷偷安装或跳过     |
| 文本/生成漂移                 | 失败，修改输入后重新生成与审查 |
| --integration 缺测试 DSN      | 失败，不 skip                  |
| 浏览器或真实 API 不可用       | E2E 失败，记录受阻项           |

## 5. 好、基准与坏例

好例：工作区工具经 SHA256 核对，当前终端设置网络代理但保留 Go checksum。基准：README 三终端启动数据库/API/Web。坏例：全局改 PATH、启动 API 时迁移、把 CI 文件存在写成云端通过。

## 6. 测试断言

py_compile 验证脚本语法；check 验证版本、文本、生成漂移和行为；重启逐字段一致。Linux CI 的 race 与云端运行是独立验证项，未执行时必须明确标注。

## 7. 错误与正确

错误：`subprocess.run("go " + user_value, shell=True)`。正确：`run([go, "test", "-count=1", "./..."], backend)`。开发脚本只操作工程固定数据库与自己拥有的进程。
