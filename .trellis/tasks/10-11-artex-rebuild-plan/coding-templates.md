# 中文注释、工程配置与交付模板

配合 [统一中文代码规范](coding-standards.md) 使用。这里提供后续可复用的注释形态和文档模板；不是新平台源码，也不证明工具已配置。模板按真实职责删减，不能保留空洞标题、虚构保证或复制与实现不一致的说明。所有 API 名称、路径、错误码和字段示例以最终 OpenAPI/领域模型为准。

## 1. Go 包文档

核心包在 `doc.go` 写一次包说明，保留 Go 文档的 `Package` 前缀，正文使用中文：

```go
// Package execution 管理 Action 与 Attempt 的执行生命周期。
//
// PostgreSQL 保存业务状态；Temporal 负责持久化编排进度。
// 本包不直接连接目标网络，也不永久保存工具输出。
// 状态变更经过授权、版本检查，并与必要审计及 outbox 原子提交。
package execution
```

简单导出函数以符号名开头，一至两句中文解释作用。复杂业务函数采用下面的形态，文档紧贴真实 `AcceptCommand` 声明；逐项内容必须由实现和测试支持：

```go
// AcceptCommand 受理执行命令，并返回可查询的受理记录。
//
// 前置条件：调用者具有项目权限，授权版本仍有效，目标已规范化。
// 幂等规则：相同命令 ID 和相同参数返回原结果；参数不同返回冲突。
// 事务边界：受理记录、业务变更、必要审计和 outbox 同时提交；
// 不在数据库事务内调用 Runner 或外部目标。
// 并发规则：按期望版本更新，竞争失败不覆盖其他调用者的结果。
// 失败语义：提交前失败不产生可执行命令；提交结果未确认时，
// 调用者保留原命令 ID 查询受理状态，不生成新 ID 重发。
```

不要强制每个函数都使用六段模板。纯格式化函数、直观访问器和短小私有函数无需重复代码能直接表达的信息。

## 2. 类型、字段与状态

字段只在有单位、并发、版本、秘密或非直观语义时展开。这个类型用于展示注释形式，最终模型应按契约定义，不从模板倒推业务字段：

```go
// CommandReceipt 表示控制面持久化的命令受理结果。
// 受理成功不代表 Runner 已执行，也不代表停止已经确认。
type CommandReceipt struct {
	CommandID string // 提交前由客户端生成的稳定标识，用于查询与幂等关联。
	Version   int64  // 业务修订号，用于并发校验；API 无损表示按契约转换。
	State     string // 命令受理/推进状态，不替代 Action 或 Attempt 状态。
}
```

枚举注释必须解释容易误读的状态，例如：

```go
// AttemptOutcomeUnknown 表示请求可能已经发出，但最终结果尚不可确认。
// 此状态不能仅凭超时转换为普通失败并自动重试。
```

TypeScript 采用中文 TSDoc/JSDoc；字段说明与 Go/OpenAPI 语义保持一致：

```ts
/** 服务端确认的命令受理记录；不代表目标执行已经完成。 */
export interface CommandReceiptView {
  /** 客户端发送前已知的稳定命令 ID。 */
  commandId: string;
  /** 十进制业务修订号，避免 int64 转成 number 后丢失精度。 */
  version: string;
  /** 来自服务端的受理状态，不能据此猜测 Runner 是否停止。 */
  state: string;
}
```

## 3. 原因、并发、重试与取消注释

在实际不变量边界写简短原因注释，不必重复完整设计：

```go
// 业务变更、受理记录和 outbox 在同一事务中提交，
// 避免数据库提交成功后进程退出导致 Workflow 命令丢失。
// dispatcher 仍可能重复投递，消费方必须按稳定命令 ID 去重。
```

```go
// 使用期望版本更新，防止迟到的 Runner 结果覆盖新状态。
// 无匹配行表示状态/版本已变化，应进入冲突分支，而不是无条件重写。
```

```go
// 停止请求只禁止新调度和新目标请求，已经发出的请求仍可能在途。
// 未收到出口撤销及 Runner 停止确认前，保留“停止中”和未知结果。
```

```ts
// 受理响应可能丢失，但服务端事务可能已经提交。
// 保留发送前生成的 commandId 查询原结果，避免新 ID 造成重复操作。
```

并发对象、外部适配器和 Workflow 需要记录这些内容；可以写在类型/函数文档或配套合同中，不在所有文件复制：

