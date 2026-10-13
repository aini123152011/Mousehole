import { Link, useParams, useSearchParams } from 'react-router-dom';
import { getAuthorizationVersion, listAuthorizationVersions } from '../../api/client';
import type { components } from '../../api/generated/schema';
import { Button } from '../../components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '../../components/ui/card';
import { useProjectQuery } from '../scope/use-project-query';
import { QueryStatus } from '../scope/query-status';
import { useListLocation } from '../scope/use-list-location';
import { Pagination } from '../scope/pagination';
import { formatTime, versionLabels } from '../scope/presentation';
import type { ScopeCommandWorkspace } from '../scope/use-scope-command';
import { AuthorizationForm } from './authorization-form';
import { AuthorizationVersionDetail } from './authorization-version-detail';
import { PolicySummary } from './policy-summary';

/** 授权列表/详情/新候选路由共用项目边界；返回列表恢复原 seek 页。 */
export function AuthorizationWorkspace({
  actorId,
  project,
  command,
}: {
  actorId: string;
  project: components['schemas']['ProjectAccess'];
  command: ScopeCommandWorkspace;
}) {
  const { versionId } = useParams();
  const [params] = useSearchParams();
  const location = useListLocation();
  const copyId = versionId === 'new' ? params.get('copy') : null;
  const detailId = copyId ?? (versionId !== 'new' ? versionId : undefined);
  const filters = { page_size: 25, ...(location.cursor ? { cursor: location.cursor } : {}) };
  const list = useProjectQuery(
    'authorization',
    actorId,
    project.id,
    ['list', filters],
    (signal) => listAuthorizationVersions(project.id, filters, signal),
    !versionId,
  );
  const detail = useProjectQuery(
    'authorization',
    actorId,
    project.id,
    ['version', detailId ?? null],
    (signal) => getAuthorizationVersion(project.id, detailId ?? '', signal),
    Boolean(detailId),
  );
  const canCreate = project.capabilities.includes('authorization:create');
  return (
    <div className="scope-page">
      <PolicySummary actorId={actorId} projectId={project.id} />
      {!versionId ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>授权版本历史</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="action-row">
              <Button asChild disabled={!canCreate}>
                <Link
                  to="/authorization/new"
                  aria-disabled={!canCreate}
                  onClick={(event) => {
                    if (!canCreate) event.preventDefault();
                  }}
                >
                  编写候选版本
                </Link>
              </Button>
              <Button
                variant="outline"
                disabled={list.isFetching}
                onClick={() => {
                  location.first();
                  void list.refetch();
                }}
              >
                刷新历史
              </Button>
            </div>
            {!canCreate && <p className="permission-note">当前只读；候选写入需要分析员能力。</p>}
            <QueryStatus query={list} empty={list.data?.items.length === 0} />
            {list.data && (
              <>
                <ul className="record-list" aria-label="授权版本列表">
                  {list.data.items.map((version) => (
                    <li key={version.id}>
                      <div className="record-title">
                        <Link
                          to={`/authorization/${version.id}${location.search ? `?${location.search}` : ''}`}
                        >
                          {version.content.title}
                        </Link>
                        <span className="type-label">版本 {version.sequence}</span>
                      </div>
                      <p>
                        {versionLabels[version.display_state]} · {formatTime(version.created_at)}
                      </p>
                      <code>{version.content_hash}</code>
                    </li>
                  ))}
                </ul>
                <Pagination
                  count={list.data.page_count}
                  hasMore={list.data.has_more}
                  nextCursor={list.data.next_cursor}
                  cursor={location.cursor}
                  busy={list.isFetching}
                  onNext={location.next}
                  onFirst={() => location.first()}
                />
              </>
            )}
          </CardContent>
        </Card>
      ) : versionId === 'new' ? (
        <>
          <Button variant="outline" asChild>
            <Link to="/authorization">返回授权列表</Link>
          </Button>
          {copyId ? (
            <>
              <QueryStatus query={detail} />
              {detail.data && (
                <AuthorizationForm
                  key={copyId}
                  initial={detail.data.content}
                  canCreate={canCreate}
                  command={command}
                />
              )}
            </>
          ) : (
            <AuthorizationForm key="new" canCreate={canCreate} command={command} />
          )}
        </>
      ) : (
        <>
          <QueryStatus query={detail} />
          {detail.data && (
            <AuthorizationVersionDetail
              version={detail.data}
              canCreate={canCreate}
              listSearch={location.search}
            />
          )}
        </>
      )}
    </div>
  );
}
