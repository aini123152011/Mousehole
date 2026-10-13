import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '../api/client';
import { createQueryClient } from './query-client';
import { App } from './app';

const apiMock = vi.hoisted(() => ({ getSession: vi.fn(), logout: vi.fn() }));
vi.mock('../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api/client')>()),
  ...apiMock,
}));
vi.mock('../features/commands/command-workspace', () => ({
  CommandWorkspace: ({ project }: { project: { id: string } }) => (
    <div data-testid="command-workspace">{project.id}</div>
  ),
}));

const session: Session = {
  actor_id: '10000000-0000-4000-8000-000000000001',
  display_name: '测试分析员',
  email: null,
  projects: [
    {
      id: '10000000-0000-4000-8000-000000000002',
      name: '项目一',
      role: 'analyst',
      capabilities: ['project:read', 'task_draft:create'],
      version: '1',
    },
    {
      id: '10000000-0000-4000-8000-000000000003',
      name: '项目二',
      role: 'viewer',
      capabilities: ['project:read'],
      version: '1',
    },
  ],
  csrf_token: 'test-csrf-token-value-with-at-least-32-chars',
  expires_at: '2026-10-12T09:02:03Z',
};

const protectedRoots = ['commands', 'assets', 'authorization', 'prechecks', 'scope-commands'];

beforeEach(() => {
  window.history.replaceState(null, '', '/');
  apiMock.getSession.mockReset().mockResolvedValue(session);
  apiMock.logout.mockReset().mockResolvedValue(undefined);
});

describe('会话与项目缓存边界', () => {
  it('默认选中首个项目，切换时清除全部领域缓存并重置工作台', async () => {
    const queryClient = createQueryClient();
    for (const root of protectedRoots)
      queryClient.setQueryData([root, session.actor_id, session.projects[0]?.id, 'command-id'], {
        secret: '前一项目回执',
      });
    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );
    const selector = await screen.findByRole('combobox', { name: '选择项目' });
    expect(selector).toHaveValue(session.projects[0]?.id);
    const before = screen.getByTestId('command-workspace');

    await userEvent.selectOptions(selector, session.projects[1]?.id ?? '');

    expect(screen.getByTestId('command-workspace')).toHaveTextContent(
      session.projects[1]?.id ?? '',
    );
    expect(screen.getByTestId('command-workspace')).not.toBe(before);
    for (const root of protectedRoots)
      expect(
        queryClient.getQueryData([root, session.actor_id, session.projects[0]?.id, 'command-id']),
      ).toBeUndefined();
  });

  it('退出时清除全部领域缓存', async () => {
    const queryClient = createQueryClient();
    for (const root of protectedRoots)
      queryClient.setQueryData([root, session.actor_id, 'project', 'command'], {
        secret: '不得跨会话保留',
      });
    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );
    await userEvent.click(await screen.findByRole('button', { name: '退出登录' }));
    await waitFor(() => expect(apiMock.logout).toHaveBeenCalledTimes(1));
    for (const root of protectedRoots)
      expect(
        queryClient.getQueryData([root, session.actor_id, 'project', 'command']),
      ).toBeUndefined();
  });

  it('服务端身份改变时清除旧身份领域缓存，并重新挂载项目工作台', async () => {
    const queryClient = createQueryClient();
    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );
    await screen.findByTestId('command-workspace');
    for (const root of protectedRoots)
      queryClient.setQueryData([root, session.actor_id, 'project', 'record'], { sensitive: true });
    const before = screen.getByTestId('command-workspace');
    apiMock.getSession.mockResolvedValueOnce({ ...session, actor_id: session.projects[0]?.id });
    await queryClient.refetchQueries({ queryKey: ['session'] });
    await waitFor(() => expect(screen.getByTestId('command-workspace')).not.toBe(before));
    for (const root of protectedRoots)
      expect(queryClient.getQueriesData({ queryKey: [root, session.actor_id] })).toHaveLength(0);
  });
});
