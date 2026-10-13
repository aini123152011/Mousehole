import { useSearchParams } from 'react-router-dom';

/** 地址栏仅存分页位置和非敏感类型；目标搜索值由页面内存持有。 */
export function useListLocation() {
  const [params, setParams] = useSearchParams();
  const cursor = params.get('cursor') ?? '';
  function next(nextCursor: string | null) {
    if (!nextCursor) return;
    setParams((previous) => {
      const result = new URLSearchParams(previous);
      result.set('cursor', nextCursor);
      return result;
    });
  }
  function first(kind?: string) {
    setParams((previous) => {
      const result = new URLSearchParams(previous);
      result.delete('cursor');
      if (kind !== undefined) {
        if (kind) result.set('kind', kind);
        else result.delete('kind');
      }
      return result;
    });
  }
  return { cursor, kind: params.get('kind') ?? '', next, first, search: params.toString() };
}
