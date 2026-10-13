import { Link } from 'react-router-dom';
import type { PolicyDecision, ProjectPolicy } from '../../api/client';
import { Button } from '../../components/ui/button';
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from '../../components/ui/card';
import { formatTime, operationLabels } from '../scope/presentation';

/** 解释历史决策及运行时缺项；静态匹配使用待检语义，不显示绿色执行允许。 */
export function PrecheckDetail({
  decision,
  currentPolicy,
  listSearch = '',
}: {
  decision: PolicyDecision;
  currentPolicy?: ProjectPolicy;
  listSearch?: string;
}) {
  const changed =
    currentPolicy &&
    (currentPolicy.project_revision !== decision.project_revision ||
      currentPolicy.global_revision !== decision.global_revision ||
      currentPolicy.active_version_id !== decision.authorization_version_id);
  return (
    <Card className="decision-detail">
      <CardHeader>
        <CardTitle>
          <h2>{decision.outcome === 'deny' ? '静态规则拒绝' : '静态规则匹配，执行条件尚未验证'}</h2>
        </CardTitle>
        <CardDescription>
          持久化于 {formatTime(decision.checked_at)}。没有执行令牌，也没有完成目标验证。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="boundary-note">
          <strong>
            {changed ? '当前规则已变化，下方为原检查快照' : '历史结果不表示当前执行许可'}
          </strong>
          <p>恢复不会重新计算。若需要查看当前规则，请结束原命令，再显式创建新的预检查。</p>
        </div>
        <p className="decision-message">{decision.message}</p>
        <p>
          原因码：
          {decision.reason_codes.map((code) => (
            <code key={code} className="reason-code">
              {code}
            </code>
          ))}
        </p>
        <dl className="scope-details">
          <div>
            <dt>规范目标</dt>
            <dd>
              <code>{decision.canonical_target}</code>
            </dd>
          </div>
          <div>
            <dt>预检查操作</dt>
            <dd>
              {operationLabels[decision.operation]}
              {decision.parameters.port ? ` · 端口 ${decision.parameters.port}` : ''}
            </dd>
          </div>
          <div>
            <dt>检查时刻</dt>
            <dd>{formatTime(decision.checked_at)}</dd>
          </div>
          <div>
            <dt>授权版本</dt>
            <dd>
              <Link to={`/authorization/${decision.authorization_version_id}`}>
                {decision.authorization_version_id}
              </Link>
            </dd>
          </div>
          <div>
            <dt>内容 hash</dt>
            <dd>
              <code>{decision.content_hash}</code>
            </dd>
          </div>
          <div>
            <dt>命中条目</dt>
            <dd>
              <code>{decision.matched_entry_id ?? '无完整匹配条目'}</code>
            </dd>
          </div>
          <div>
            <dt>全局 / 项目策略版本</dt>
            <dd>
              {decision.global_revision} / {decision.project_revision}
            </dd>
          </div>
          <div>
            <dt>算法 / 策略 / 目录</dt>
            <dd>
              {decision.algorithm_version} / {decision.policy_version} / {decision.catalog_version}
            </dd>
          </div>
          <div>
            <dt>检查 / 原命令 ID</dt>
            <dd>
              <code>{decision.id}</code>
              <br />
              <code>{decision.command_id}</code>
            </dd>
          </div>
        </dl>
        <h3 className="section-title">运行时仍须复核</h3>
        <ul className="runtime-checks">
          {decision.runtime_checks.map((check) => (
            <li key={check.code}>
              <span className="type-label">
                {check.status === 'required' ? '尚待检查' : '该操作不适用'}
              </span>
              <span>{check.message}</span>
              <code>{check.code}</code>
            </li>
          ))}
        </ul>
        <p className="field-hint">
          执行前仍需重新核验身份、授权时窗、域名解析与最终地址、连接出口、速率和预算。
          当前页面只说明静态条件，不连接目标。
        </p>
        <Button asChild variant="outline" className="mt-5">
          <Link to={`/prechecks${listSearch ? `?${listSearch}` : ''}`}>返回预检查历史</Link>
        </Button>
      </CardContent>
    </Card>
  );
}
