# 前端质量与中文注释

统一 UTF-8 无 BOM、LF、单个结尾换行。根 Prettier 配置：2 空格、分号、单引号、尾逗号、100 目标行宽；JSON 服从 parser。ESLint 检查 TS、Hooks、热更新与禁止 any/非空断言，Prettier 单独处理排版。生成目录仅有明确最小豁免，不扩大到整个 API 或测试。

业务组件、对外类型与 Hook 写中文 TSDoc，复杂状态说明 ID/参数绑定、失败与取消语义。简单函数一两句即可，避免每行翻译。基础控件保留许可证/生成标记，中文补充说明行为。测试说明中文，稳定机器状态英文。

```typescript
/** 按原命令查询回执；404 仍是待确认，不自动重新提交。 */
```

从根运行 `python scripts/check.py --scope web`，依次执行根/Web Prettier、ESLint、tsc、生成漂移、Vitest 和生产构建。前端依赖通过唯一 pnpm-lock.yaml 和 frozen-lockfile 安装，版本见 package.json 与根工具清单。

真实 API、PostgreSQL 和合成 OIDC issuer 已启动后运行 `pnpm --dir web e2e`。mock/组件检查证明交互逻辑，真实 E2E 证明登录、Cookie 会话、Vite 代理、Go 和数据库贯通；二者准确区分。浏览器验收覆盖登录/退出、创建/恢复、响应丢失、明确冲突、权限撤销、键盘与窄屏，并断言 URL 与 Web Storage 不含协议秘密。Playwright trace 会保存请求头和 Cookie，身份流程不得生成或上传未脱敏 trace；涉及 Cookie、CSRF、DSN 的断言和子进程错误只输出固定说明或布尔结果。只测试风险与可观察行为，不为格式或注释新增镜像测试。

统一入口 `python scripts/verify_authorization_e2e.py --environment-file <显式环境映射>` 负责服务生命周期和本轮随机双项目；分页验收必须创建超过一页数据，检查无缺漏/重复、cursor 与搜索历史位置匹配。新旧 unknown 都要跨路由验证原 ID/参数与仅一次 PUT。成员撤销最后在独立项目执行，重跑换新夹具，不恢复 revoked 状态。截图仅合成数据，须人工核对中文说明、焦点和窄屏。

E2E 登录 helper 必须先等待 session 加载后的“登录或工作台”任一稳定状态，再决定是否点击登录。禁止在 `page.goto` 后立即用 `isVisible()` 分支；它不等待 React 首次渲染，会让测试根本没有发起 OIDC 流程。

```typescript
import { expect, type Page } from '@playwright/test';

async function login(page: Page) {
  await page.goto('/');
  const loginLink = page.getByRole('link', { name: '登录' });
  const workspace = page.getByRole('textbox', { name: '任务标题' });
  await expect(loginLink.or(workspace)).toBeVisible();
  if (await loginLink.isVisible()) await loginLink.click();
  await expect(workspace).toBeVisible();
}
```

禁止提交控制台调试、关闭 Hooks 规则、把未知结果写成失败、自动重发未知 PUT、UI 乐观伪造受理、敏感 localStorage、复制手写 OpenAPI 类型。已执行与未执行检查分别登记。
