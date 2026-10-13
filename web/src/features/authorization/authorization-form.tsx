import { useEffect } from 'react';
import { useFieldArray, useForm, useWatch, type FieldPath } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import type { AuthorizationContent } from '../../api/client';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '../../components/ui/card';
import type { ScopeCommandWorkspace } from '../scope/use-scope-command';
import { operationLabels } from '../scope/presentation';
import { normalizeTaskTitle } from '../commands/command-schema';

const boundedText = (max: number, label: string) =>
  z
    .string()
    .refine((value) => !/\p{Cc}/u.test(value), `${label}不能包含控制字符。`)
    .transform(normalizeTaskTitle)
    .refine((value) => value.length > 0, `请填写${label}。`)
    .refine((value) => Array.from(value).length <= max, `${label}最多 ${max} 个字符。`);
const positiveInteger = z
  .string()
  .regex(/^[1-9][0-9]*$/, '请输入正整数字符串。')
  .refine(
    (value) => !/^[1-9][0-9]*$/.test(value) || BigInt(value) <= 9223372036854775807n,
    '不能超过数据库整数上限。',
  );
const entrySchema = z
  .object({
    kind: z.enum(['domain', 'ip', 'cidr', 'url_exact']),
    value: boundedText(2048, '范围目标'),
    includeSubdomains: z.boolean(),
    operations: z
      .array(z.enum(['dns_lookup', 'tcp_connect', 'http_metadata']))
      .min(1, '请选择至少一种操作。'),
    portsText: z.string(),
    resolvedCidrsText: z.string(),
  })
  .superRefine((entry, context) => {
    const ports = portValues(entry.portsText);
    if (
      ports.length > 32 ||
      ports.some((value) => !Number.isInteger(value) || value < 1 || value > 65535)
    )
      context.addIssue({
        code: 'custom',
        path: ['portsText'],
        message: '端口为 1–65535，最多 32 个，用逗号分隔。',
      });
    if (
      (entry.operations.includes('tcp_connect') || entry.operations.includes('http_metadata')) &&
      ports.length === 0
    )
      context.addIssue({
        code: 'custom',
        path: ['portsText'],
        message: 'TCP/HTTP 必须显式声明端口。',
      });
    if (cidrValues(entry.resolvedCidrsText).length > 32)
      context.addIssue({
        code: 'custom',
        path: ['resolvedCidrsText'],
        message: '解析 CIDR 最多 32 条。',
      });
  });
const formSchema = z
  .object({
    title: boundedText(120, '授权标题'),
    basisRef: boundedText(500, '授权依据引用'),
    contact: boundedText(200, '联系人'),
    validFrom: z
      .string()
      .refine((value) => Number.isFinite(Date.parse(value)), '请填写有效的开始时间。'),
    expiresAt: z
      .string()
      .refine((value) => Number.isFinite(Date.parse(value)), '请填写有效的结束时间。'),
    maxRequests: positiveInteger,
    maxConcurrency: positiveInteger,
    maxRequestsPerSecond: positiveInteger,
    entries: z.array(entrySchema).min(1).max(100, '每版本最多 100 条范围。'),
  })
  .superRefine((value, context) => {
    if (Date.parse(value.validFrom) >= Date.parse(value.expiresAt))
      context.addIssue({
        code: 'custom',
        path: ['expiresAt'],
        message: '结束时间必须晚于开始时间。',
      });
    if (
      /^[1-9][0-9]*$/.test(value.maxConcurrency) &&
      /^[1-9][0-9]*$/.test(value.maxRequests) &&
      BigInt(value.maxConcurrency) > BigInt(value.maxRequests)
    )
      context.addIssue({
        code: 'custom',
        path: ['maxConcurrency'],
        message: '并发上限不能超过请求总量。',
      });
  });

