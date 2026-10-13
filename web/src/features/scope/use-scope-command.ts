import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  acceptScopeCommand,
  CommandAPIError,
  getScopeCommandReceipt,
  isDefinitiveRejection,
  type ScopeCommandRequest,
  type ScopeCommandReceipt,
} from '../../api/client';
import { handleProtectedAccessError } from '../../app/protected-cache';

interface Selection {
  commandId: string;
  source: 'submission' | 'lookup';
  body: ScopeCommandRequest | null;
}
type Phase = 'idle' | 'sending' | 'unknown' | 'accepted' | 'rejected';

/** 回执只在 Query 内存中；退出/跨项目取消请求后不允许迟到响应写回旧缓存。 */
export const scopeCommandKeys = {
  detail: (actorId: string, projectId: string, commandId: string | null) =>
    ['scope-commands', actorId, projectId, commandId] as const,
};

/** 管理资产、候选和预检查的单次受理；未知状态锁定原 ID/参数，不自动发送第二条命令。 */
export function useScopeCommand(actorId: string, projectId: string) {
  const client = useQueryClient();
  const [selection, setSelection] = useState<Selection | null>(null);
  const [phase, setPhase] = useState<Phase>('idle');
  const [notice, setNotice] = useState<string | null>(null);
  const [validationErrors, setValidationErrors] = useState<CommandAPIError['validationErrors']>([]);
  const control = useRef<{
    selection: Selection | null;
    phase: Phase;
    generation: number;
    controller: AbortController | null;
  }>({ selection: null, phase: 'idle', generation: 0, controller: null });
  useEffect(
    () => () => {
      ++control.current.generation;
      control.current.controller?.abort();
    },
    [],
  );

  function select(next: Selection | null, nextPhase: Phase) {
    control.current.selection = next;
    control.current.phase = nextPhase;
    setSelection(next);
    setPhase(nextPhase);
  }

  const receipt = useQuery<ScopeCommandReceipt>({
    queryKey: scopeCommandKeys.detail(actorId, projectId, selection?.commandId ?? null),
    enabled: false,
    queryFn: ({ signal }) => {
      if (!selection) throw new Error('尚未选择范围命令。');
      return getScopeCommandReceipt(projectId, selection.commandId, signal, selection.body);
    },
    retry: false,
  });

  function refreshBusiness() {
    for (const root of ['assets', 'authorization', 'prechecks'])
      void client.invalidateQueries({ queryKey: [root, actorId, projectId] });
  }

  async function submit(body: ScopeCommandRequest) {
    if (control.current.selection) return;
    // 同一事件循环也只产生一个 UUID；参数与 ID 在网络请求前拥有稳定所有权。
    const commandId = crypto.randomUUID();
    const bound = structuredClone(body);
    const generation = ++control.current.generation;
    const controller = new AbortController();
    control.current.controller = controller;
    select({ commandId, source: 'submission', body: bound }, 'sending');
    setNotice(null);
    setValidationErrors([]);
    try {
      const data = await acceptScopeCommand(projectId, commandId, bound, controller.signal);
      if (control.current.generation !== generation || controller.signal.aborted) return;
      client.setQueryData(scopeCommandKeys.detail(actorId, projectId, commandId), data);
      select(control.current.selection, 'accepted');
      refreshBusiness();
    } catch (error: unknown) {
      if (control.current.generation !== generation) return;
      handleProtectedAccessError(client, error, actorId, projectId);
      setNotice(
        error instanceof CommandAPIError
          ? error.message
          : '连接中断，受理状态待确认。请保留原命令 ID 查询。',
      );
      setValidationErrors(error instanceof CommandAPIError ? error.validationErrors : []);
      select(control.current.selection, isDefinitiveRejection(error) ? 'rejected' : 'unknown');
    }
  }

  async function lookup(commandId: string) {
    const current = control.current;
    if (
      current.phase === 'sending' ||
      (current.selection?.source === 'submission' &&
        current.phase === 'unknown' &&
        commandId !== current.selection.commandId)
    )
      return;
    const generation = ++current.generation;
    const same = current.selection?.commandId === commandId;
    const expected = same ? (current.selection?.body ?? null) : null;
    const previous = client.getQueryData(scopeCommandKeys.detail(actorId, projectId, commandId));
    if (!same) select({ commandId, source: 'lookup', body: null }, 'unknown');
    setNotice(null);
    try {
      await client.fetchQuery({
        queryKey: scopeCommandKeys.detail(actorId, projectId, commandId),
        queryFn: ({ signal }) => getScopeCommandReceipt(projectId, commandId, signal, expected),
        staleTime: 0,
        retry: false,
      });
      if (control.current.generation !== generation) return;
      select(control.current.selection, 'accepted');
      refreshBusiness();
    } catch (error: unknown) {
      if (control.current.generation !== generation) return;
      handleProtectedAccessError(client, error, actorId, projectId);
      setNotice(
        previous !== undefined
          ? '本次查询未更新，保留上次确认的历史回执。'
          : error instanceof CommandAPIError && error.status === 404
            ? '暂未找到原回执，受理状态仍待确认；可保留原 ID 继续查询。'
            : error instanceof CommandAPIError
              ? error.message
              : '查询连接中断，受理状态仍待确认。',
      );
    }
  }

  function reset() {
    const current = control.current;
    if (
      current.selection?.source === 'submission' &&
      (current.phase === 'sending' || current.phase === 'unknown')
    )
      return;
    ++current.generation;
    select(null, 'idle');
    setNotice(null);
  }

  return {
    selection,
    phase,
    notice,
    validationErrors,
    receipt: receipt.data,
    isQuerying: receipt.isFetching,
    isLocked: selection !== null,
    canLookup: selection?.source !== 'submission' || phase === 'accepted' || phase === 'rejected',
    canReset:
      selection !== null &&
      (selection.source === 'lookup' || phase === 'accepted' || phase === 'rejected'),
    submit,
    lookup,
    reset,
  };
}

/** 三个页面共享的交互状态，不保存第二份服务器回执。 */
export type ScopeCommandWorkspace = ReturnType<typeof useScopeCommand>;
