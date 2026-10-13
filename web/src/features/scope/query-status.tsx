import type { UseQueryResult } from '@tanstack/react-query';
import { Button } from '../../components/ui/button';
import { CommandAPIError } from '../../api/client';
import { formatTime } from './presentation';

/** 加载、无数据和更新失败分别显示；存在旧记录时不把它标成实时确认。 */
export function QueryStatus({ query, empty }: { query: UseQueryResult; empty?: boolean }) {
  if (query.isPending)
    return (
      <p className="query-status" role="status">
        正在读取项目记录…
      </p>
    );
  if (query.isError)
    return (
      <div className="query-status query-error" role="alert">
        <p>
          {query.data === undefined
            ? '暂时无法读取。'
            : `本次更新失败，下方保留 ${formatTime(new Date(query.dataUpdatedAt).toISOString())} 确认的旧数据。`}
        </p>
        <p>
          {query.error instanceof CommandAPIError
            ? query.error.message
            : '连接暂时不可用，请重新查询。'}
        </p>
        <Button
          type="button"
          variant="outline"
          disabled={query.isFetching}
          onClick={() => {
            void query.refetch();
          }}
        >
          重新查询
        </Button>
      </div>
    );
  if (empty)
    return (
      <p className="query-status" role="status">
        当前筛选下暂无记录。
      </p>
    );
  if (query.isFetching)
    return (
      <p className="query-status" role="status">
        正在更新，下方为上次确认的数据…
      </p>
    );
  return null;
}