/** 仅映射已知合同路径，不把服务端任意字符串当作表单内部字段。 */
function authorizationField(path: string): FieldPath<z.infer<typeof formSchema>> | null {
  const topFields = {
    'payload.title': 'title',
    'payload.basis_ref': 'basisRef',
    'payload.contact': 'contact',
    'payload.valid_from': 'validFrom',
    'payload.expires_at': 'expiresAt',
    'payload.budget.max_requests': 'maxRequests',
    'payload.budget.max_concurrency': 'maxConcurrency',
    'payload.budget.max_requests_per_second': 'maxRequestsPerSecond',
    'payload.entries': 'entries',
  } as const;
  for (const [server, field] of Object.entries(topFields)) if (server === path) return field;
  const entry =
    /^payload\.entries\.([0-9]{1,2})\.(value|ports|operations|resolved_ip_cidrs)(?:\.[0-9]{1,2})?$/.exec(
      path,
    );
  if (!entry) return null;
  const index = Number(entry[1]);
  switch (entry[2]) {
    case 'value':
      return `entries.${index}.value`;
    case 'ports':
      return `entries.${index}.portsText`;
    case 'operations':
      return `entries.${index}.operations`;
    case 'resolved_ip_cidrs':
      return `entries.${index}.resolvedCidrsText`;
    default:
      return null;
  }
}

function fieldLabel(path: string) {
  const field = authorizationField(path);
  const labels: Partial<Record<FieldPath<z.infer<typeof formSchema>>, string>> = {
    title: '授权标题',
    basisRef: '授权依据引用',
    contact: '联系人',
    validFrom: '有效开始',
    expiresAt: '有效结束',
    maxRequests: '请求总量上限',
    maxConcurrency: '并发上限',
    maxRequestsPerSecond: '每秒请求上限',
    entries: '结构化范围',
  };
  if (field && labels[field]) return labels[field];
  const entry = /^payload\.entries\.([0-9]{1,2})\./.exec(path);
  return entry ? `范围 ${Number(entry[1]) + 1}` : '授权范围';
}

function portValues(value: string) {
  return value.trim()
    ? value
        .split(/[,，\s]+/)
        .filter(Boolean)
        .map(Number)
    : [];
}
function cidrValues(value: string) {
  return value.split(/[,，\s]+/).filter(Boolean);
}
function localTime(value: string) {
  const date = new Date(value);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
}
const blankEntry: z.infer<typeof entrySchema> = {
  kind: 'domain',
  value: '',
  includeSubdomains: false,
  operations: ['dns_lookup'],
  portsText: '',
  resolvedCidrsText: '',
};

function defaults(initial?: AuthorizationContent): z.infer<typeof formSchema> {
  return {
    title: initial ? `${initial.title}` : '',
    basisRef: initial?.basis_ref ?? '',
    contact: initial?.contact ?? '',
    validFrom: localTime(initial?.valid_from ?? new Date(Date.now() - 60_000).toISOString()),
    expiresAt: localTime(initial?.expires_at ?? new Date(Date.now() + 86_400_000).toISOString()),
    maxRequests: initial?.budget.max_requests ?? '100',
    maxConcurrency: initial?.budget.max_concurrency ?? '2',
    maxRequestsPerSecond: initial?.budget.max_requests_per_second ?? '1',
    entries: initial?.entries.map((entry) => ({
      kind: entry.kind,
      value: entry.value,
      includeSubdomains: entry.include_subdomains,
      operations: entry.operations,
      portsText: entry.ports.join(', '),
      resolvedCidrsText: entry.resolved_ip_cidrs.join('\n'),
    })) ?? [{ ...blankEntry }],
  };
}

