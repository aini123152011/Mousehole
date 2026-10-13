import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  CommandAPIError,
  type ScopeCommandReceipt,
  type ScopeCommandRequest,
} from '../../api/client';
import { createQueryClient } from '../../app/query-client';
import { clearProtectedCache } from '../../app/protected-cache';
import { analystProject, deferred, enterpriseSession } from '../../test/fixtures';
import { precheckReceipt, scopeIds } from '../../test/scope-fixtures';
import { scopeCommandKeys, useScopeCommand } from './use-scope-command';

const mock = vi.hoisted(() => ({ acceptScopeCommand: vi.fn(), getScopeCommandReceipt: vi.fn() }));
vi.mock('../../api/client', async (original) => ({
  ...(await original<typeof import('../../api/client')>()),
  ...mock,
}));
const actorId = enterpriseSession.actor_id;
const projectId = analystProject.id;
const body: ScopeCommandRequest = {
  kind: 'precheck_target',
  payload: {
    authorization_version_id: scopeIds.version,
    operation: 'dns_lookup',
    target: 'example.invalid',
    parameters: {},
  },
};
function setup() {
  const client = createQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { ...renderHook(() => useScopeCommand(actorId, projectId), { wrapper }), client };
}
beforeEach(() => {
  mock.acceptScopeCommand.mockReset();
  mock.getScopeCommandReceipt.mockReset();
  vi.spyOn(crypto, 'randomUUID').mockReturnValue(scopeIds.command);
});

describe('范围命令恢复与请求所有权', () => {
  it('同 tick 双击仅一次 PUT；断线与 GET404 保留原 ID，不接受换参数/换ID', async () => {
    mock.acceptScopeCommand.mockRejectedValue(new TypeError('合成断线'));
    mock.getScopeCommandReceipt
      .mockRejectedValueOnce(
        new CommandAPIError(404, {
          code: 'not_found',
          message: '未找到',
          request_id: scopeIds.command,
        }),
      )
      .mockResolvedValueOnce(precheckReceipt());
    const { result } = setup();
    await act(async () => {
      void result.current.submit(body);
      await result.current.submit(body);
    });
    expect(mock.acceptScopeCommand).toHaveBeenCalledTimes(1);
    expect(result.current.phase).toBe('unknown');
    await act(async () => {
      result.current.reset();
      await result.current.lookup(scopeIds.entry);
      await result.current.submit({
        ...body,
        payload: { ...body.payload, target: 'changed.invalid' },
      });
    });
    expect(mock.getScopeCommandReceipt).not.toHaveBeenCalled();
    expect(mock.acceptScopeCommand).toHaveBeenCalledTimes(1);
    await act(async () => {
      await result.current.lookup(scopeIds.command);
    });
    expect(result.current.phase).toBe('unknown');
    expect(result.current.canReset).toBe(false);
    await act(async () => {
      await result.current.lookup(scopeIds.command);
    });
    await waitFor(() => expect(result.current.receipt).toEqual(precheckReceipt()));
    expect(mock.getScopeCommandReceipt).toHaveBeenLastCalledWith(
      projectId,
      scopeIds.command,
      expect.any(AbortSignal),
      body,
    );
    expect(result.current.phase).toBe('accepted');
  });

  it('工作台卸载取消写请求；迟到已受理响应不能复活退出后缓存', async () => {
    const pending = deferred<ScopeCommandReceipt>();
    mock.acceptScopeCommand.mockReturnValue(pending.promise);
    const { result, unmount, client } = setup();
    act(() => {
      void result.current.submit(body);
    });
    const signal: unknown = mock.acceptScopeCommand.mock.calls[0]?.[3];
    unmount();
    await clearProtectedCache(client);
    expect(signal instanceof AbortSignal && signal.aborted).toBe(true);
    await act(async () => {
      pending.resolve(precheckReceipt());
    });
    expect(
      client.getQueryData(scopeCommandKeys.detail(actorId, projectId, scopeIds.command)),
    ).toBeUndefined();
  });

  it('旧 GET 迟到不会覆盖新选择；查询失败保留原确认时刻', async () => {
    const first = deferred<ScopeCommandReceipt>();
    const second = deferred<ScopeCommandReceipt>();
    mock.getScopeCommandReceipt.mockImplementation((_project: string, id: string) =>
      id === scopeIds.command ? first.promise : second.promise,
    );
    const { result } = setup();
    act(() => {
      void result.current.lookup(scopeIds.command);
    });
    act(() => {
      void result.current.lookup(scopeIds.entry);
    });
    await act(async () => {
      second.resolve(precheckReceipt(scopeIds.entry));
    });
    await waitFor(() => expect(result.current.receipt?.command_id).toBe(scopeIds.entry));
    await act(async () => {
      first.resolve(precheckReceipt());
    });
    expect(result.current.selection?.commandId).toBe(scopeIds.entry);
    mock.getScopeCommandReceipt.mockRejectedValue(new TypeError('合成更新失败'));
    await act(async () => {
      await result.current.lookup(scopeIds.entry);
    });
    expect(result.current.notice).toContain('保留上次确认');
    expect(result.current.receipt?.accepted_at).toBe(precheckReceipt().accepted_at);
  });

  it('401 清除所有领域，403 清除同 actor/project 的领域缓存', async () => {
    const { result, client } = setup();
    for (const root of ['assets', 'authorization', 'prechecks', 'commands'])
      client.setQueryData([root, actorId, projectId, 'data'], { protected: true });
    client.setQueryData(['assets', actorId, scopeIds.version, 'other'], { other: true });
    mock.acceptScopeCommand.mockRejectedValue(
      new CommandAPIError(403, {
        code: 'forbidden',
        message: '已撤销',
        request_id: scopeIds.command,
      }),
    );
    await act(async () => {
      await result.current.submit(body);
    });
    for (const root of ['assets', 'authorization', 'prechecks', 'commands'])
      expect(client.getQueryData([root, actorId, projectId, 'data'])).toBeUndefined();
    expect(client.getQueryData(['assets', actorId, scopeIds.version, 'other'])).toBeDefined();
    act(() => result.current.reset());
    mock.acceptScopeCommand.mockRejectedValue(
      new CommandAPIError(401, {
        code: 'unauthenticated',
        message: '登录失效',
        request_id: scopeIds.command,
      }),
    );
    await act(async () => {
      await result.current.submit(body);
    });
    expect(client.getQueriesData({ queryKey: ['assets'] })).toHaveLength(0);
  });
});
