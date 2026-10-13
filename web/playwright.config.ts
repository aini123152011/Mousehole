import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  // 合成夹具共用项目当前策略，逐个执行，权限撤销作为依赖阶段最后运行。
  workers: 1,
  retries: 0,
  reporter: 'list',
  // Playwright trace 会收集请求头和 Cookie；本项目的诊断产物禁止保存会话秘密。
  use: { baseURL: 'http://127.0.0.1:5173', trace: 'off' },
  projects: [
    {
      name: 'chromium',
      testIgnore: '**/access-revocation.spec.ts',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'access-revocation',
      testMatch: '**/access-revocation.spec.ts',
      dependencies: ['chromium'],
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // 后端与数据库由调用者显式启动；缺依赖时真实失败，不用模拟数据掩盖。
  webServer: {
    command: 'pnpm dev',
    url: 'http://127.0.0.1:5173',
    reuseExistingServer: !process.env.CI,
  },
});
