import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { expect, type Page } from '@playwright/test';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import type { components } from '../src/api/generated/schema';
import schemas from '../src/api/generated/schemas.json' with { type: 'json' };

const execFileAsync = promisify(execFile);
const validator = new Ajv2020({ strict: true, discriminator: true });
addFormats(validator);
validator.addSchema(schemas, 'mousehole');
const validateScope = validator.compile<components['schemas']['ScopeCommandReceipt']>({
  $ref: 'mousehole#/$defs/ScopeCommandReceipt',
});
const validatePolicy = validator.compile<components['schemas']['ProjectPolicy']>({
  $ref: 'mousehole#/$defs/ProjectPolicy',
});

/** 测试身份必须来自显式夹具，禁止继续使用会被撤销的固定开发成员。 */
export function requiredEnvironment(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`真实浏览器验收缺少环境字段：${name}`);
  return value;
}

/** 登录等待稳定 session 状态，再明确选择本轮随机主项目。 */
export async function login(page: Page) {
  await page.goto('/');
  const link = page.getByRole('link', { name: '登录', exact: true });
  const selector = page.getByRole('combobox', { name: '选择项目' });
  await expect(link.or(selector)).toBeVisible();
  if (await link.isVisible()) await link.click();
  await expect(selector).toBeVisible();
  await selector.selectOption(requiredEnvironment('MOUSEHOLE_E2E_PROJECT_ID'));
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeVisible();
}

/** 只向真实同源写请求传入会话 CSRF，断言失败不回显会话正文或秘密。 */
export async function csrfHeaders(page: Page) {
  const response = await page.request.get('/api/v1/session');
  expect(response.status()).toBe(200);
  const session: unknown = await response.json();
  if (
    typeof session !== 'object' ||
    session === null ||
    !('csrf_token' in session) ||
    typeof session.csrf_token !== 'string'
  )
    throw new Error('真实会话缺少 CSRF 字段。');
  return { 'X-CSRF-Token': session.csrf_token, Origin: 'http://127.0.0.1:5173' };
}

/** CLI 子进程错误含参数，必须用固定错误避免输出数据库连接和认证配置。 */
export async function adminCommand(args: string[]) {
  try {
    await execFileAsync(
      requiredEnvironment('MOUSEHOLE_E2E_ADMIN_BIN'),
      [...args, '--database-url', requiredEnvironment('MOUSEHOLE_DATABASE_URL')],
      { timeout: 20_000, windowsHide: true },
    );
  } catch {
    throw new Error('管理员合成夹具操作失败；连接信息已隐藏。');
  }
}

/** 真实响应先过生成 schema，避免浏览器测试以类型断言绕过公开合同。 */
export function scopeReceipt(value: unknown) {
  if (!validateScope(value)) throw new Error('真实范围回执不符合 OpenAPI。');
  return value;
}

/** 当前策略同样通过生成 schema，revision 按十进制字符串保留。 */
export function projectPolicy(value: unknown) {
  if (!validatePolicy(value)) throw new Error('真实项目策略不符合 OpenAPI。');
  return value;
}
