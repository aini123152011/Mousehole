import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { login } from './support';

test('候选 CIDR 主机位由服务端定位说明，键盘解锁后修正并保存新命令', async ({ page }) => {
  await login(page);
  await page.getByRole('link', { name: '授权范围', exact: true }).click();
  await page.getByRole('link', { name: '编写候选版本' }).click();
  await page
    .getByRole('textbox', { name: '授权标题', exact: true })
    .fill(`CIDR 合成候选 ${randomUUID().slice(0, 8)}`);
  await page.getByRole('textbox', { name: '授权依据引用', exact: true }).fill('合成字段校验依据');
  await page.getByRole('textbox', { name: '联系人', exact: true }).fill('合成联系人');
  await page.getByRole('combobox', { name: '范围类型 1' }).selectOption('cidr');
  await page.getByRole('checkbox', { name: 'DNS 名称检查', exact: true }).uncheck();
  await page.getByRole('checkbox', { name: 'TCP 连接条件', exact: true }).check();
  await page.getByRole('textbox', { name: '允许端口 1' }).fill('443');
  const input = page.getByRole('textbox', { name: '范围目标 1' });
  await input.fill('10.42.7.8/16');
  await page.getByRole('button', { name: '保存新候选版本' }).click();
  await expect(page.getByText('本次命令明确拒绝，编辑内容已保留', { exact: true })).toBeVisible();
  const rejectedId = await page.getByTestId('scope-command-id').innerText();
  await expect(input).toHaveValue('10.42.7.8/16');
  await expect(input).toHaveAccessibleDescription(/CIDR 含主机位.*10\.42\.0\.0\/16/);
  await expect(page.getByRole('alert', { name: '候选字段校验说明' })).toHaveText(
    /范围 1.*10\.42\.0\.0\/16/,
  );
  await page.getByRole('button', { name: '结束本次命令，继续编辑' }).focus();
  await page.keyboard.press('Enter');
  await input.fill('10.42.0.0/16');
  await page.getByRole('button', { name: '保存新候选版本' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText('原命令已受理', { exact: true })).toBeVisible();
  expect(await page.getByTestId('scope-command-id').innerText()).not.toBe(rejectedId);
});
