import { Link } from 'react-router-dom';
import type { AuthorizationVersion } from '../../api/client';
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { formatTime, operationLabels, versionLabels } from '../scope/presentation';

/** 只读完整授权内容；复制生成新候选，不给旧内容提供编辑或假激活按钮。 */
export function AuthorizationVersionDetail({
  version,
  canCreate,
  listSearch = '',
}: {
  version: AuthorizationVersion;
  canCreate: boolean;
  listSearch?: string;
}) {
  const content = version.content;
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{content.title}</h2>
        </CardTitle>
        <CardDescription>
          版本 {version.sequence} · {versionLabels[version.display_state]} · 保存内容不可变
        </CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="scope-details">
          <div>
            <dt>版本 ID</dt>
            <dd>
              <code>{version.id}</code>
            </dd>
          </div>
          <div>
            <dt>内容 SHA-256</dt>
            <dd>
              <code>{version.content_hash}</code>
            </dd>
          </div>
          <div>
            <dt>授权依据引用</dt>
            <dd>{content.basis_ref}</dd>
          </div>
          <div>
            <dt>联系人</dt>
            <dd>{content.contact}</dd>
          </div>
          <div>
            <dt>有效时窗</dt>
            <dd>
              {formatTime(content.valid_from)} 至 {formatTime(content.expires_at)}（不含结束时刻）
            </dd>
          </div>
          <div>
            <dt>请求 / 并发 / 每秒上限</dt>
            <dd>
              {content.budget.max_requests} / {content.budget.max_concurrency} /{' '}
              {content.budget.max_requests_per_second}，实际消耗尚未验证
            </dd>
          </div>
          <div>
            <dt>固定限制</dt>
            <dd>非破坏性 · 禁止重定向 · 无凭据</dd>
          </div>
          <div>
            <dt>解释版本</dt>
            <dd>
              {content.algorithm_version} / {content.policy_version} / {content.catalog_version}
            </dd>
          </div>
        </dl>
        <h3 className="section-title">完整规范范围</h3>
        <ol className="record-list">
          {content.entries.map((entry, index) => (
            <li key={version.entry_ids[index] ?? `${entry.kind}:${entry.value}`}>
              <p>
                <strong>{entry.kind}</strong> · <code>{entry.value}</code>
              </p>
              <p>{entry.include_subdomains ? '包含标签边界内子域' : '不包含额外子域'}</p>
              <p>
                操作：{entry.operations.map((operation) => operationLabels[operation]).join('、')}
              </p>
              <p>端口：{entry.ports.join(', ') || '未声明'}</p>
              <p>解析网络：{entry.resolved_ip_cidrs.join(', ') || '未声明'}</p>
              <p>
                条目 ID：<code>{version.entry_ids[index]}</code>
              </p>
            </li>
          ))}
        </ol>
        <div className="action-row mt-5">
          <Button asChild variant="outline">
            <Link to={`/authorization${listSearch ? `?${listSearch}` : ''}`}>返回授权列表</Link>
          </Button>
          {canCreate && (
            <Button asChild>
              <Link to={`/authorization/new?copy=${version.id}`}>复制为新候选</Link>
            </Button>
          )}
        </div>
        <p className="field-hint">
          候选、过期、被替代或撤销版本不能静态通过。激活/撤销仅由受控管理员 CLI；撤销不回退旧版本。
        </p>
      </CardContent>
    </Card>
  );
}
