import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';

const projectId = '00000000-0000-4000-8000-000000000002';
const headers = { 'X-Mousehole-Development': 'true', Origin: 'http://127.0.0.1:5173' };

// 此文件要求真实 API 和 PostgreSQL；只写入合成草稿，不探测任何目标。
test('真实创建、同 ID 回放与冲突、刷新后找回持久化回执', async ({ page, request }, testInfo) => {
  const title = `端到端草稿 ${randomUUID()}`;
  await page.goto('/');
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('desktop-empty.png'), fullPage: true });
  await page.getByRole('textbox', { name: '任务标题' }).fill(`\u0085 ${title} \u3000`);
  await page.getByRole('button', { name: '创建草稿' }).click();
  await expect(page.getByRole('heading', { name: '已受理', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
  const commandId = await page.getByTestId('current-command-id').innerText();
  const path = `/api/v1/projects/${projectId}/commands/${commandId}`;
  const original = await request.get(path, { headers });
  expect(original.status()).toBe(200);
  const receipt: unknown = await original.json();
  if (
    typeof receipt !== 'object' ||
    receipt === null ||
    !('task_id' in receipt) ||
    typeof receipt.task_id !== 'string'
  ) {
    throw new Error('真实回执缺少任务 ID。');
  }
  const replay = await request.put(path, {
    headers,
    data: { kind: 'create_task_draft', payload: { title } },
  });
  expect(replay.status()).toBe(200);
  expect(await replay.json()).toEqual(receipt);
  const conflict = await request.put(path, {
    headers,
    data: { kind: 'create_task_draft', payload: { title: `${title}（不同参数）` } },
  });
  expect(conflict.status()).toBe(409);
  expect(await conflict.json()).toMatchObject({ code: 'idempotency_conflict' });
  await page.reload();
  await page.getByRole('textbox', { name: '已有命令 ID' }).fill(commandId);
  await page.getByRole('button', { name: '查询回执' }).click();
  await expect(page.getByRole('heading', { name: '已受理', exact: true })).toBeVisible();
  await expect(page.getByText(receipt.task_id, { exact: true })).toBeVisible();
  // 整页截图从页顶取景，避免当前滚动位置改变固定导航的截图位置。
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: testInfo.outputPath('desktop-recovered.png'), fullPage: true });
});

test('服务端已提交但浏览器响应丢失时仅 PUT 一次，通过原 ID GET 恢复', async ({ page }) => {
  let putCount = 0;
  let committed = false;
  await page.route('**/api/v1/projects/*/commands/*', async (route) => {
    if (route.request().method() !== 'PUT') {
      await route.continue();
      return;
    }
    putCount += 1;
    // 先等待真实提交完成，再制造浏览器侧响应丢失，不能用假回执代替数据库。
    const response = await route.fetch({ maxRetries: 0 });
    expect(response.status()).toBe(201);
    committed = true;
    await route.abort('failed');
  });
  await page.goto('/');
  await page.getByRole('textbox', { name: '任务标题' }).fill(`响应丢失恢复 ${randomUUID()}`);
  await page.getByRole('button', { name: '创建草稿' }).click();
  await expect(page.getByRole('heading', { name: '受理待确认', exact: true })).toBeVisible();
  expect(committed).toBe(true);
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeDisabled();
  await expect(page.getByRole('textbox', { name: '已有命令 ID' })).toBeDisabled();
  await expect(page.getByRole('button', { name: '新建另一份草稿' })).toHaveCount(0);
  const commandId = await page.getByTestId('current-command-id').innerText();
  await page.getByRole('button', { name: '查询原命令' }).click();
  await expect(page.getByRole('heading', { name: '已受理', exact: true })).toBeVisible();
  await expect(page.getByTestId('current-command-id')).toHaveText(commandId);
  expect(putCount).toBe(1);
});

test('查询不存在的原 ID 保留待确认说明，不创建新草稿', async ({ page }) => {
  let putCount = 0;
  page.on('request', (request) => {
    if (request.method() === 'PUT') putCount += 1;
  });
  await page.goto('/');
  await page.getByRole('textbox', { name: '已有命令 ID' }).fill(randomUUID());
  await page.getByRole('button', { name: '查询回执' }).click();
  await expect(page.getByText(/暂未查询到回执，受理状态仍待确认/)).toBeVisible();
  await expect(page.getByRole('button', { name: '查询原命令' })).toBeEnabled();
  await page.getByRole('button', { name: '返回新建草稿' }).click();
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeFocused();
  expect(putCount).toBe(0);
});

test('手机布局无横向溢出，跳转链接、表单错误和焦点可由键盘使用', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeVisible();
  const navigation = page.getByRole('complementary', { name: '项目导航' });
  const navigationBounds = await navigation.boundingBox();
  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: '跳到工作台' })).toBeFocused();
  expect(await navigation.boundingBox()).toEqual(navigationBounds);
  await page.keyboard.press('Enter');
  await expect(page.locator('#main-content')).toBeFocused();
  await page.getByRole('button', { name: '创建草稿' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('alert')).toHaveText('请输入任务标题。');
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeFocused();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: testInfo.outputPath('mobile-validation.png'), fullPage: true });
});
