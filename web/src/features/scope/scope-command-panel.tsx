import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from 'react-router-dom';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from '../../components/ui/card';
import { commandLookupSchema } from '../commands/command-schema';
import type { ScopeCommandWorkspace } from './use-scope-command';
import { formatTime } from './presentation';

/** 跨资产/范围/预检查页面保留原命令恢复入口；未知提交期间仅查询，不重发。 */
export function ScopeCommandPanel({ workspace }: { workspace: ScopeCommandWorkspace }) {
  const lookup = useForm({
    resolver: zodResolver(commandLookupSchema),
    defaultValues: { commandId: '' },
  });
  const result = workspace.receipt?.result;
  return (
    <Card className="scope-command-panel">
      <CardHeader>
        <CardTitle>
          <h2>原命令回执</h2>
        </CardTitle>
        <CardDescription>原 ID 查询只读取已保存记录；恢复不会重新检查当前规则。</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="lookup-row"
          onSubmit={lookup.handleSubmit(async ({ commandId }) => workspace.lookup(commandId))}
          noValidate
        >
          <div className="grow">
            <label htmlFor="scope-lookup-id">范围命令 ID</label>
            <Input
              id="scope-lookup-id"
              autoComplete="off"
              spellCheck={false}
              disabled={!workspace.canLookup}
              aria-invalid={Boolean(lookup.formState.errors.commandId)}
              aria-describedby="scope-lookup-error"
              {...lookup.register('commandId')}
            />
            {lookup.formState.errors.commandId && (
              <p id="scope-lookup-error" className="field-error" role="alert">
                {lookup.formState.errors.commandId.message}
              </p>
            )}
          </div>
          <Button
            type="submit"
            variant="outline"
            disabled={!workspace.canLookup || workspace.isQuerying}
          >
            查询原回执
          </Button>
        </form>
        {workspace.selection && (
          <div className="command-reference">
            <p>当前范围命令 ID</p>
            <code data-testid="scope-command-id">{workspace.selection.commandId}</code>
            <p>保留此 ID，刷新后可在本项目恢复。目标输入和回执不保存在浏览器持久存储。</p>
          </div>
        )}
        <div className="scope-status" role="status" aria-live="polite">
          {workspace.phase === 'sending' && <strong>正在提交，ID 与参数已锁定</strong>}
          {workspace.phase === 'unknown' && <strong>受理待确认</strong>}
          {workspace.phase === 'rejected' && <strong>本次命令明确拒绝，编辑内容已保留</strong>}
          {workspace.phase === 'accepted' && <strong>原命令已受理</strong>}
          {workspace.notice && <p>{workspace.notice}</p>}
        </div>
        {workspace.selection && workspace.phase !== 'sending' && (
          <Button
            variant="outline"
            disabled={workspace.isQuerying}
            onClick={() => {
              void workspace.lookup(workspace.selection?.commandId ?? '');
            }}
          >
            按当前 ID 查询
          </Button>
        )}
        {workspace.receipt && (
          <div className="receipt-summary">
            <p>原受理时刻：{formatTime(workspace.receipt.accepted_at)}</p>
            {result?.type === 'assets_imported' && (
              <p>
                输入 {result.input_count} 行，唯一 {result.unique_count} 条，本批重复{' '}
                {result.duplicate_count} 条，项目已有 {result.existing_count}{' '}
                条。来源已保存，仅代表导入声明。
              </p>
            )}
            {result?.type === 'authorization_version' && (
              <p>
                候选版本 {result.version.sequence} 已保存。
                <Link to={`/authorization/${result.version.id}`}>查看不可变内容</Link>；激活需管理员
                CLI。
              </p>
            )}
            {result?.type === 'precheck' && (
              <p>
                {result.decision.outcome === 'deny'
                  ? '静态规则拒绝'
                  : '静态规则匹配，执行条件尚未验证'}
                。<Link to={`/prechecks/${result.decision.id}`}>查看原检查解释</Link>
              </p>
            )}
            <p>此处为历史回执，不表示现在仍有效或已经执行。</p>
          </div>
        )}
        {workspace.canReset && (
          <Button className="mt-4" variant="secondary" onClick={workspace.reset}>
            结束本次命令，继续编辑
          </Button>
        )}
        {workspace.phase === 'unknown' && workspace.selection?.source === 'submission' && (
          <p className="field-hint">结果未确认前不能新建命令或改变绑定参数。</p>
        )}
      </CardContent>
    </Card>
  );
}
