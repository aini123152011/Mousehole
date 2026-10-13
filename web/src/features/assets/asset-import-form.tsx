import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { previewAssets, type AssetImportPayload } from '../../api/client';
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
import { useProjectQuery } from '../scope/use-project-query';
import { assetLabels } from '../scope/presentation';

const inputSchema = z
  .object({ kind: z.enum(['domain', 'ip', 'url']), value: z.string().min(1).max(2048) })
  .strict();
const importSchema = z
  .object({
    source: z.enum(['manual', 'api_import']),
    kind: z.enum(['domain', 'ip', 'url']),
    text: z.string().trim().min(1, '请填写至少一条资产。'),
    notes: z.string().max(500, '来源备注最多 500 个字符。'),
    externalRef: z.string().max(500, '外部引用最多 500 个字符。'),
  })
  .superRefine((value, context) => {
    try {
      parseEntries(value);
    } catch (error: unknown) {
      context.addIssue({
        code: 'custom',
        path: ['text'],
        message: error instanceof Error ? error.message : '导入内容无效。',
      });
    }
  });

function parseEntries(value: {
  source: 'manual' | 'api_import';
  kind: 'domain' | 'ip' | 'url';
  text: string;
}): AssetImportPayload['entries'] {
  if (value.source === 'manual') {
    const rows = value.text
      .trim()
      .split(/\r?\n/)
      .map((line) => ({ kind: value.kind, value: line.trim() }));
    if (rows.length > 100) throw new Error('每批最多 100 行，请拆分导入。');
    return rows;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(value.text);
  } catch {
    throw new Error('API 来源请填写 JSON 数组，每项包含 kind 和 value。');
  }
  const result = z.array(inputSchema).min(1).max(100).safeParse(parsed);
  if (!result.success)
    throw new Error(
      'JSON 必须是 1–100 个 kind/value 对象；类型为 domain、ip 或 url，不能包含额外字段。',
    );
  return result.data;
}

