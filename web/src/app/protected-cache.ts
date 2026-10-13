import type { QueryClient } from '@tanstack/react-query';
import { CommandAPIError, setSessionMemory } from '../api/client';

const protectedRoots = new Set([
  'commands',
  'assets',
  'authorization',
  'prechecks',
  'scope-commands',
]);

/** 撤销旧服务器状态的所有权；先取消请求，再移除缓存，避免迟到查询恢复敏感记录。 */
export async function clearProtectedCache(
  client: QueryClient,
  actorId?: string,
  projectId?: string,
) {
  const filters = {
    predicate: (query: { queryKey: readonly unknown[] }) =>
      typeof query.queryKey[0] === 'string' &&
      protectedRoots.has(query.queryKey[0]) &&
      (actorId === undefined || query.queryKey[1] === actorId) &&
      (projectId === undefined || query.queryKey[2] === projectId),
  };
  const cancellation = client.cancelQueries(filters);
  client.removeQueries(filters);
  await cancellation;
}

/** 项目拒绝只清当前边界；会话失效清全部受保护数据，并重新读取服务端身份。 */
export function handleProtectedAccessError(
  client: QueryClient,
  error: unknown,
  actorId: string,
  projectId: string,
) {
  if (!(error instanceof CommandAPIError)) return;
  if (error.status === 401) {
    setSessionMemory(null);
    void clearProtectedCache(client);
    void client.invalidateQueries({ queryKey: ['session'] });
  } else if (
    error.status === 403 &&
    (error.code === 'project_forbidden' || error.code === 'forbidden')
  ) {
    void clearProtectedCache(client, actorId, projectId);
    void client.invalidateQueries({ queryKey: ['session'] });
  }
}
