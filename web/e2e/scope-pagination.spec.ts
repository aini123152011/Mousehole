import { randomUUID } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
import type { AuthorizationContent, ScopeCommandRequest } from '../src/api/client';
import { csrfHeaders, login, requiredEnvironment, scopeReceipt } from './support';

const projectId = requiredEnvironment('MOUSEHOLE_E2E_PROJECT_ID');
const base = `/api/v1/projects/${projectId}`;

/** 通过真实受保护 API 种足分页样本；浏览器仍读取普通项目列表，不添加测试专用接口。 */
async function writeCommand(page: Page, body: ScopeCommandRequest) {
  const response = await page.request.put(`${base}/scope-commands/${randomUUID()}`, {
    headers: await csrfHeaders(page),
    data: body,
  });
  expect(response.status()).toBe(201);
  return scopeReceipt(await response.json());
}

async function listedIds(page: Page, name: string) {
  const paths = await page
    .getByRole('list', { name })
    .getByRole('link')
    .evaluateAll((links) => links.map((link) => link.getAttribute('href') ?? ''));
  return paths.map((path) => path.split('?')[0]?.split('/').at(-1) ?? '');
}

test('授权与预检查分别超过一页，详情返回保持 cursor，页间没有重漏', async ({ page }) => {
  test.setTimeout(90_000);
  await login(page);
  const content: AuthorizationContent = {
    title: `分页合成候选 ${randomUUID().slice(0, 8)}`,
    basis_ref: '分页合成验收依据',
    contact: '合成联系人',
    valid_from: new Date(Date.now() - 60_000).toISOString(),
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
    algorithm_version: 'target-v1',
    policy_version: 'policy-v1',
    catalog_version: 'precheck-v1',
    max_operation_level: 'non_destructive',
    budget: { max_requests: '100', max_concurrency: '2', max_requests_per_second: '1' },
    redirect_policy: 'deny',
    credential_policy: 'none',
    entries: [
      {
        kind: 'domain',
        value: 'pagination.example.invalid',
        include_subdomains: false,
        operations: ['dns_lookup'],
        ports: [],
        resolved_ip_cidrs: [],
      },
    ],
  };
  const versionIds = new Set<string>();
  const decisionIds = new Set<string>();
  for (let index = 0; index < 26; index++) {
    const saved = await writeCommand(page, {
      kind: 'create_authorization_version',
      payload: { ...content, title: `${content.title} ${index + 1}` },
    });
    if (saved.result.type !== 'authorization_version') throw new Error('分页夹具未保存候选。');
    versionIds.add(saved.result.version.id);
    const checked = await writeCommand(page, {
      kind: 'precheck_target',
      payload: {
        authorization_version_id: saved.result.version.id,
        operation: 'dns_lookup',
        target: 'pagination.example.invalid',
        parameters: {},
      },
    });
    if (checked.result.type !== 'precheck') throw new Error('分页夹具未保存决策。');
    // 候选不产生授权；此夹具只验证列表，不激活或连接目标。
    expect(checked.result.decision.outcome).toBe('deny');
    decisionIds.add(checked.result.decision.id);
  }
  for (const [section, listName, detailHeading, returnName, seeded] of [
    ['授权范围', '授权版本列表', content.title, '返回授权列表', versionIds],
    ['策略预检查', '预检查历史列表', '静态规则拒绝', '返回预检查历史', decisionIds],
  ] as const) {
    await page.getByRole('link', { name: section, exact: true }).click();
    await expect(page.getByRole('list', { name: listName }).getByRole('link')).toHaveCount(25);
    const first = await listedIds(page, listName);
    await page.getByRole('button', { name: '下一页', exact: true }).click();
    await expect(page.getByRole('button', { name: '回到首页' })).toBeEnabled();
    await expect(page.getByRole('button', { name: '下一页', exact: true })).toBeDisabled();
    const second = await listedIds(page, listName);
    expect(second.length).toBeGreaterThan(0);
    expect(new Set([...first, ...second]).size).toBe(first.length + second.length);
    const combined = new Set([...first, ...second]);
    for (const id of seeded) expect(combined.has(id)).toBe(true);
    const listURL = page.url();
    await page.getByRole('list', { name: listName }).getByRole('link').first().click();
    await expect(
      page.getByRole('heading', {
        name: section === '授权范围' ? new RegExp(detailHeading) : detailHeading,
      }),
    ).toBeVisible();
    await page.getByRole('link', { name: returnName, exact: true }).click();
    expect(page.url()).toBe(listURL);
    await expect(page.getByRole('list', { name: listName }).getByRole('link')).toHaveCount(
      second.length,
    );
    expect(await listedIds(page, listName)).toEqual(second);
    await page.getByRole('button', { name: '回到首页' }).click();
    await expect(page.getByRole('list', { name: listName }).getByRole('link')).toHaveCount(25);
    expect(await listedIds(page, listName)).toEqual(first);
  }
});
