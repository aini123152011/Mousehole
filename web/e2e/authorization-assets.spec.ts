import { randomUUID } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import {
  adminCommand,
  csrfHeaders,
  login,
  projectPolicy,
  requiredEnvironment,
  scopeReceipt,
} from './support';

const projectId = requiredEnvironment('MOUSEHOLE_E2E_PROJECT_ID');
const base = `/api/v1/projects/${projectId}`;
const captures = resolve('../docs/validation/assets');

async function capture(page: Page, name: string) {
  await mkdir(captures, { recursive: true });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: resolve(captures, name), fullPage: true });
}

async function accepted(page: Page) {
  await expect(page.getByText('原命令已受理', { exact: true })).toBeVisible();
  const commandId = await page.getByTestId('scope-command-id').innerText();
  const response = await page.request.get(`${base}/scope-commands/${commandId}`);
  expect(response.status()).toBe(200);
  return scopeReceipt(await response.json());
}

async function finishCommand(page: Page) {
  await page.getByRole('button', { name: '结束本次命令，继续编辑' }).click();
}

async function activate(page: Page, versionId: string) {
  const response = await page.request.get(`${base}/policy`);
  expect(response.status()).toBe(200);
  const policy = projectPolicy(await response.json());
  await adminCommand([
    'scope',
    'activate',
    '--project-id',
    projectId,
    '--version-id',
    versionId,
    '--command-id',
    randomUUID(),
    '--expected-policy-revision',
    policy.project_revision,
    '--operator-ref',
    '浏览器合成验收',
  ]);
}

// 全部目标是合成名称，本测试只访问控制面 API / PostgreSQL / 合成 issuer。
test('中文资产预览保留错误行，键盘导入后显示规范值和本项目来源', async ({ page }) => {
  await login(page);
  await page.getByRole('link', { name: '资产', exact: true }).click();
  const suffix = randomUUID().slice(0, 8);
  const domain = `e2e-${suffix}.example.invalid`;
  const lines = page.getByRole('textbox', { name: '资产列表（每行一个）' });
  await lines.fill(`${domain.toUpperCase()}.\nhttps://bad.invalid/`);
  await page.getByRole('button', { name: '预览规范化' }).click();
  await expect(page.getByText('存在无效行，整批未受理；请修正后再次预览。')).toBeVisible();
  await expect(page.getByText('第 2 行', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '确认导入' })).toBeDisabled();
  await lines.fill(`${domain.toUpperCase()}.\n${domain}`);
  await page.getByRole('textbox', { name: '来源备注' }).fill('合成资产来源，未验证');
  await page.getByRole('button', { name: '预览规范化' }).focus();
  await page.keyboard.press('Enter');
  await expect(
    page.getByText('输入 2 行，唯一 1 条，本批重复 1 条，项目已有 0 条。'),
  ).toBeVisible();
  await page.getByRole('button', { name: '确认导入' }).focus();
  await page.keyboard.press('Enter');
  const receipt = await accepted(page);
  expect(receipt.result.type).toBe('assets_imported');
  if (receipt.result.type !== 'assets_imported') throw new Error('导入没有得到资产结果。');
  expect(receipt.result.assets[0]?.canonical_value).toBe(domain);
  const list = page.locator('.asset-record').filter({ hasText: domain });
  await expect(list).toHaveCount(1);
  await list.getByText('查看本项目来源（1 份声明）').click();
  await expect(list.getByText('备注：合成资产来源，未验证')).toBeVisible();
  await capture(page, 'p0-authorization-assets-desktop.png');
  const storage = await page.evaluate(() => ({
    local: Object.keys(localStorage).length,
    session: Object.keys(sessionStorage).length,
    url: location.href,
  }));
  expect(storage.local).toBe(0);
  expect(storage.session).toBe(0);
  expect(storage.url.includes(domain)).toBe(false);
});

