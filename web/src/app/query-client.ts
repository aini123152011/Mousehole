import { QueryClient } from '@tanstack/react-query';

/** 为当前页面建立服务器状态缓存；命令失败不会自动重发。 */
export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}