```text
责任与所有权：谁创建、读取、修改、关闭资源。
保护方式：mutex、CAS、事务锁或单一 owner；需要时写明锁顺序。
生命周期：启动、取消、错误传播和退出等待。
外部副作用：目标、权限、速率、超时与证据要求。
重放规则：是否允许自动重试，条件、上限和幂等关联。
未知结果：如何隔离旧 Runner、核对与人工介入。
兼容性：Workflow 版本/历史 replay 与升级方式。
```

## 4. React 业务 Hook 与组件

下面是 Hook 文档与查询形态示例，`taskApi` 和 `taskKeys` 由真实 client 薄适配层提供。它展示约束，不是可直接运行的模块：

```ts
/**
 * 查询任务权威快照，并纳入统一服务器数据缓存。
 *
 * @remarks
 * SSE 只触发回补或刷新，本 Hook 不根据模型叙述推断执行结果。
 * Query key 包含项目和任务身份，避免切换项目后显示旧缓存。
 *
 * @param projectId - 当前项目的稳定标识。
 * @param taskId - 当前任务的稳定标识。
 * @returns 查询状态、任务快照和重新查询方法。
 */
export function useTaskQuery(projectId: string, taskId: string) {
  return useQuery({
    queryKey: taskKeys.detail(projectId, taskId),
    queryFn: ({ signal }) => taskApi.getTask({ projectId, taskId, signal }),
  });
}
```

组件文档放在真实导出声明前，Props 定义明确数据权威和回调含义：

```ts
/** 任务概况组件的受控输入。 */
export interface TaskSummaryProps {
  /** 服务端任务快照，不包含前端推测的执行终态。 */
  task: TaskSnapshot;
  /** 最近一次同步确认时间；尚未成功同步时为 null。 */
  lastSyncedAt: string | null;
  /** 暂停命令已受理时触发，不代表出口或 Runner 已停止。 */
  onPauseAccepted: (commandId: string) => void;
}
```

```ts
/**
 * 展示任务进度、阻塞原因、覆盖缺口和可执行操作。
 *
 * @remarks
 * 分别展示任务聚合状态、停止请求状态和在途 Attempt。
 * 未收到停止确认时继续显示“停止中”，并提供未知结果明细。
 */
```

函数同步抛出或返回的 Promise 可能拒绝时，使用 `@throws` 说明真实错误类型及触发条件，并标明同步抛出或异步拒绝。Query Hook 通常通过返回状态表达请求错误，应在返回状态说明中描述，不宣称同步抛出。返回 `Result` 的函数说明成功/错误分支。业务组件 Props 若存在互斥模式，应由实际类型使用可判别联合表达，避免假装任意布尔组合都合法。

## 5. SQL 查询与迁移

查询文件保留 sqlc 固定指令，用中文解释并发与结果含义。以下仅展示说明方式，表名/字段/状态必须来自真实 schema：

```sql
-- name: CompareAndSetActionState :one
-- 仅当项目、当前状态和版本与请求匹配时更新；无返回行表示竞争失败。
-- 调用方在同一事务写入必要审计与 outbox，本查询不启动外部执行。
UPDATE actions
SET state = $1, version = version + 1
WHERE project_id = $2 AND id = $3 AND state = $4 AND version = $5
RETURNING id, state, version;
```

已发布迁移不可修改。每份迁移或关联文档包含下列中文说明；工具专用 up/down 指令在最终选型后保持其原语法：

```sql
-- 目的：增加并发版本校验所需的字段或约束。
-- 依赖：明确前置迁移及应用读写版本。
-- 兼容：先扩展 schema，再部署兼容读写，最后回填/收缩旧字段。
-- 锁与耗时：说明锁类型、大表影响以及上线前验证条件。
-- 事务：说明是否可放入事务；非事务 DDL 单独执行。
-- 验证：记录行数、约束、索引及关键查询的检查方式。
-- 回滚边界：写明可逆、需备份恢复或前向修复，不承诺恢复已删数据。
```

列级元数据采用实际 `COMMENT ON` 或权威 schema 文档，例如：

```sql
COMMENT ON COLUMN actions.version IS
  '业务修订号，每次成功状态变更递增，用于并发控制；API 使用无损表示。';
```

## 6. API 与中文错误

OpenAPI 中文 description 的模板：

