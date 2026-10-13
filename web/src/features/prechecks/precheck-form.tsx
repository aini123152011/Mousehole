import { useEffect } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from '../../components/ui/card';
import type { ScopeCommandWorkspace } from '../scope/use-scope-command';
import { operationLabels } from '../scope/presentation';

const precheckSchema = z
  .object({
    versionId: z
      .string()
      .trim()
      .regex(
        /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
        '请输入规范授权版本 UUID。',
      ),
    operation: z.enum(['dns_lookup', 'tcp_connect', 'http_metadata']),
    target: z.string().trim().min(1, '请填写候选目标。').max(2048, '目标最多 2048 个字符。'),
    port: z.string(),
  })
  .superRefine((value, context) => {
    if (
      value.operation === 'tcp_connect' &&
      (!/^[0-9]+$/.test(value.port) || Number(value.port) < 1 || Number(value.port) > 65535)
    )
      context.addIssue({
        code: 'custom',
        path: ['port'],
        message: 'TCP 需要 1–65535 的显式端口。',
      });
  });

/** 只提交静态语义输入；未知结果锁定原版本、目标、参数，重新检查需显式新命令。 */
export function PrecheckForm({
  currentVersionId,
  canCheck,
  command,
}: {
  currentVersionId: string | null;
  canCheck: boolean;
  command: ScopeCommandWorkspace;
}) {
  const original =
    command.selection?.body?.kind === 'precheck_target' ? command.selection.body.payload : null;
  const form = useForm({
    resolver: zodResolver(precheckSchema),
    defaultValues: {
      versionId: original?.authorization_version_id ?? currentVersionId ?? '',
      operation: original?.operation ?? 'dns_lookup',
      target: original?.target ?? '',
      port: original?.parameters.port === undefined ? '443' : String(original.parameters.port),
    },
  });
  const operation = useWatch({ control: form.control, name: 'operation' });
  const { getValues, setValue } = form;
  useEffect(() => {
    if (currentVersionId && !getValues('versionId')) setValue('versionId', currentVersionId);
  }, [currentVersionId, getValues, setValue]);
  const disabled = command.isLocked || !canCheck;
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>候选目标预检查</h2>
        </CardTitle>
        <CardDescription>
          纯本地检查，不查询目标 DNS、不发 TCP/HTTP；结果不能作为执行许可。
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!canCheck && (
          <p className="permission-note">当前只读，可查看历史；创建预检查需要分析员能力。</p>
        )}
        <form
          className="scope-form"
          noValidate
          onSubmit={form.handleSubmit(async (value) => {
            await command.submit({
              kind: 'precheck_target',
              payload: {
                authorization_version_id: value.versionId,
                operation: value.operation,
                target: value.target,
                parameters: value.operation === 'tcp_connect' ? { port: Number(value.port) } : {},
              },
            });
          })}
        >
          <div>
            <label htmlFor="precheck-version">授权版本 ID</label>
            <Input
              id="precheck-version"
              autoComplete="off"
              spellCheck={false}
              disabled={disabled}
              aria-invalid={Boolean(form.formState.errors.versionId)}
              aria-describedby="precheck-version-hint precheck-version-error"
              {...form.register('versionId')}
            />
            <p id="precheck-version-hint" className="field-hint">
              默认填当前激活版本；也可检查项目内候选或历史版本，查看被拒原因。
            </p>
            {form.formState.errors.versionId && (
              <p className="field-error" id="precheck-version-error" role="alert">
                {form.formState.errors.versionId.message}
              </p>
            )}
          </div>
          <div>
            <label htmlFor="precheck-operation">预检查操作</label>
            <select id="precheck-operation" disabled={disabled} {...form.register('operation')}>
              {Object.entries(operationLabels).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="precheck-target">候选目标</label>
            <Input
              id="precheck-target"
              disabled={disabled}
              autoComplete="off"
              spellCheck={false}
              aria-invalid={Boolean(form.formState.errors.target)}
              aria-describedby="precheck-target-hint precheck-target-error"
              {...form.register('target')}
            />
            <p id="precheck-target-hint" className="field-hint">
              {operation === 'http_metadata'
                ? '绝对 HTTP(S) URL；精确匹配协议、端口、路径和 query。HEAD，无凭据，禁止重定向。'
                : operation === 'tcp_connect'
                  ? '域名或规范 IP。端口和解析网络须由同一条范围完整声明。'
                  : '仅域名，固定 A/AAAA 语义；此操作不会实际解析。'}
            </p>
            {form.formState.errors.target && (
              <p className="field-error" id="precheck-target-error" role="alert">
                {form.formState.errors.target.message}
              </p>
            )}
          </div>
          {operation === 'tcp_connect' && (
            <div>
              <label htmlFor="precheck-port">目标端口</label>
              <Input
                id="precheck-port"
                inputMode="numeric"
                disabled={disabled}
                aria-invalid={Boolean(form.formState.errors.port)}
                aria-describedby="precheck-port-error"
                {...form.register('port')}
              />
              {form.formState.errors.port && (
                <p id="precheck-port-error" className="field-error" role="alert">
                  {form.formState.errors.port.message}
                </p>
              )}
            </div>
          )}
          <Button type="submit" disabled={disabled}>
            检查静态条件
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
