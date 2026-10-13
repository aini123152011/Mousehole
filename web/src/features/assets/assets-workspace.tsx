import { Link, useParams } from 'react-router-dom';
import { listAssets, type AssetFilters } from '../../api/client';
import type { components } from '../../api/generated/schema';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '../../components/ui/card';
import { useProjectQuery } from '../scope/use-project-query';
import { useListLocation } from '../scope/use-list-location';
import { Pagination } from '../scope/pagination';
import { QueryStatus } from '../scope/query-status';
import { assetLabels, formatTime } from '../scope/presentation';
import type { ScopeCommandWorkspace } from '../scope/use-scope-command';
import { AssetImportForm } from './asset-import-form';

/** 项目路由祖先持有敏感搜索输入，详情返回仍匹配原 cursor，不写历史 state。 */
export interface AssetSearchState {
  draft: string;
  value: string;
  setDraft: (value: string) => void;
  applyToNextLocation: (value: string) => void;
}

/** 项目资产查询/导入与来源展开；全企业去重结果不会泄露其他项目关联。 */
export function AssetsWorkspace({
  actorId,
  project,
  command,
  searchState,
}: {
  actorId: string;
  project: components['schemas']['ProjectAccess'];
  command: ScopeCommandWorkspace;
  searchState: AssetSearchState;
}) {
  const { assetId } = useParams();
  const location = useListLocation();
  const {
    draft: search,
    setDraft: setSearch,
    value: appliedSearch,
    applyToNextLocation: applySearch,
  } = searchState;
  const kind =
    location.kind === 'domain' || location.kind === 'ip' || location.kind === 'url'
      ? location.kind
      : undefined;
  const filters: AssetFilters = {
    page_size: 25,
    ...(kind ? { kind } : {}),
    ...(appliedSearch ? { search: appliedSearch } : {}),
    ...(location.cursor ? { cursor: location.cursor } : {}),
  };
  const assets = useProjectQuery('assets', actorId, project.id, ['list', filters], (signal) =>
    listAssets(project.id, filters, signal),
  );
  const selected = assets.data?.items.find((asset) => asset.id === assetId);
  if (assetId)
    return (
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>资产来源详情</h2>
          </CardTitle>
          <CardDescription>来源只是本项目导入声明，不是完成验证的证据。</CardDescription>
        </CardHeader>
        <CardContent>
          <QueryStatus query={assets} />
          {selected ? (
            <>
              <p>
                <code>{selected.canonical_value}</code>
              </p>
              <ul className="source-list">
                {selected.sources.map((source) => (
                  <li key={source.command_id}>
                    <p>
                      {source.source === 'manual' ? '手动录入' : 'API 导入声明'} ·{' '}
                      {formatTime(source.created_at)}
                    </p>
                    <p>备注：{source.notes || '未填写'}</p>
                    <p>引用：{source.external_ref || '未填写'}</p>
                    <p>
                      来源命令：<code>{source.command_id}</code>
                    </p>
                  </li>
                ))}
              </ul>
            </>
          ) : (
            assets.data && <p>当前分页位置未找到此资产，请返回项目列表查找。</p>
          )}
          <Button asChild variant="outline" className="mt-4">
            <Link to={`/assets${location.search ? `?${location.search}` : ''}`}>返回资产列表</Link>
          </Button>
        </CardContent>
      </Card>
    );
  return (
    <div className="scope-page">
      <AssetImportForm
        actorId={actorId}
        projectId={project.id}
        canImport={project.capabilities.includes('asset:import')}
        command={command}
      />
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>项目资产</h2>
          </CardTitle>
          <CardDescription>只显示本项目资产和来源；来源声明没有完成观测或验证。</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="list-filter"
            onSubmit={(event) => {
              event.preventDefault();
              applySearch(search.trim());
              location.first();
            }}
          >
            <div>
              <label htmlFor="asset-filter-kind">筛选资产类型</label>
              <select
                id="asset-filter-kind"
                value={kind ?? ''}
                onChange={(event) => location.first(event.target.value)}
              >
                <option value="">全部类型</option>
                {Object.entries(assetLabels).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </div>
            <div className="grow">
              <label htmlFor="asset-search">规范值搜索</label>
              <Input
                id="asset-search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                maxLength={200}
                autoComplete="off"
              />
            </div>
            <Button variant="outline" type="submit" disabled={assets.isFetching}>
              应用筛选
            </Button>
            <Button
              variant="ghost"
              type="button"
              disabled={assets.isFetching}
              onClick={() => {
                location.first();
                void assets.refetch();
              }}
            >
              刷新
            </Button>
          </form>
          <p className="field-hint">目标搜索仅保留当前页面内存，地址栏仅记录类型和分页位置。</p>
          <QueryStatus query={assets} empty={assets.data?.items.length === 0} />
          {assets.data && (
            <>
              <ul className="record-list">
                {assets.data.items.map((asset) => (
                  <li key={asset.id} className="asset-record">
                    <div className="record-title">
                      <span className="type-label">{assetLabels[asset.kind]}</span>
                      <code>{asset.canonical_value}</code>
                    </div>
                    <p className="field-hint">
                      登记于 {formatTime(asset.created_at)} · {asset.algorithm_version}
                    </p>
                    <details>
                      <summary>查看本项目来源（{asset.sources.length} 份声明）</summary>
                      <ul className="source-list">
                        {asset.sources.map((source) => (
                          <li key={source.command_id}>
                            <p>
                              {source.source === 'manual' ? '手动录入' : 'API 导入声明'} ·{' '}
                              {formatTime(source.created_at)}
                            </p>
                            <p>备注：{source.notes || '未填写'}</p>
                            <p>引用：{source.external_ref || '未填写'}</p>
                            <p>
                              来源命令：<code>{source.command_id}</code>
                            </p>
                          </li>
                        ))}
                      </ul>
                    </details>
                    <Link to={`/assets/${asset.id}${location.search ? `?${location.search}` : ''}`}>
                      查看来源详情
                    </Link>
                  </li>
                ))}
              </ul>
              <Pagination
                count={assets.data.page_count}
                hasMore={assets.data.has_more}
                nextCursor={assets.data.next_cursor}
                cursor={location.cursor}
                busy={assets.isFetching}
                onNext={location.next}
                onFirst={() => location.first()}
              />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