test('API 来源导入超过一页，敏感搜索和非首页详情返回保留原筛选与位置', async ({ page }) => {
  await login(page);
  await page.getByRole('link', { name: '资产', exact: true }).click();
  const group = `page-${randomUUID().slice(0, 8)}.invalid`;
  const entries = Array.from({ length: 30 }, (_, index) => ({
    kind: 'domain',
    value: `host-${index}.${group}`,
  }));
  await page.getByRole('combobox', { name: '来源类型' }).selectOption('api_import');
  await page.getByRole('textbox', { name: 'API 资产 JSON' }).fill(JSON.stringify(entries));
  await page.getByRole('button', { name: '预览规范化' }).click();
  await expect(page.getByRole('button', { name: '确认导入' })).toBeEnabled();
  await page.getByRole('button', { name: '确认导入' }).click();
  await accepted(page);
  await finishCommand(page);
  await page.getByRole('textbox', { name: '规范值搜索' }).fill(group);
  await page.getByRole('button', { name: '应用筛选' }).click();
  await expect(page.locator('.asset-record')).toHaveCount(25);
  await expect(page.getByText('本页 25 条 · 总量未提供 · 还有下一页')).toBeVisible();
  expect(page.url().includes(group)).toBe(false);
  await page.getByRole('button', { name: '下一页', exact: true }).click();
  await expect(page.locator('.asset-record')).toHaveCount(5);
  const originalURL = page.url();
  await page.getByRole('link', { name: '查看来源详情' }).first().click();
  await expect(page.getByRole('heading', { name: '资产来源详情' })).toBeVisible();
  await page.getByRole('link', { name: '返回资产列表' }).click();
  await expect(page.getByRole('textbox', { name: '规范值搜索' })).toHaveValue(group);
  await expect(page.locator('.asset-record')).toHaveCount(5);
  expect(page.url()).toBe(originalURL);
  await page.getByRole('textbox', { name: '规范值搜索' }).fill(`${group}-none`);
  await page.getByRole('button', { name: '应用筛选' }).click();
  await expect(page.getByText('当前筛选下暂无记录。')).toBeVisible();
  await page.goBack();
  await expect(page.getByRole('textbox', { name: '规范值搜索' })).toHaveValue(group);
  await expect(page.locator('.asset-record')).toHaveCount(5);
  expect(page.url()).toBe(originalURL);
  await page.goForward();
  await expect(page.getByRole('textbox', { name: '规范值搜索' })).toHaveValue(`${group}-none`);
  await expect(page.getByText('当前筛选下暂无记录。')).toBeVisible();
});

