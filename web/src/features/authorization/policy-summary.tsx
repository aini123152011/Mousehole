import { Link } from 'react-router-dom';
import { getProjectPolicy } from '../../api/client';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { QueryStatus } from '../scope/query-status';
import { useProjectQuery } from '../scope/use-project-query';

/** 当前授权指针与有效禁止的只读视图；管理动作由 CLI 明确执行。 */
export function PolicySummary({ actorId, projectId }: { actorId: string; projectId: string }) {
  const query = useProjectQuery('authorization', actorId, projectId, ['policy'], (signal) =>
    getProjectPolicy(projectId, signal),
  );
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>当前授权与紧急禁止</h2>
        </CardTitle>
        <CardDescription>
          禁止优先、默认拒绝。无激活版本不获得许可；历史检查不能绕过当前禁止。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <QueryStatus query={query} />
        {query.data && (
          <>
            <dl className="scope-details">
              <div>
                <dt>全局 / 项目策略版本</dt>
                <dd>
                  {query.data.global_revision} / {query.data.project_revision}
                </dd>
              </div>
              <div>
                <dt>当前激活版本</dt>
                <dd>
                  {query.data.active_version_id ? (
                    <Link to={`/authorization/${query.data.active_version_id}`}>
                      {query.data.active_version_id}
                    </Link>
                  ) : (
                    '无；静态规则默认拒绝'
                  )}
                </dd>
              </div>
            </dl>
            <h3 className="section-title">有效禁止规则（{query.data.denies.length} 条）</h3>
            {query.data.denies.length === 0 ? (
              <p>当前没有显式紧急禁止。平台保留地址硬拒绝始终生效。</p>
            ) : (
              <ul className="record-list">
                {query.data.denies.map((deny) => (
                  <li key={deny.id}>
                    <p>
                      {deny.project_id === null ? '全企业' : '本项目'} · {deny.kind} ·{' '}
                      <code>{deny.value}</code>
                    </p>
                    <p>
                      {deny.include_subdomains ? '包含子域 · ' : ''}
                      {deny.reason}
                    </p>
                    <p>
                      规则 ID：<code>{deny.id}</code>
                    </p>
                  </li>
                ))}
              </ul>
            )}
            <details className="admin-guidance">
              <summary>管理员操作说明（CLI）</summary>
              <p>
                分析员不能激活、撤销或改变禁止。请把项目 ID、候选版本 ID
                和最新策略版本交给有部署权限的管理员。以下只是命令说明，不会执行管理动作。
              </p>
              <code>
                admin scope activate --project-id {projectId} --version-id &lt;候选版本ID&gt;
                --command-id &lt;新UUID&gt; --expected-policy-revision {query.data.project_revision}{' '}
                --operator-ref &lt;操作引用&gt;
              </code>
              <p>
                数据库配置由管理员受控传入；并发冲突须读取最新 revision，不能覆盖。已成功命令以原 ID
                查询回执。
              </p>
            </details>
          </>
        )}
        <Button
          variant="outline"
          className="mt-4"
          disabled={query.isFetching}
          onClick={() => {
            void query.refetch();
          }}
        >
          刷新当前策略
        </Button>
      </CardContent>
    </Card>
  );
}
