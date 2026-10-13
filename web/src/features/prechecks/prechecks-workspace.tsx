import { Link, useParams } from 'react-router-dom';
import { getPrecheck, getProjectPolicy, listPrechecks } from '../../api/client';
import type { components } from '../../api/generated/schema';
import { Card, CardHeader, CardTitle, CardContent } from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { useProjectQuery } from '../scope/use-project-query';
import { useListLocation } from '../scope/use-list-location';
import { Pagination } from '../scope/pagination';
import { QueryStatus } from '../scope/query-status';
import { formatTime, operationLabels } from '../scope/presentation';
import type { ScopeCommandWorkspace } from '../scope/use-scope-command';
import { PrecheckForm } from './precheck-form';
import { PrecheckDetail } from './precheck-detail';

/** 预检查表单与历史读取不要求资产先入库，列表/详情使用项目和 actor 缓存边界。 */
export function PrechecksWorkspace({
  actorId,
  project,
  command,
}: {
  actorId: string;
  project: components['schemas']['ProjectAccess'];
  command: ScopeCommandWorkspace;
}) {
  const { decisionId } = useParams();
  const location = useListLocation();
  const filters = { page_size: 25, ...(location.cursor ? { cursor: location.cursor } : {}) };
  const policy = useProjectQuery('authorization', actorId, project.id, ['policy'], (signal) =>
    getProjectPolicy(project.id, signal),
  );
  const list = useProjectQuery(
    'prechecks',
    actorId,
    project.id,
    ['list', filters],
    (signal) => listPrechecks(project.id, filters, signal),
    !decisionId,
  );
  const detail = useProjectQuery(
    'prechecks',
    actorId,
    project.id,
    ['decision', decisionId ?? null],
    (signal) => getPrecheck(project.id, decisionId ?? '', signal),
    Boolean(decisionId),
  );
  return (
    <div className="scope-page">
      <QueryStatus query={policy} />
      {!decisionId ? (
        <>
          <PrecheckForm
            currentVersionId={policy.data?.active_version_id ?? null}
            canCheck={project.capabilities.includes('policy:precheck')}
            command={command}
          />
          <Card>
            <CardHeader>
              <CardTitle>
                <h2>历史静态决策</h2>
              </CardTitle>
            </CardHeader>
            <CardContent>
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
              <QueryStatus query={list} empty={list.data?.items.length === 0} />
              {list.data && (
                <>
                  <ul className="record-list" aria-label="预检查历史列表">
                    {list.data.items.map((decision) => (
                      <li key={decision.id}>
                        <div className="record-title">
                          <Link
                            to={`/prechecks/${decision.id}${location.search ? `?${location.search}` : ''}`}
                          >
                            {decision.outcome === 'deny' ? '静态规则拒绝' : '静态匹配，运行时待检'}
                          </Link>
                          <span className="type-label">{operationLabels[decision.operation]}</span>
                        </div>
                        <p>
                          <code>{decision.canonical_target}</code>
                        </p>
                        <p>
                          {formatTime(decision.checked_at)} · 项目策略版本{' '}
                          {decision.project_revision}
                        </p>
                      </li>
                    ))}
                  </ul>
                  <Pagination
                    count={list.data.page_count}
                    hasMore={list.data.has_more}
                    cursor={location.cursor}
                    nextCursor={list.data.next_cursor}
                    busy={list.isFetching}
                    onNext={location.next}
                    onFirst={() => location.first()}
                  />
                </>
              )}
            </CardContent>
          </Card>
        </>
      ) : (
        <>
          <QueryStatus query={detail} />
          {detail.data && (
            <PrecheckDetail
              decision={detail.data}
              {...(policy.data ? { currentPolicy: policy.data } : {})}
              listSearch={location.search}
            />
          )}
        </>
      )}
    </div>
  );
}