/** 候选表单是独立输入草稿；保存后不会原地修改旧版本，冲突和明确拒绝保留编辑内容。 */
export function AuthorizationForm({
  initial,
  canCreate,
  command,
}: {
  initial?: AuthorizationContent;
  canCreate: boolean;
  command: ScopeCommandWorkspace;
}) {
  const original =
    command.selection?.body?.kind === 'create_authorization_version'
      ? command.selection.body.payload
      : initial;
  const form = useForm({ resolver: zodResolver(formSchema), defaultValues: defaults(original) });
  const { setError } = form;
  useEffect(() => {
    if (command.selection?.body?.kind !== 'create_authorization_version') return;
    for (const issue of command.validationErrors) {
      const field = authorizationField(issue.field);
      if (field) setError(field, { type: 'server', message: issue.message });
    }
  }, [command.selection?.body?.kind, command.validationErrors, setError]);
  const watchedEntries = useWatch({ control: form.control, name: 'entries' });
  const entries = useFieldArray({ control: form.control, name: 'entries' });
  const disabled = command.isLocked || !canCreate;
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{initial ? '复制为新候选版本' : '编写候选授权版本'}</h2>
        </CardTitle>
        <CardDescription>
          候选保存后内容不可编辑，需管理员显式激活；新版本不会自动替代当前授权。
        </CardDescription>
      </CardHeader>
      <CardContent>
        {command.selection?.body?.kind === 'create_authorization_version' &&
          command.validationErrors.length > 0 && (
            <div className="boundary-note" role="alert" aria-label="候选字段校验说明">
              <strong>服务端校验未通过，原输入已保留</strong>
              {command.validationErrors.map((issue) => (
                <p key={issue.field}>
                  {fieldLabel(issue.field)}：{issue.message}
                </p>
              ))}
              <p>结束本次命令后修正对应字段，再保存为新的候选版本。</p>
            </div>
          )}
        {!canCreate && (
          <p className="permission-note">只读成员可查看历史，保存候选需要项目分析员能力。</p>
        )}
        <form
          noValidate
          className="scope-form"
          onSubmit={form.handleSubmit(async (value) => {
            const payload: AuthorizationContent = {
              title: value.title,
              basis_ref: value.basisRef,
              contact: value.contact,
              valid_from: new Date(value.validFrom).toISOString(),
              expires_at: new Date(value.expiresAt).toISOString(),
              algorithm_version: 'target-v1',
              policy_version: 'policy-v1',
              catalog_version: 'precheck-v1',
              max_operation_level: 'non_destructive',
              redirect_policy: 'deny',
              credential_policy: 'none',
              budget: {
                max_requests: value.maxRequests,
                max_concurrency: value.maxConcurrency,
                max_requests_per_second: value.maxRequestsPerSecond,
              },
              entries: value.entries.map((entry) => ({
                kind: entry.kind,
                value: entry.value,
                include_subdomains: entry.includeSubdomains,
                operations: entry.operations,
                ports: portValues(entry.portsText),
                resolved_ip_cidrs: cidrValues(entry.resolvedCidrsText),
              })),
            };
            await command.submit({ kind: 'create_authorization_version', payload });
          })}
        >
          <div>
            <label htmlFor="authorization-title">授权标题</label>
            <Input
              id="authorization-title"
              disabled={disabled}
              autoComplete="off"
              aria-invalid={Boolean(form.formState.errors.title)}
              aria-describedby="authorization-title-error"
              {...form.register('title')}
            />
            {form.formState.errors.title && (
              <p id="authorization-title-error" className="field-error" role="alert">
                {form.formState.errors.title.message}
              </p>
            )}
          </div>
          <div className="form-grid">
            <div>
              <label htmlFor="authorization-basis">授权依据引用</label>
              <Input
                id="authorization-basis"
                disabled={disabled}
                autoComplete="off"
                aria-invalid={Boolean(form.formState.errors.basisRef)}
                aria-describedby="authorization-basis-error"
                {...form.register('basisRef')}
              />
              {form.formState.errors.basisRef && (
                <p id="authorization-basis-error" className="field-error" role="alert">
                  {form.formState.errors.basisRef.message}
                </p>
              )}
            </div>
            <div>
              <label htmlFor="authorization-contact">联系人</label>
              <Input
                id="authorization-contact"
                disabled={disabled}
                autoComplete="off"
                aria-invalid={Boolean(form.formState.errors.contact)}
                aria-describedby="authorization-contact-error"
                {...form.register('contact')}
              />
              {form.formState.errors.contact && (
                <p id="authorization-contact-error" className="field-error" role="alert">
                  {form.formState.errors.contact.message}
                </p>
              )}
            </div>
          </div>
          <p className="field-hint">依据引用为人工确认信息，平台未验证法律效力；不要填写凭据。</p>
          <div className="form-grid">
            <div>
              <label htmlFor="authorization-from">有效开始（本机时区）</label>
              <Input
                id="authorization-from"
                type="datetime-local"
                disabled={disabled}
                aria-invalid={Boolean(form.formState.errors.validFrom)}
                aria-describedby="authorization-from-error"
                {...form.register('validFrom')}
              />
              {form.formState.errors.validFrom && (
                <p id="authorization-from-error" className="field-error" role="alert">
                  {form.formState.errors.validFrom.message}
                </p>
              )}
            </div>
            <div>
              <label htmlFor="authorization-until">有效结束（本机时区）</label>
              <Input
                id="authorization-until"
                type="datetime-local"
                disabled={disabled}
                aria-invalid={Boolean(form.formState.errors.expiresAt)}
                aria-describedby="authorization-until-error"
                {...form.register('expiresAt')}
              />
              {form.formState.errors.expiresAt && (
                <p id="authorization-until-error" className="field-error" role="alert">
                  {form.formState.errors.expiresAt.message}
                </p>
              )}
            </div>
          </div>
          <div className="form-grid budgets">
            {(
              [
                ['maxRequests', '请求总量上限'],
                ['maxConcurrency', '并发上限'],
                ['maxRequestsPerSecond', '每秒请求上限'],
              ] as const
            ).map(([name, label]) => (
              <div key={name}>
                <label htmlFor={`authorization-${name}`}>{label}</label>
                <Input
                  id={`authorization-${name}`}
                  inputMode="numeric"
                  disabled={disabled}
                  aria-invalid={Boolean(form.formState.errors[name])}
                  aria-describedby={`authorization-${name}-error`}
                  {...form.register(name)}
                />
                {form.formState.errors[name] && (
                  <p id={`authorization-${name}-error`} className="field-error" role="alert">
                    {form.formState.errors[name]?.message}
                  </p>
                )}
              </div>
            ))}
          </div>
          <p className="field-hint">这里只保存有单位的硬上限；尚未检查实际消耗、限流或执行预算。</p>
          <div className="entry-heading">
            <h3>结构化范围（{entries.fields.length} 条）</h3>
            <Button
              type="button"
              variant="outline"
              disabled={disabled || entries.fields.length >= 100}
              onClick={() => entries.append({ ...blankEntry })}
            >
              添加范围
            </Button>
          </div>
          {entries.fields.map((entry, index) => (
            <fieldset key={entry.id} disabled={disabled} className="scope-entry">
              <legend>范围 {index + 1}</legend>
              <div className="form-grid">
                <div>
                  <label htmlFor={`entry-${entry.id}-kind`}>范围类型 {index + 1}</label>
                  <select
                    id={`entry-${entry.id}-kind`}
                    {...form.register(`entries.${index}.kind`, {
                      onChange: (event: { target: { value: string } }) => {
                        if (event.target.value !== 'domain')
                          form.setValue(`entries.${index}.includeSubdomains`, false);
                      },
                    })}
                  >
                    <option value="domain">域名（标签边界）</option>
                    <option value="ip">IP 地址</option>
                    <option value="cidr">CIDR 网络</option>
                    <option value="url_exact">精确 HTTP(S) URL</option>
                  </select>
                </div>
                <div>
                  <label htmlFor={`entry-${entry.id}-value`}>范围目标 {index + 1}</label>
                  <Input
                    id={`entry-${entry.id}-value`}
                    autoComplete="off"
                    aria-invalid={Boolean(form.formState.errors.entries?.[index]?.value)}
                    aria-describedby={`entry-${entry.id}-value-error`}
                    {...form.register(`entries.${index}.value`)}
                  />
                  {form.formState.errors.entries?.[index]?.value && (
                    <p id={`entry-${entry.id}-value-error`} className="field-error" role="alert">
                      {form.formState.errors.entries[index]?.value?.message}
                    </p>
                  )}
                </div>
              </div>
              <label className="checkbox-label">
                <input
                  type="checkbox"
                  disabled={disabled || watchedEntries[index]?.kind !== 'domain'}
                  {...form.register(`entries.${index}.includeSubdomains`)}
                />
                包含子域（仅域名规则可用）
              </label>
              <div
                className="operation-options"
                role="group"
                aria-label={`可用操作 ${index + 1}`}
                aria-describedby={`entry-${entry.id}-operations-error`}
              >
                {Object.entries(operationLabels).map(([operation, label]) => (
                  <label key={operation} className="checkbox-label">
                    <input
                      type="checkbox"
                      value={operation}
                      {...form.register(`entries.${index}.operations`)}
                    />
                    {label}
                  </label>
                ))}
              </div>
              {form.formState.errors.entries?.[index]?.operations && (
                <p id={`entry-${entry.id}-operations-error`} className="field-error" role="alert">
                  {form.formState.errors.entries[index]?.operations?.message}
                </p>
              )}
              <div className="form-grid">
                <div>
                  <label htmlFor={`entry-${entry.id}-ports`}>允许端口 {index + 1}</label>
                  <Input
                    id={`entry-${entry.id}-ports`}
                    placeholder="例如 443, 8443"
                    aria-invalid={Boolean(form.formState.errors.entries?.[index]?.portsText)}
                    aria-describedby={`entry-${entry.id}-ports-error`}
                    {...form.register(`entries.${index}.portsText`)}
                  />
                  {form.formState.errors.entries?.[index]?.portsText && (
                    <p id={`entry-${entry.id}-ports-error`} className="field-error" role="alert">
                      {form.formState.errors.entries[index]?.portsText?.message}
                    </p>
                  )}
                </div>
                <div>
                  <label htmlFor={`entry-${entry.id}-resolved`}>允许解析 CIDR {index + 1}</label>
                  <textarea
                    className="scope-textarea"
                    id={`entry-${entry.id}-resolved`}
                    rows={2}
                    placeholder="每行一个规范网络，如 10.20.0.0/16"
                    autoComplete="off"
                    aria-invalid={Boolean(
                      form.formState.errors.entries?.[index]?.resolvedCidrsText,
                    )}
                    aria-describedby={`entry-${entry.id}-resolved-error`}
                    {...form.register(`entries.${index}.resolvedCidrsText`)}
                  />
                  {form.formState.errors.entries?.[index]?.resolvedCidrsText && (
                    <p id={`entry-${entry.id}-resolved-error`} className="field-error" role="alert">
                      {form.formState.errors.entries[index]?.resolvedCidrsText?.message}
                    </p>
                  )}
                </div>
              </div>
              <p className="field-hint">
                TCP/HTTP 显式列端口；域名连接需在同条规则声明解析 CIDR。CIDR
                含主机位会被拒绝；不同范围不能拼接权限。
              </p>
              <Button
                type="button"
                variant="ghost"
                disabled={entries.fields.length <= 1}
                onClick={() => entries.remove(index)}
              >
                移除范围 {index + 1}
              </Button>
            </fieldset>
          ))}
          <div className="boundary-note">
            <strong>非破坏性上限 · 禁止重定向 · 无凭据</strong>
            <p>
              固定 target-v1 / policy-v1 / precheck-v1。保存与匹配均由服务端复核，不发起目标请求。
            </p>
          </div>
          <Button type="submit" disabled={disabled}>
            保存新候选版本
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