/** 手动与 API 来源共用服务端预览；输入变化时旧预览失效，不跳过错误行。 */
export function AssetImportForm({
  actorId,
  projectId,
  canImport,
  command,
}: {
  actorId: string;
  projectId: string;
  canImport: boolean;
  command: ScopeCommandWorkspace;
}) {
  const original =
    command.selection?.body?.kind === 'import_assets' ? command.selection.body.payload : null;
  const form = useForm({
    resolver: zodResolver(importSchema),
    defaultValues: {
      source: original?.source ?? 'manual',
      kind: original?.entries[0]?.kind ?? 'domain',
      text: original
        ? original.source === 'manual'
          ? original.entries.map((entry) => entry.value).join('\n')
          : JSON.stringify(original.entries, null, 2)
        : '',
      notes: original?.notes ?? '',
      externalRef: original?.external_ref ?? '',
    },
  });
  const values = useWatch({ control: form.control });
  const parsed = importSchema.safeParse(values);
  const payload: AssetImportPayload | null = parsed.success
    ? {
        source: parsed.data.source,
        entries: parseEntries(parsed.data),
        notes: parsed.data.notes,
        external_ref: parsed.data.externalRef,
      }
    : null;
  const preview = useProjectQuery(
    'assets',
    actorId,
    projectId,
    ['preview', payload],
    (signal) => {
      if (!payload) throw new Error('请修正导入参数。');
      return previewAssets(projectId, payload, signal);
    },
    false,
  );
  const disabled = command.isLocked || !canImport;
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>批量导入资产</h2>
        </CardTitle>
        <CardDescription>
          先查看规范值与错误行，再整批保存。导入仅登记来源，不获得执行许可。
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!canImport && <p className="permission-note">当前是只读成员；导入需要项目分析员能力。</p>}
        <form
          onSubmit={form.handleSubmit(async () => {
            await preview.refetch();
          })}
          noValidate
          className="scope-form"
        >
          <div className="form-grid">
            <div>
              <label htmlFor="asset-source">来源类型</label>
              <select id="asset-source" disabled={disabled} {...form.register('source')}>
                <option value="manual">手动录入</option>
                <option value="api_import">API 导入声明</option>
              </select>
            </div>
            {values.source !== 'api_import' && (
              <div>
                <label htmlFor="asset-kind">资产类型</label>
                <select id="asset-kind" disabled={disabled} {...form.register('kind')}>
                  {Object.entries(assetLabels).map(([value, label]) => (
                    <option value={value} key={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>
          <div>
            <label htmlFor="asset-lines">
              {values.source === 'api_import' ? 'API 资产 JSON' : '资产列表（每行一个）'}
            </label>
            <textarea
              id="asset-lines"
              className="scope-textarea mono-input"
              rows={5}
              autoComplete="off"
              spellCheck={false}
              disabled={disabled}
              aria-invalid={Boolean(form.formState.errors.text)}
              aria-describedby="asset-text-hint asset-text-error"
              {...form.register('text')}
            />
            <p id="asset-text-hint" className="field-hint">
              {values.source === 'api_import'
                ? '例如 [{"kind":"domain","value":"example.invalid"}]。此来源为导入声明，尚未完成观测或验证。'
                : '支持域名、IP 和 HTTP(S) URL。最多 100 行；空行和无效行需要修正。'}
            </p>
            {form.formState.errors.text && (
              <p className="field-error" id="asset-text-error" role="alert">
                {form.formState.errors.text.message}
              </p>
            )}
          </div>
          <div className="form-grid">
            <div>
              <label htmlFor="asset-notes">来源备注</label>
              <Input
                id="asset-notes"
                disabled={disabled}
                autoComplete="off"
                {...form.register('notes')}
              />
              {form.formState.errors.notes && (
                <p role="alert" className="field-error">
                  {form.formState.errors.notes.message}
                </p>
              )}
            </div>
            <div>
              <label htmlFor="asset-external-ref">外部引用</label>
              <Input
                id="asset-external-ref"
                disabled={disabled}
                autoComplete="off"
                {...form.register('externalRef')}
              />
              {form.formState.errors.externalRef && (
                <p role="alert" className="field-error">
                  {form.formState.errors.externalRef.message}
                </p>
              )}
            </div>
          </div>
          <div className="action-row">
            <Button type="submit" variant="outline" disabled={disabled || preview.isFetching}>
              预览规范化
            </Button>
            <Button
              type="button"
              disabled={
                disabled ||
                preview.isFetching ||
                preview.isError ||
                !preview.data?.valid ||
                !payload
              }
              onClick={() => {
                if (payload && preview.data?.valid && !preview.isError && !preview.isFetching)
                  void command.submit({ kind: 'import_assets', payload });
              }}
            >
              确认导入
            </Button>
          </div>
        </form>
        {preview.isFetching && (
          <p role="status" className="query-status">
            正在预览，不创建资产…
          </p>
        )}
        {preview.isError && (
          <p className="field-error" role="alert">
            预览暂时不可用，请修正输入或重新预览。
          </p>
        )}
        {preview.data && (
          <section aria-label="规范化预览" className="preview-block">
            <p role="status">
              输入 {preview.data.input_count} 行，唯一 {preview.data.unique_count} 条，本批重复{' '}
              {preview.data.duplicate_count} 条，项目已有 {preview.data.existing_count} 条。
            </p>
            {!preview.data.valid && (
              <p className="field-error">存在无效行，整批未受理；请修正后再次预览。</p>
            )}
            <ol className="preview-rows">
              {preview.data.rows.map((row) => (
                <li key={row.row}>
                  <span>第 {row.row} 行</span>
                  <code>{row.canonical_value || '—'}</code>
                  {row.error && <span className="field-error">{row.error}</span>}
                </li>
              ))}
            </ol>
          </section>
        )}
      </CardContent>
    </Card>
  );
}
