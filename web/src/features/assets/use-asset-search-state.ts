import { useState } from 'react';
import { useLocation } from 'react-router-dom';

interface SearchSnapshot {
  draft: string;
  value: string;
}

/** 搜索仅保存在项目祖先内存；位置键恢复与 cursor 同时生效，不写浏览器历史 state。 */
export function useAssetSearchState() {
  const location = useLocation();
  const [state, setState] = useState(() => {
    const current = { draft: '', value: '' };
    return {
      key: location.key,
      current,
      entries: new Map<string, SearchSnapshot>([[location.key, current]]),
      pending: null as SearchSnapshot | null,
    };
  });
  let snapshot = state.current;
  if (state.key !== location.key) {
    // 在子查询渲染前恢复快照，避免先用新搜索请求旧 cursor 再补偿。
    snapshot = state.entries.get(location.key) ?? state.pending ?? state.current;
    setState({
      key: location.key,
      current: snapshot,
      entries: new Map(state.entries).set(location.key, snapshot),
      pending: null,
    });
  }
  return {
    ...snapshot,
    setDraft(draft: string) {
      setState((previous) => {
        const current = { ...previous.current, draft };
        return {
          ...previous,
          current,
          entries: new Map(previous.entries).set(previous.key, current),
        };
      });
    },
    applyToNextLocation(value: string) {
      setState((previous) => {
        // 原历史位置仍代表原已应用条件；新筛选只归属于随后创建的新位置。
        const current = { value: previous.current.value, draft: previous.current.value };
        return {
          ...previous,
          current,
          entries: new Map(previous.entries).set(previous.key, current),
          pending: { value, draft: value },
        };
      });
    },
  };
}