test('候选正文只读、管理员激活、默认拒绝与运行时待检、当前禁止后的历史恢复', async ({
  page,
}, testInfo) => {
  await login(page);
  await page.getByRole('link', { name: '授权范围', exact: true }).click();
  await page.getByRole('link', { name: '编写候选版本' }).click();
  const domain = `scope-${randomUUID().slice(0, 8)}.example.invalid`;
  const title = `合成企业授权 ${randomUUID().slice(0, 8)}`;
  await page.getByRole('textbox', { name: '授权标题', exact: true }).fill(title);
  await page.getByRole('textbox', { name: '授权依据引用', exact: true }).fill('内部合成验收依据');
  await page.getByRole('textbox', { name: '联系人', exact: true }).fill('合成管理员');
  await page.getByRole('textbox', { name: '范围目标 1' }).fill(domain);
  await page.getByRole('checkbox', { name: '包含子域（仅域名规则可用）' }).check();
  await page.getByRole('button', { name: '保存新候选版本' }).focus();
  await page.keyboard.press('Enter');
  const saved = await accepted(page);
  if (saved.result.type !== 'authorization_version') throw new Error('候选保存缺少授权版本。');
  const version = saved.result.version;
  await page.getByRole('link', { name: '查看不可变内容' }).click();
  await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
  await expect(page.getByRole('textbox', { name: '授权标题', exact: true })).toHaveCount(0);
  await expect(page.getByText(version.content_hash, { exact: true })).toBeVisible();
  await capture(page, 'p0-authorization-version-desktop.png');
  await activate(page, version.id);
  await finishCommand(page);
  await page.getByRole('link', { name: '策略预检查', exact: true }).click();
  await expect(page.getByRole('textbox', { name: '授权版本 ID' })).toHaveValue(version.id);
  await page.getByRole('textbox', { name: '候选目标' }).fill(`bad${domain}`);
  await page.getByRole('button', { name: '检查静态条件' }).click();
  const denied = await accepted(page);
  if (denied.result.type !== 'precheck') throw new Error('预检查缺少决策。');
  expect(denied.result.decision.outcome).toBe('deny');
  await finishCommand(page);
  await page.getByRole('textbox', { name: '候选目标' }).fill(`a.${domain}`);
  await page.getByRole('button', { name: '检查静态条件' }).focus();
  await page.keyboard.press('Enter');
  const matched = await accepted(page);
  if (matched.result.type !== 'precheck') throw new Error('预检查缺少决策。');
  expect(matched.result.decision.outcome).toBe('requires_runtime_checks');
  await page.getByRole('link', { name: '查看原检查解释' }).click();
  await expect(
    page.getByRole('heading', { name: '静态规则匹配，执行条件尚未验证', exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('heading', { name: '运行时仍须复核' })).toBeVisible();
  await expect(page.getByRole('button', { name: /允许执行|开始扫描/ })).toHaveCount(0);
  await capture(page, 'p0-authorization-precheck-desktop.png');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  await capture(page, 'p0-authorization-precheck-mobile.png');
  await page.setViewportSize({ width: 1280, height: 900 });
  const policyResponse = await page.request.get(`${base}/policy`);
  const current = projectPolicy(await policyResponse.json());
  const denyFile = testInfo.outputPath('synthetic-deny.json');
  await writeFile(
    denyFile,
    JSON.stringify({
      kind: 'domain',
      value: domain,
      include_subdomains: true,
      reason: '合成紧急禁止',
    }),
    'utf8',
  );
  await adminCommand([
    'scope',
    'deny-add',
    '--project-id',
    projectId,
    '--command-id',
    randomUUID(),
    '--operator-ref',
    '浏览器合成验收',
    '--expected-policy-revision',
    current.project_revision,
    '--payload-file',
    denyFile,
  ]);
  await page.reload();
  await expect(page.getByText('当前规则已变化，下方为原检查快照')).toBeVisible();
  await page.getByRole('textbox', { name: '范围命令 ID' }).fill(matched.command_id);
  await page.getByRole('button', { name: '查询原回执' }).click();
  const recovered = await accepted(page);
  expect(recovered).toEqual(matched);
  await finishCommand(page);
  await page.getByRole('link', { name: '返回预检查历史' }).click();
  await page.getByRole('textbox', { name: '候选目标' }).fill(`a.${domain}`);
  await page.getByRole('button', { name: '检查静态条件' }).click();
  const nowDenied = await accepted(page);
  if (nowDenied.result.type !== 'precheck') throw new Error('预检查缺少决策。');
  expect(nowDenied.result.decision.outcome).toBe('deny');
  expect(nowDenied.result.decision.project_revision).not.toBe(
    matched.result.decision.project_revision,
  );
});

test('真实提交丢失响应后跨页面仍锁定原 ID，仅 GET 恢复，项目切换清除输入与命令', async ({
  page,
}) => {
  let writes = 0;
  await page.route('**/scope-commands/*', async (route) => {
    if (route.request().method() !== 'PUT') {
      await route.continue();
      return;
    }
    ++writes;
    const response = await route.fetch({ maxRetries: 0 });
    expect(response.status()).toBe(201);
    await route.abort('failed');
  });
  await login(page);
  await page.getByRole('link', { name: '资产', exact: true }).click();
  await page
    .getByRole('textbox', { name: '资产列表（每行一个）' })
    .fill(`unknown-${randomUUID().slice(0, 8)}.invalid`);
  await page.getByRole('button', { name: '预览规范化' }).click();
  await expect(page.getByRole('button', { name: '确认导入' })).toBeEnabled();
  await page.getByRole('button', { name: '确认导入' }).click();
  await expect(page.getByText('受理待确认', { exact: true })).toBeVisible();
  const commandId = await page.getByTestId('scope-command-id').innerText();
  await page.getByRole('link', { name: '策略预检查', exact: true }).click();
  await expect(page.getByTestId('scope-command-id')).toHaveText(commandId);
  await expect(page.getByRole('textbox', { name: '候选目标' })).toBeDisabled();
  await expect(page.getByRole('textbox', { name: '范围命令 ID' })).toBeDisabled();
  await page.getByRole('button', { name: '按当前 ID 查询' }).focus();
  await page.keyboard.press('Enter');
  await accepted(page);
  expect(writes).toBe(1);
  await page
    .getByRole('combobox', { name: '选择项目' })
    .selectOption(requiredEnvironment('MOUSEHOLE_E2E_REVOKE_PROJECT_ID'));
  await expect(page.getByTestId('scope-command-id')).toHaveCount(0);
  await expect(page.getByRole('textbox', { name: '候选目标' })).toHaveValue('');
  await expect(page.getByRole('textbox', { name: '候选目标' })).toBeEnabled();
  expect(page.url().includes(commandId)).toBe(false);
  const headers = await csrfHeaders(page);
  const conflict = await page.request.put(`${base}/scope-commands/${commandId}`, {
    headers,
    data: {
      kind: 'import_assets',
      payload: {
        entries: [{ kind: 'domain', value: 'different.invalid' }],
        source: 'manual',
        notes: '',
        external_ref: '',
      },
    },
  });
  expect(conflict.status()).toBe(409);
});
