import { useQuery, useQueryClient } from '@tanstack/react-query';
import { handleProtectedAccessError } from '../../app/protected-cache';

/** 领域查询始终包含 actor/project/filter/object 身份；权限失效取消同边界的旧请求。 */
export function useProjectQuery<T>(
  root: 'assets' | 'authorization' | 'prechecks',
  actorId: string,
  projectId: string,
  identity: readonly unknown[],
  fetcher: (signal: AbortSignal) => Promise<T>,
  enabled = true,
) {
  const client = useQueryClient();
  return useQuery({
    queryKey: [root, actorId, projectId, ...identity],
    queryFn: async ({ signal }) => {
      try {
        return await fetcher(signal);
      } catch (error: unknown) {
        if (!signal.aborted) handleProtectedAccessError(client, error, actorId, projectId);
        throw error;
      }
    },
    enabled,
    retry: false,
  });
}
