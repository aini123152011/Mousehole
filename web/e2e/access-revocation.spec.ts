import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { adminCommand, csrfHeaders, login, requiredEnvironment } from './support';

const actorId = requiredEnvironment('MOUSEHOLE_E2E_ACTOR_ID');
const projectId = requiredEnvironment('MOUSEHOLE_E2E_PROJECT_ID');
const revokeProjectId = requiredEnvironment('MOUSEHOLE_E2E_REVOKE_PROJECT_ID');

test('管理员撤销成员权限后，当前页面、后续请求和旧 Cookie 重载均无项目访问', async ({ page }) => {
  const adminBinary = process.env.MOUSEHOLE_E2E_ADMIN_BIN;
  const databaseURL = process.env.MOUSEHOLE_DATABASE_URL;
  if (!adminBinary || !databaseURL) {
    throw new Error('权限撤销 E2E 必须显式设置管理员二进制和数据库连接。');
  }

  await login(page);
  const headers = await csrfHeaders(page);
  const sessionCookie = (await page.context().cookies()).find((cookie) =>
    cookie.name.endsWith('mousehole_session'),
  );
  if (!sessionCookie) throw new Error('真实登录未设置服务端会话 Cookie。');

  await page.getByRole('combobox', { name: '选择项目' }).selectOption(revokeProjectId);
  await adminCommand(['revoke-membership', '--user-id', actorId, '--project-id', revokeProjectId]);

  const responsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === 'PUT' &&
      response.url().includes(`/projects/${revokeProjectId}/commands/`),
  );
  await page.getByRole('textbox', { name: '任务标题' }).fill(`撤销后拒绝 ${randomUUID()}`);
  await page.getByRole('button', { name: '创建草稿' }).click();
  const rejected = await responsePromise;
  expect(rejected.status()).toBe(403);
  expect(await rejected.json()).toMatchObject({ code: 'project_forbidden' });
  await expect(page.getByRole('combobox', { name: '选择项目' })).toHaveValue(projectId);
  await expect(page.getByRole('textbox', { name: '任务标题' })).toHaveValue('');

  const direct = await page.request.put(
    `/api/v1/projects/${revokeProjectId}/commands/${randomUUID()}`,
    {
      headers,
      data: { kind: 'create_task_draft', payload: { title: '撤销后的直接请求' } },
    },
  );
  expect(direct.status()).toBe(403);
  expect(await direct.json()).toMatchObject({ code: 'project_forbidden' });

  const cookieAfterRejection = (await page.context().cookies()).find(
    (cookie) => cookie.name === sessionCookie.name,
  );
  // 布尔断言在失败时只输出 true/false，不把 Cookie 原值写入测试日志。
  expect(cookieAfterRejection?.value === sessionCookie.value).toBe(true);
  await page.reload();
  await expect(page.getByRole('combobox', { name: '选择项目' })).toHaveValue(projectId);
  await expect(page.getByRole('textbox', { name: '任务标题' })).toHaveValue('');

  const sessionAfterReload = await page.request.get('/api/v1/session');
  expect(sessionAfterReload.status()).toBe(200);
  const remaining: unknown = await sessionAfterReload.json();
  const safeSession =
    typeof remaining === 'object' &&
    remaining !== null &&
    'actor_id' in remaining &&
    remaining.actor_id === actorId &&
    'projects' in remaining &&
    Array.isArray(remaining.projects) &&
    remaining.projects.some(
      (project: unknown) =>
        typeof project === 'object' &&
        project !== null &&
        'id' in project &&
        project.id === projectId,
    ) &&
    !remaining.projects.some(
      (project: unknown) =>
        typeof project === 'object' &&
        project !== null &&
        'id' in project &&
        project.id === revokeProjectId,
    );
  expect(safeSession).toBe(true);
  const rejectedAfterReload = await page.request.put(
    `/api/v1/projects/${revokeProjectId}/commands/${randomUUID()}`,
    {
      headers,
      data: { kind: 'create_task_draft', payload: { title: '旧 Cookie 重载后的直接请求' } },
    },
  );
  expect(rejectedAfterReload.status()).toBe(403);
  expect(await rejectedAfterReload.json()).toMatchObject({ code: 'project_forbidden' });
  await expect(page.getByRole('textbox', { name: '任务标题' })).toBeVisible();
});
