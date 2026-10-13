import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  acceptCommand,
  CommandAPIError,
  getCommandReceipt,
  isDefinitiveRejection,
} from '../../api/client';
import { handleProtectedAccessError } from '../../app/protected-cache';
import { normalizeTaskTitle } from './command-schema';

interface CommandSelection {
  commandId: string;
  /** 只有本页发出的未知命令必须锁定原参数；纯查询不产生新业务写入。 */
  source: 'submission' | 'lookup';
  title: string | null;
}

type AdmissionPhase = 'idle' | 'sending' | 'unknown' | 'accepted' | 'rejected';
interface AdmissionNotice {
  message: string;
  requestId: string | null;
}

/** 缓存身份包含项目和命令，迟到查询不能替换当前选择。 */
export const commandKeys = {
  detail: (actorId: string, projectId: string, commandId: string | null) =>
    ['commands', actorId, projectId, commandId] as const,
};

function noticeFor(error: unknown, isLookup: boolean): AdmissionNotice {
  if (error instanceof CommandAPIError) {
    if (isLookup && error.code === 'command_not_found') {
      return {
        message: '暂未查询到回执，受理状态仍待确认。请保留原命令 ID，稍后再次查询。',
        requestId: error.requestId,
      };
    }
    return { message: error.message, requestId: error.requestId };
  }
  return { message: '连接中断，受理状态待确认。请保留原命令 ID 并查询。', requestId: null };
}

/**
 * 管理一次提交及人工恢复查询。回执只保存在 TanStack Query，局部状态仅表达交互。
 * 提交前同步锁定 ID/参数；响应丢失不会自动重发，也不会允许换参数创建另一命令。
 * 查询使用选择代次隔离迟到结果，已有 ID 的查询不创建草稿。
 */
export function useCommandWorkspace(actorId: string, projectId: string) {
  const queryClient = useQueryClient();
  const [selection, setSelection] = useState<CommandSelection | null>(null);
  const [phase, setPhase] = useState<AdmissionPhase>('idle');
  const [notice, setNotice] = useState<AdmissionNotice | null>(null);
  const generation = useRef(0);
  const writeController = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      ++generation.current;
      writeController.current?.abort();
    },
    [],
  );
  // 仅同步交互锁，不存放服务端快照；事件在下一次渲染前也须遵守同一边界。
  const interaction = useRef<{ selection: CommandSelection | null; phase: AdmissionPhase }>({
    selection: null,
    phase: 'idle',
  });

  function selectCommand(value: CommandSelection | null) {
    interaction.current.selection = value;
    setSelection(value);
  }

  function changePhase(value: AdmissionPhase) {
    interaction.current.phase = value;
    setPhase(value);
  }

  const query = useQuery({
    queryKey: commandKeys.detail(actorId, projectId, selection?.commandId ?? null),
    queryFn: ({ signal }) => {
      if (!selection) throw new Error('尚未选择命令。');
      return getCommandReceipt(projectId, selection.commandId, signal, selection.title);
    },
    enabled: false,
    retry: false,
  });

  async function createDraft(title: string) {
    // ref 在第一次点击时立即占有操作，防止 React 下一次渲染前的重复提交。
    if (interaction.current.selection !== null) return;
    const commandId = crypto.randomUUID();
    const normalizedTitle = normalizeTaskTitle(title);
    const attempt = ++generation.current;
    const controller = new AbortController();
    writeController.current = controller;
    selectCommand({ commandId, source: 'submission', title: normalizedTitle });
    setNotice(null);
    changePhase('sending');
    try {
      // 单次请求不在 mutation cache 留存第二份标题/回执；唯一服务器快照归 Query。
      const receipt = await acceptCommand(
        projectId,
        commandId,
        { kind: 'create_task_draft', payload: { title: normalizedTitle } },
        controller.signal,
      );
      if (generation.current !== attempt || controller.signal.aborted) return;
      queryClient.setQueryData(commandKeys.detail(actorId, projectId, commandId), receipt);
      changePhase('accepted');
    } catch (error: unknown) {
      if (generation.current !== attempt) return;
      handleProtectedAccessError(queryClient, error, actorId, projectId);
      setNotice(noticeFor(error, false));
      changePhase(isDefinitiveRejection(error) ? 'rejected' : 'unknown');
    }
  }

  async function lookup(commandId: string) {
    const current = interaction.current;
    if (current.phase === 'sending') return;
    if (
      current.selection?.source === 'submission' &&
      current.phase === 'unknown' &&
      current.selection.commandId !== commandId
    ) {
      return;
    }
    const attempt = ++generation.current;
    const isSameCommand = current.selection?.commandId === commandId;
    const expectedTitle = isSameCommand ? (current.selection?.title ?? null) : null;
    const previousReceipt = queryClient.getQueryData(
      commandKeys.detail(actorId, projectId, commandId),
    );
    if (!isSameCommand) {
      selectCommand({ commandId, source: 'lookup', title: null });
      changePhase('unknown');
    }
    setNotice(null);
    try {
      await queryClient.fetchQuery({
        queryKey: commandKeys.detail(actorId, projectId, commandId),
        queryFn: ({ signal }) => getCommandReceipt(projectId, commandId, signal, expectedTitle),
        staleTime: 0,
        retry: false,
      });
      if (generation.current === attempt) changePhase('accepted');
    } catch (error: unknown) {
      if (generation.current !== attempt) return;
      handleProtectedAccessError(queryClient, error, actorId, projectId);
      // 即便查询拒绝或超时，也不据此解锁本页曾发出的未知提交。
      setNotice(
        previousReceipt === undefined
          ? noticeFor(error, true)
          : {
              message: '本次查询未能更新回执，页面保留上次确认的受理记录。',
              requestId: error instanceof CommandAPIError ? error.requestId : null,
            },
      );
    }
  }

  function startNewDraft() {
    const current = interaction.current;
    if (
      current.selection?.source === 'submission' &&
      (current.phase === 'sending' || current.phase === 'unknown')
    ) {
      return;
    }
    ++generation.current;
    selectCommand(null);
    changePhase('idle');
    setNotice(null);
  }

  return {
    selection,
    phase,
    notice,
    receipt: query.data,
    isQuerying: query.isFetching,
    isFormLocked: selection !== null,
    canChooseLookup:
      selection?.source !== 'submission' || phase === 'accepted' || phase === 'rejected',
    canStartNew:
      selection !== null &&
      (selection.source === 'lookup' || phase === 'accepted' || phase === 'rejected'),
    createDraft,
    lookup,
    startNewDraft,
  };
}

/** 项目祖先持有的旧草稿命令交互，跨功能导航仍保留原 ID/标题。 */
export type CommandWorkspaceState = ReturnType<typeof useCommandWorkspace>;
