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
  CommandWorkspace: ({ actorId, project }: { actorId: string; project: { id: string } }) => (
    <div data-testid="command-workspace">
      {actorId}:{project.id}
    </div>
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

beforeEach(() => {
  apiMock.getSession.mockReset().mockResolvedValue(session);
  apiMock.logout.mockReset().mockResolvedValue(undefined);
});

describe('会话与项目缓存边界', () => {
  it('默认选中首个项目，切换时清除命令缓存并重置工作台', async () => {
    const queryClient = createQueryClient();
    queryClient.setQueryData(
      ['commands', session.actor_id, session.projects[0]?.id, 'command-id'],
      { secret: '前一项目回执' },
    );
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
      `${session.actor_id}:${session.projects[1]?.id}`,
    );
    expect(screen.getByTestId('command-workspace')).not.toBe(before);
    expect(queryClient.getQueriesData({ queryKey: ['commands'] })).toHaveLength(0);
  });

  it('退出时清除当前 actor 的全部命令缓存', async () => {
    const queryClient = createQueryClient();
    queryClient.setQueryData(['commands', session.actor_id, 'project', 'command'], {
      secret: '不得跨会话保留',
    });
    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );
    await userEvent.click(await screen.findByRole('button', { name: '退出登录' }));
    await waitFor(() => expect(apiMock.logout).toHaveBeenCalledTimes(1));
    expect(queryClient.getQueriesData({ queryKey: ['commands'] })).toHaveLength(0);
  });
});
