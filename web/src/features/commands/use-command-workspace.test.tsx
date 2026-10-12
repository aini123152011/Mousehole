import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CommandReceipt } from '../../api/client';
import { CommandAPIError } from '../../api/client';
import { createQueryClient } from '../../app/query-client';
import { deferred, developmentSession, receiptFor } from '../../test/fixtures';
import { commandKeys, useCommandWorkspace } from './use-command-workspace';

const apiMock = vi.hoisted(() => ({ acceptCommand: vi.fn(), getCommandReceipt: vi.fn() }));
vi.mock('../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/client')>()),
  ...apiMock,
}));

const projectId = developmentSession.project_id;
const firstId = '00000000-0000-4000-8000-000000000003';
const secondId = '00000000-0000-4000-8000-000000000004';

function setup() {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { ...renderHook(() => useCommandWorkspace(projectId), { wrapper }), queryClient };
}

beforeEach(() => {
  apiMock.acceptCommand.mockReset();
  apiMock.getCommandReceipt.mockReset();
  vi.spyOn(crypto, 'randomUUID').mockReturnValue(firstId);
});

describe('命令交互与服务器缓存', () => {
  it('提交响应丢失及 GET 404 均锁定原 ID 和参数，查询恢复后才允许新建', async () => {
    apiMock.acceptCommand.mockRejectedValue(new TypeError('合成响应丢失'));
    apiMock.getCommandReceipt
      .mockRejectedValueOnce(
        new CommandAPIError(404, {
          code: 'command_not_found',
          message: '暂未找到命令',
          request_id: firstId,
        }),
      )
      .mockResolvedValueOnce(receiptFor(firstId, '标题'));
    const { result, queryClient } = setup();
    await act(async () => {
      await result.current.createDraft(' \u0085标题 ');
    });
    expect(result.current.phase).toBe('unknown');
    expect(result.current.selection).toEqual({
      commandId: firstId,
      source: 'submission',
      title: '标题',
    });
    await act(async () => {
      result.current.startNewDraft();
      await result.current.createDraft('其他标题');
      await result.current.lookup(secondId);
    });
    expect(apiMock.acceptCommand).toHaveBeenCalledTimes(1);
    expect(apiMock.getCommandReceipt).not.toHaveBeenCalled();
    await act(async () => {
      await result.current.lookup(firstId);
    });
    expect(result.current.phase).toBe('unknown');
    expect(result.current.canStartNew).toBe(false);
    expect(result.current.notice?.message).toContain('仍待确认');
    await act(async () => {
      await result.current.lookup(firstId);
    });
    expect(result.current.phase).toBe('accepted');
    expect(result.current.receipt).toEqual(receiptFor(firstId, '标题'));
    expect(queryClient.getQueryData(commandKeys.detail(projectId, firstId))).toEqual(
      receiptFor(firstId, '标题'),
    );
    expect(apiMock.getCommandReceipt).toHaveBeenLastCalledWith(
      projectId,
      firstId,
      expect.any(AbortSignal),
      '标题',
    );
  });

  it('同步交互锁阻止下一次渲染前重复提交和切换命令', async () => {
    const pending = deferred<CommandReceipt>();
    apiMock.acceptCommand.mockReturnValue(pending.promise);
    const { result } = setup();
    act(() => {
      void result.current.createDraft('标题');
      void result.current.createDraft('重复标题');
      void result.current.lookup(secondId);
      result.current.startNewDraft();
    });
    await waitFor(() => expect(apiMock.acceptCommand).toHaveBeenCalledTimes(1));
    expect(apiMock.getCommandReceipt).not.toHaveBeenCalled();
    expect(result.current.phase).toBe('sending');
    expect(result.current.selection?.commandId).toBe(firstId);
    await act(async () => {
      pending.resolve(receiptFor(firstId, '标题'));
    });
    await waitFor(() => expect(result.current.phase).toBe('accepted'));
  });

  it('明确冲突拒绝保持输入，允许用户显式开始另一份草稿', async () => {
    apiMock.acceptCommand.mockRejectedValue(
      new CommandAPIError(409, {
        code: 'idempotency_conflict',
        message: '命令 ID 已绑定其他参数',
        request_id: firstId,
      }),
    );
    const { result } = setup();
    await act(async () => {
      await result.current.createDraft('原输入');
    });
    expect(result.current.phase).toBe('rejected');
    expect(result.current.selection?.title).toBe('原输入');
    expect(result.current.canStartNew).toBe(true);
    act(() => result.current.startNewDraft());
    expect(result.current.phase).toBe('idle');
    expect(result.current.isFormLocked).toBe(false);
  });

  it('纯查询失败不发送 PUT，允许返回新建', async () => {
    apiMock.getCommandReceipt.mockRejectedValue(new TypeError('合成断线'));
    const { result } = setup();
    await act(async () => {
      await result.current.lookup(firstId);
    });
    expect(result.current.phase).toBe('unknown');
    expect(result.current.canStartNew).toBe(true);
    expect(apiMock.acceptCommand).not.toHaveBeenCalled();
    act(() => result.current.startNewDraft());
    expect(result.current.selection).toBeNull();
  });

  it('较早查询的迟到响应不会覆盖当前命令选择和回执', async () => {
    const first = deferred<CommandReceipt>();
    const second = deferred<CommandReceipt>();
    apiMock.getCommandReceipt.mockImplementation((_projectId: string, commandId: string) =>
      commandId === firstId ? first.promise : second.promise,
    );
    const { result } = setup();
    act(() => {
      void result.current.lookup(firstId);
    });
    act(() => {
      void result.current.lookup(secondId);
    });
    await act(async () => {
      second.resolve(receiptFor(secondId, '第二份'));
    });
    await waitFor(() => expect(result.current.receipt?.command_id).toBe(secondId));
    await act(async () => {
      first.resolve(receiptFor(firstId, '第一份'));
    });
    expect(result.current.selection?.commandId).toBe(secondId);
    expect(result.current.receipt?.task_title).toBe('第二份');
    expect(result.current.phase).toBe('accepted');
  });

  it('已确认命令重新查询失败时保留上次权威回执并提示陈旧状态', async () => {
    apiMock.getCommandReceipt
      .mockResolvedValueOnce(receiptFor(firstId))
      .mockRejectedValueOnce(new Error('合成依赖不可用'));
    const { result } = setup();
    await act(async () => {
      await result.current.lookup(firstId);
    });
    await act(async () => {
      await result.current.lookup(firstId);
    });
    expect(result.current.receipt).toEqual(receiptFor(firstId));
    expect(result.current.phase).toBe('accepted');
    expect(result.current.notice?.message).toContain('保留上次确认');
  });
});