```yaml
operationId: getCommandReceipt
summary: 查询命令受理结果
description: |
  按发送前生成的命令 ID 查询持久化受理记录。
  每次查询重新校验身份与对象权限。
  查询暂未找到不证明目标操作从未发生；客户端不能据此生成新 ID 重发。
```

完整端点还必须定义参数、schema、响应、权限、版本/幂等、期限和各类失败语义；这个片段只示范描述，不是完整 OpenAPI 文件。

对外错误示例：

```json
{
  "code": "version_conflict",
  "message": "任务状态已变化，请重新加载后确认操作。",
  "request_id": "req_example",
  "details": {
    "resource_type": "task"
  }
}
```

`details` 必须有允许字段 schema，不回传 SQL、堆栈、秘密或原始目标内容。英文 `code` 用于逻辑和监控，中文 `message` 用于人阅读；客户端不能匹配整句中文决定行为。版本冲突保留用户输入；未知操作不能仅提供“重新执行”按钮。

## 7. 日志与审计说明

普通结构化日志的示例形态：

```json
{
  "level": "INFO",
  "event": "command_accepted",
  "message": "执行命令已受理，等待工作流处理。",
  "request_id": "req_example",
  "command_id": "cmd_example",
  "project_id": "project_example",
  "duration_ms": 12
}
```

字段根据真实记录选择，不把所有 ID 强塞到每条日志。生产日志同时由统一 logger 记录时间等元数据。日志示例不包含完整参数/目标响应，也不替代业务审计。关键审计事件在权威 schema 中说明 actor、授权/策略版本、对象引用、命令与决定、事务边界、保留/访问权限；WORM checkpoint 与签名证明的边界沿用设计。

## 8. 测试场景模板

Go 函数名使用英文稳定标识；场景说明使用中文。以下是测试说明骨架，真实实现必须有实际安排、操作和断言，不能直接将空函数计为测试：

```go
func TestAcceptCommand_SameKeyDifferentPayload(t *testing.T) {
	// 准备：同一项目已有命令受理记录，固定 command_id 和参数摘要。
	// 操作：使用相同 command_id 提交不同参数。
	// 预期：返回稳定冲突类别，原记录不变，不追加第二个执行 outbox。
}
```

前端用例说明与可观察行为对应，不锁死 class、内部 state 或无意义快照：

```ts
describe('任务暂停命令', () => {
  it('受理响应丢失后按原命令 ID 查询，确认前仍显示停止中', async () => {
    // 准备：模拟服务端已受理，但提交响应未到达；后续查询返回原记录。
    // 操作：通过用户可见入口请求暂停并等待状态恢复。
    // 预期：原命令 ID 保持不变，不重复执行；未确认停止前不显示已停止。
  });
});
```

重要测试在正文或测试文档中说明：依赖、合成数据、故障注入、可观察结果、清理与未覆盖项。必需集成 CI 缺依赖应失败；本地 skip 不能成为“验证通过”。Go 并发例还需验证有效 lease/CAS 单获胜者及审计/outbox 一致性。

## 9. 配置模板与格式检查

以下配置在新仓库初始化时按最终工具支持情况落地，本次不写入 Mousehole 或 ARTEX 的工程根目录。

`.editorconfig` 示例：

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = tab
tab_width = 8
```

`.gitattributes` 示例，二进制文件的最终清单按真实仓库补充：

```gitattributes
* text=auto eol=lf
*.png -text
*.jpg -text
*.pdf -text
*.zip -text
```

Prettier 配置示例：

```json
{
  "printWidth": 100,
  "tabWidth": 2,
  "useTabs": false,
  "semi": true,
  "singleQuote": true,
  "trailingComma": "all"
}
```

格式器的输入范围使用明确 globs/ignore；`backend/**/*.go` 和 SQL 不交给 Prettier 格式化。ESLint 通过 Prettier 兼容配置关闭冲突排版规则，并按当前 ESLint/TS/React 版本配置正确性规则。生成 client、sqlc、第三方原文的排版例外必须精确，业务手写包装层不能一起忽略。

`web/package.json` 的拟定脚本片段：

```json
{
  "scripts": {
    "format": "prettier --write .",
    "format:check": "prettier --check .",
    "lint": "eslint .",
    "typecheck": "tsc --noEmit -p tsconfig.app.json",
    "test": "vitest run",
    "test:e2e": "playwright test",
    "build": "vite build"
  }
}
```

路径和 scripts 是建议；若最终 Vite 骨架采用 project references，类型检查改为实际适用的 `tsc -b` 或逐项目检查，并覆盖 app 与工具配置。OpenAPI 生成脚本在确定生成器/语言版本后写出精确命令，避免伪造未选型工具。Go 在 `backend/` 候选检查为 `go vet ./...`、`go test ./...`、锁定版本的 `golangci-lint run`；相关包按需运行 race。生成漂移、SQL/migration、Temporal replay 和真实依赖集成另有专门脚本，不能只跑这些通用命令就宣称完整通过。

## 10. 模块 README 与 ADR

模块 README 保留这些中文内容，简单模块可合并相邻段落：

```markdown
# 模块名称

