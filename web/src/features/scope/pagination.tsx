import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/button';

/** 总量未知时只陈述本页数量；seek 下一页写入历史，浏览器返回可恢复原页。 */
export function Pagination({
  count,
  hasMore,
  cursor,
  nextCursor,
  busy,
  onNext,
  onFirst,
}: {
  count: string;
  hasMore: boolean;
  cursor: string;
  nextCursor: string | null;
  busy: boolean;
  onNext: (cursor: string | null) => void;
  onFirst: () => void;
}) {
  const navigate = useNavigate();
  return (
    <nav aria-label="记录分页" className="list-pagination">
      <p>
        本页 {count} 条 · 总量未提供{hasMore ? ' · 还有下一页' : ' · 已到末页'}
      </p>
      <div className="action-row">
        {cursor && (
          <>
            <Button variant="outline" onClick={() => navigate(-1)} disabled={busy}>
              返回上一位置
            </Button>
            <Button variant="outline" onClick={onFirst} disabled={busy}>
              回到首页
            </Button>
          </>
        )}
        <Button
          variant="outline"
          disabled={busy || !hasMore || !nextCursor}
          onClick={() => onNext(nextCursor)}
        >
          下一页
        </Button>
      </div>
    </nav>
  );
}
