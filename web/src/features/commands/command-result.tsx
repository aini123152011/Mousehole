import {
  ArrowRight,
  Check,
  CheckCircle2,
  CircleDashed,
  Clock3,
  FileText,
  Loader2,
  Search,
  XCircle,
} from 'lucide-react';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card';
import type { useCommandWorkspace } from './use-command-workspace';

interface CommandResultProps {
  workspace: ReturnType<typeof useCommandWorkspace>;
  onStartNew: () => void;
}

/** 回执区只展示 API 确认的数据；未找到和连接失败均保留待确认语义。 */
export function CommandResult({ workspace, onStartNew }: CommandResultProps) {
  const { selection, receipt, phase, notice, isQuerying } = workspace;
  const confirmed = receipt !== undefined;
  const rejected = phase === 'rejected' && !confirmed;
  const busy = phase === 'sending' || isQuerying;
  const label = confirmed
    ? '已受理'
    : rejected
      ? '请求被拒绝'
      : busy
        ? '确认中'
        : selection
          ? '受理待确认'
          : '等待提交';
  const StatusIcon = confirmed
    ? CheckCircle2
    : rejected
      ? XCircle
      : busy
        ? Loader2
        : selection
          ? Clock3
          : CircleDashed;

  return (
    <Card className="result-card">
      <CardHeader className="result-heading">
        <CardTitle>
          <h2>受理回执</h2>
        </CardTitle>
        <span>命令受理记录</span>
      </CardHeader>
      <CardContent className="result-content">
        <div
          className={`status-block ${confirmed ? 'status-confirmed' : rejected ? 'status-rejected' : selection ? 'status-pending' : ''}`}
          role="status"
          aria-live="polite"
          aria-atomic="true"
        >
          <div className="status-icon">
            <StatusIcon size={25} className={busy ? 'animate-spin' : ''} aria-hidden="true" />
          </div>
          <h3>{label}</h3>
          <p>
            {confirmed
              ? '服务端已保存这份任务草稿。'
              : rejected
                ? '本次请求未通过受理校验。'
                : phase === 'sending'
                  ? '正在等待服务端的持久化回执。'
                  : isQuerying
                    ? '正在查询原命令，请稍候。'
                    : selection
                      ? '请沿用原命令 ID 查询服务端记录。'
                      : '创建草稿后，确认结果会显示在这里。'}
          </p>
        </div>

        {receipt ? (
          <div className="receipt-details">
            <div className="task-receipt">
              <div className="task-icon">
                <FileText size={18} aria-hidden="true" />
              </div>
              <div>
                <span>已保存的任务</span>
                <h4>{receipt.task_title}</h4>
              </div>
              <Badge className="draft-state" variant="outline">
                草稿
              </Badge>
            </div>
            <dl>
              <div>
                <dt>受理时间</dt>
                <dd>
                  <time dateTime={receipt.accepted_at}>
                    {new Intl.DateTimeFormat('zh-CN', {
                      dateStyle: 'medium',
                      timeStyle: 'medium',
                      timeZone: 'UTC',
                    }).format(new Date(receipt.accepted_at))}{' '}
                    UTC
                  </time>
                </dd>
              </div>
              <div>
                <dt>业务版本</dt>
                <dd>v{receipt.version}</dd>
              </div>
              <div className="stacked-detail">
                <dt>命令 ID</dt>
                <dd>
                  <code>{receipt.command_id}</code>
                </dd>
              </div>
              <div className="stacked-detail">
                <dt>任务 ID</dt>
                <dd>
                  <code>{receipt.task_id}</code>
                </dd>
              </div>
            </dl>
            <p className="receipt-note">
              <Check size={14} aria-hidden="true" />
              已受理表示草稿已保存，任务尚未执行。
            </p>
          </div>
        ) : selection ? (
          <div className="pending-details">
            <div className="stacked-detail">
              <span>查询关联的命令 ID</span>
              <code>{selection.commandId}</code>
            </div>
            {selection.title && (
              <div className="stacked-detail">
                <span>绑定的任务标题</span>
                <strong>{selection.title}</strong>
              </div>
            )}
            <p>提交结果未确认时，请保留当前 ID 和参数。</p>
          </div>
        ) : (
          <div className="empty-receipt">
            <div className="receipt-illustration" aria-hidden="true">
              <div />
              <span />
              <span />
              <span />
            </div>
            <p>一份草稿，一条可追溯的记录。</p>
          </div>
        )}

        {notice && (
          <div className={`result-notice ${rejected ? 'notice-error' : ''}`} role="alert">
            <p>{notice.message}</p>
            {notice.requestId && (
              <small>
                请求关联 ID：<code>{notice.requestId}</code>
              </small>
            )}
          </div>
        )}
        <div className="result-actions">
          {selection && phase !== 'sending' && (
            <Button
              variant="outline"
              disabled={isQuerying}
              onClick={() => {
                void workspace.lookup(selection.commandId);
              }}
            >
              {isQuerying ? (
                <Loader2 className="animate-spin" size={15} aria-hidden="true" />
              ) : (
                <Search size={15} aria-hidden="true" />
              )}
              查询原命令
            </Button>
          )}
          {workspace.canStartNew && (
            <Button variant="ghost" onClick={onStartNew}>
              {selection?.source === 'lookup' ? '返回新建草稿' : '新建另一份草稿'}
              <ArrowRight size={15} aria-hidden="true" />
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
