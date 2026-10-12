import { useLayoutEffect, useRef, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  ArrowRight,
  Check,
  Clipboard,
  FilePlus2,
  Info,
  Loader2,
  Search,
  ShieldCheck,
} from 'lucide-react';
import type { DevelopmentSession } from '../../api/client';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '../../components/ui/card';
import { Badge } from '../../components/ui/badge';
import { commandLookupSchema, normalizeTaskTitle, taskDraftSchema } from './command-schema';
import { useCommandWorkspace } from './use-command-workspace';
import { CommandResult } from './command-result';

/** 工作台依赖服务端校验过的当前开发项目。 */
interface CommandWorkspaceProps {
  session: DevelopmentSession;
}

/** 提供草稿创建与既有命令恢复；未知提交期间锁定参数并保留 ID。 */
export function CommandWorkspace({ session }: CommandWorkspaceProps) {
  const workspace = useCommandWorkspace(session.project_id);
  const [copyNotice, setCopyNotice] = useState('');
  const draft = useForm({ resolver: zodResolver(taskDraftSchema), defaultValues: { title: '' } });
  const { setFocus } = draft;
  const focusAfterUnlock = useRef(false);
  const lookup = useForm({
    resolver: zodResolver(commandLookupSchema),
    defaultValues: { commandId: '' },
  });
  const title = useWatch({ control: draft.control, name: 'title' });
  const length = Array.from(normalizeTaskTitle(title)).length;

  useLayoutEffect(() => {
    // 原输入在锁定状态不能接收焦点；等待解锁渲染完成再恢复键盘工作位置。
    if (!workspace.isFormLocked && focusAfterUnlock.current) {
      focusAfterUnlock.current = false;
      setFocus('title');
    }
  }, [workspace.isFormLocked, setFocus]);

  async function copyCommandId() {
    if (!workspace.selection) return;
    try {
      await navigator.clipboard.writeText(workspace.selection.commandId);
      setCopyNotice('命令 ID 已复制。');
    } catch {
      setCopyNotice('暂时无法复制，请选择下方命令 ID 手动复制。');
    }
  }

  function startNewDraft() {
    if (!workspace.canStartNew) return;
    focusAfterUnlock.current = true;
    if (workspace.phase === 'accepted') draft.reset({ title: '' });
    workspace.startNewDraft();
    setCopyNotice('');
  }

  return (
    <>
      <div className="workbench-grid">
        <div className="input-column">
          <Card className="draft-card">
            <CardHeader className="card-heading">
              <div className="heading-icon">
                <FilePlus2 size={20} aria-hidden="true" />
              </div>
              <div>
                <CardTitle>
                  <h2>新建任务草稿</h2>
                </CardTitle>
                <CardDescription>给任务一个明确的名字，开始整理你的工作。</CardDescription>
              </div>
              <Badge variant="secondary" className="quiet-badge">
                草稿
              </Badge>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={draft.handleSubmit(async ({ title: value }) => {
                  await workspace.createDraft(value);
                })}
                noValidate
              >
                <div className="field-heading">
                  <label htmlFor="task-title">
                    任务标题<span aria-hidden="true"> *</span>
                  </label>
                  <span className={length > 120 ? 'invalid-count' : ''}>{length} / 120</span>
                </div>
                <Input
                  id="task-title"
                  className="title-input"
                  placeholder="例如：内部平台评估准备"
                  autoComplete="off"
                  disabled={workspace.isFormLocked}
                  aria-invalid={Boolean(draft.formState.errors.title)}
                  aria-describedby={draft.formState.errors.title ? 'title-error' : 'title-hint'}
                  {...draft.register('title')}
                />
                {draft.formState.errors.title ? (
                  <p className="field-error" id="title-error" role="alert">
                    {draft.formState.errors.title.message}
                  </p>
                ) : (
                  <p className="field-hint" id="title-hint">
                    1–120 个字符。提交后，标题会与命令 ID 绑定。
                  </p>
                )}
                <div className="draft-boundary">
                  <ShieldCheck size={19} aria-hidden="true" />
                  <div>
                    <strong>此操作只创建草稿</strong>
                    <p>草稿不会发起扫描、目标请求或 Agent 执行。</p>
                  </div>
                </div>
                {workspace.selection?.source === 'submission' && (
                  <div className="command-reference">
                    <div>
                      <span>本次命令 ID</span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          void copyCommandId();
                        }}
                        aria-label="复制命令 ID"
                      >
                        <Clipboard size={14} aria-hidden="true" />
                        复制
                      </Button>
                    </div>
                    <code data-testid="current-command-id">{workspace.selection.commandId}</code>
                    <p>保留此 ID，刷新页面后也可以查询原回执。</p>
                  </div>
                )}
                <div className="form-footer">
                  <span>
                    <Info size={14} aria-hidden="true" />
                    仅保存到当前项目
                  </span>
                  <Button type="submit" disabled={workspace.isFormLocked}>
                    {workspace.phase === 'sending' ? (
                      <>
                        <Loader2 className="animate-spin" size={16} aria-hidden="true" />
                        正在提交
                      </>
                    ) : (
                      <>
                        创建草稿
                        <ArrowRight size={16} aria-hidden="true" />
                      </>
                    )}
                  </Button>
                </div>
              </form>
              <p className="sr-only" role="status">
                {copyNotice}
              </p>
            </CardContent>
          </Card>

          <Card className="lookup-card">
            <CardHeader>
              <CardTitle>
                <h2>
                  <Search size={17} aria-hidden="true" />
                  找回已有命令
                </h2>
              </CardTitle>
              <CardDescription>提交响应丢失或刷新页面后，输入原命令 ID 查询。</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={lookup.handleSubmit(async ({ commandId }) => {
                  await workspace.lookup(commandId);
                })}
                noValidate
              >
                <label htmlFor="lookup-command-id" className="sr-only">
                  已有命令 ID
                </label>
                <div className="lookup-row">
                  <Input
                    id="lookup-command-id"
                    className="mono-input"
                    placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                    autoComplete="off"
                    spellCheck={false}
                    disabled={!workspace.canChooseLookup}
                    aria-invalid={Boolean(lookup.formState.errors.commandId)}
                    aria-describedby={
                      lookup.formState.errors.commandId ? 'lookup-error' : 'lookup-hint'
                    }
                    {...lookup.register('commandId')}
                  />
                  <Button type="submit" variant="outline" disabled={!workspace.canChooseLookup}>
                    查询回执
                    <Search size={15} aria-hidden="true" />
                  </Button>
                </div>
                {lookup.formState.errors.commandId && (
                  <p id="lookup-error" className="field-error" role="alert">
                    {lookup.formState.errors.commandId.message}
                  </p>
                )}
                <p id="lookup-hint" className="field-hint">
                  查询只读取原记录，不会再次创建草稿。
                </p>
              </form>
            </CardContent>
          </Card>
        </div>

        <CommandResult workspace={workspace} onStartNew={startNewDraft} />
      </div>

      <div className="principles-strip">
        <div>
          <span className="principle-index">01</span>
          <div>
            <strong>清晰的任务标题</strong>
            <p>为工作保留明确的起点</p>
          </div>
        </div>
        <div>
          <span className="principle-index">02</span>
          <div>
            <strong>可确认的结果</strong>
            <p>回执显示草稿已保存</p>
          </div>
        </div>
        <div>
          <span className="principle-index">03</span>
          <div>
            <strong>随时找回记录</strong>
            <p>保留命令 ID，刷新后查询</p>
          </div>
        </div>
        <Check size={18} aria-hidden="true" />
      </div>
    </>
  );
}