## 职责与边界

说明服务对象、数据权威、不能承担的职责及依赖方向。

## 接口、数据与不变量

链接权威 schema/状态机，说明版本、幂等、事务与未知结果。

## 运行与配置

列出实际依赖、默认值、单位、秘密引用、生效/重启方式。

## 本地验证

写出可运行命令、合成数据准备、环境要求和成功证据。

## 故障与恢复

说明停止确认、在途/未知、reconcile、证据核对及恢复边界。

## 兼容与变更

链接迁移、Workflow 版本、来源/生成规则和相关 ADR。
```

ADR 文件建议 `NNNN-english-decision-name.md`，正文模板：

```markdown
# ADR-NNNN：中文决策标题

状态：拟议 / 已采用 / 已替代

## 问题与约束

给出源码、需求或测量依据，区分事实与假设。

## 决策与备选方案

描述选择和理由，说明备选方案的实际取舍。

## 影响与验证

列出兼容、运维和数据边界，写明验证证据及未知项。

## 复核条件

说明哪些负载、需求或故障证据会触发重新选择。
```

## 11. PR 与审查模板

PR 使用中文说明最终行为，验证分为已执行与未执行。以下用于将来业务提交，纯文档提交按需删减：

```markdown
具体问题与结果：描述触发条件、原行为和变更后的可观察行为。

变更范围：模块与契约变化；说明迁移、Workflow/生成结果及文档更新。

验证：实际命令、环境、通过/失败证据；列出未执行项目及原因。

兼容与风险：说明部署顺序、回滚边界、未知项及需要复核的影响。

关联：需求/验收编号、任务、ADR 或跟踪问题。
```

审查者按变更相关性使用清单；无关项目写“不适用”及简短原因，不机械要求全部业务提交跑全部套件：

- [ ] 手写注释、文档和用户说明为中文，机器标识与工具语法稳定。
- [ ] 导出合同和重要字段说明完整；原因、并发、副作用与失败语义真实。
- [ ] 格式、命名、模块边界、类型和生成内容符合统一配置。
- [ ] 权限/授权在服务端复核，未知能力拒绝，审批不能扩大执行边界。
- [ ] 版本冲突、幂等、事务/outbox、取消请求与确认、未知结果均未混淆。
- [ ] 证据/发现/报告版本与来源可追溯，日志/错误不泄露秘密或原始内容。
- [ ] 测试验证行为和失败路径；跳过与未执行检查已如实说明。
- [ ] 界面状态、键盘/焦点、SSE 恢复、批量范围和冲突输入符合合同。
- [ ] 迁移/Workflow 升级、生成器、来源许可与恢复文档同步。
- [ ] 未使用“有注释”“配置存在”“测试被跳过”代替正确性证明。

## 12. 开发任务交接模板

后续开发和 AI 协作每次交接包含足够上下文；新任务按实际完成情况填写，不能预填“通过”：

```text
任务目标：本次要实现的可观察结果，对应需求/验收编号。
负责范围：允许修改的模块/文件，明确协作者所有权。
必读合同：领域设计、中文规范、相关真实 spec 和研究证据。
关键约束：授权、事务、幂等、重放、取消、证据和权限边界。
中文要求：注释、文档、提示与测试意图；英文机器契约不翻译。
验证要求：与变更相称的准确命令、环境和场景。
交付说明：实际改动、验证结果、未执行项及兼容/风险。
```

与其他协作者共享工作区时保留他人改动，禁止因与自身假设不同就回退。开发和审查的上下文同时引用规范及模板，避免只在主对话中口头约定中文风格。
