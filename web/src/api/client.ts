import createClient from 'openapi-fetch';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import type { ValidateFunction } from 'ajv';
import type { components, paths } from './generated/schema';
import schemas from './generated/schemas.json';

/** OpenAPI 是唯一传输合同；本模块只负责凭据边界和运行时响应校验。 */
export type Session = components['schemas']['Session'];
/** 已持久化的受理回执，不代表任何目标行为已执行。 */
export type CommandReceipt = components['schemas']['CommandReceipt'];
/** 与稳定命令 ID 绑定的创建参数。 */
export type CreateTaskDraftCommand = components['schemas']['CreateTaskDraftCommand'];
type APIError = components['schemas']['APIError'];

const validator = new Ajv2020({ allErrors: true, strict: true, discriminator: true });
addFormats(validator);
validator.addSchema(schemas, 'mousehole');

function schemaValidator<T>(name: keyof components['schemas']): (value: unknown) => value is T {
  // 按实际响应首次使用编译，草稿/登录路径不为未访问领域提前编译全部 schema。
  let compiled: ValidateFunction<T> | null = null;
  return (value: unknown): value is T => {
    compiled ??= validator.compile<T>({ $ref: `mousehole#/$defs/${name}` });
    return compiled(value);
  };
}

const validateSession = schemaValidator<Session>('Session');
const validateReceipt = schemaValidator<CommandReceipt>('CommandReceipt');
const validateError = schemaValidator<APIError>('APIError');
const client = createClient<paths>({ baseUrl: window.location.origin });
const commandPath = '/api/v1/projects/{project_id}/commands/{command_id}';
let currentSession: Session | null = null;

/** 受保护写请求读取内存中的 CSRF token；Cookie 由浏览器自动管理。 */
export function setSessionMemory(session: Session | null) {
  currentSession = session;
}

/** API 错误只携带已校验的公开信息，未知网络失败不会被解释为未受理。 */
export class CommandAPIError extends Error {
  readonly status: number;
  readonly code: APIError['code'] | 'unconfirmed_response';
  readonly requestId: string | null;
  readonly validationErrors: NonNullable<APIError['validation_errors']>;

  constructor(status: number, body: unknown) {
    const valid =
      validateError(body) &&
      (body.validation_errors === undefined || (status === 400 && body.code === 'invalid_request'));
    super(valid ? body.message : '服务响应暂无法确认，请保留命令 ID 查询。');
    this.name = 'CommandAPIError';
    this.status = status;
    this.code = valid ? body.code : 'unconfirmed_response';
    this.requestId = valid ? body.request_id : null;
    this.validationErrors = valid ? (body.validation_errors ?? []) : [];
  }
}

function parseReceipt(value: unknown, projectId: string, commandId: string): CommandReceipt {
  if (
    !validateReceipt(value) ||
    value.project_id !== projectId ||
    value.command_id !== commandId ||
    !Number.isFinite(Date.parse(value.accepted_at))
  ) {
    throw new CommandAPIError(0, null);
  }
  return value;
}

/** 查询当前服务端会话；401 由页面显示登录入口，响应只保存在 Query 与内存。 */
export async function getSession(signal: AbortSignal): Promise<Session> {
  const { data, error, response } = await client.GET('/api/v1/session', {
    credentials: 'include',
    signal: AbortSignal.any([signal, AbortSignal.timeout(15_000)]),
  });
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (response.status !== 200 || !validateSession(data)) throw new CommandAPIError(0, null);
  currentSession = data;
  return data;
}

/** 撤销当前 Cookie 会话；请求仍需 session-bound CSRF 和精确来源。 */
export async function logout(): Promise<void> {
  const csrf = currentSession?.csrf_token;
  if (!csrf) throw new CommandAPIError(403, null);
  const { error, response } = await client.POST('/api/v1/auth/logout', {
    credentials: 'include',
    params: { header: { 'X-CSRF-Token': csrf } },
  });
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (response.status !== 204) throw new CommandAPIError(0, null);
  currentSession = null;
}

/**
 * 仅发送一次 PUT。调用者必须在进入本函数前保存 ID 和规范化参数。
 * Promise 拒绝不证明事务未提交，5xx、超时及无效响应均需查询原 ID。
 */
export async function acceptCommand(
  projectId: string,
  commandId: string,
  body: CreateTaskDraftCommand,
  signal?: AbortSignal,
): Promise<CommandReceipt> {
  const csrf = currentSession?.csrf_token;
  if (!csrf) throw new CommandAPIError(401, null);
  const { data, error, response } = await client.PUT(commandPath, {
    credentials: 'include',
    params: {
      path: { project_id: projectId, command_id: commandId },
      header: { 'X-CSRF-Token': csrf },
    },
    body,
    signal: signal
      ? AbortSignal.any([signal, AbortSignal.timeout(15_000)])
      : AbortSignal.timeout(15_000),
  });
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (response.status !== 200 && response.status !== 201) throw new CommandAPIError(0, null);
  const receipt = parseReceipt(data, projectId, commandId);
  if (receipt.task_title !== body.payload.title) throw new CommandAPIError(0, null);
  return receipt;
}

/** 按项目和命令查询回执；恢复本页提交时还须核对绑定标题，404 仍表示待确认。 */
export async function getCommandReceipt(
  projectId: string,
  commandId: string,
  signal: AbortSignal,
  expectedTitle: string | null = null,
): Promise<CommandReceipt> {
  const { data, error, response } = await client.GET(commandPath, {
    credentials: 'include',
    params: { path: { project_id: projectId, command_id: commandId } },
    signal: AbortSignal.any([signal, AbortSignal.timeout(15_000)]),
  });
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (response.status !== 200) throw new CommandAPIError(0, null);
  const receipt = parseReceipt(data, projectId, commandId);
  if (expectedTitle !== null && receipt.task_title !== expectedTitle)
    throw new CommandAPIError(0, null);
  return receipt;
}

/** 只有明确业务拒绝才解锁新操作；认证、依赖失败和未知响应保持待确认。 */
export function isDefinitiveRejection(error: unknown): error is CommandAPIError {
  if (!(error instanceof CommandAPIError)) return false;
  switch (error.code) {
    case 'invalid_request':
      return error.status === 400;
    case 'project_forbidden':
    case 'forbidden':
      return error.status === 403;
    case 'idempotency_conflict':
    case 'command_conflict':
    case 'policy_revision_conflict':
      return error.status === 409;
    case 'request_too_large':
      return error.status === 413;
    case 'unsupported_media_type':
      return error.status === 415;
    default:
      return false;
  }
}

/** 项目资产及来源声明；来源不等于已验证观测。 */
export type Asset = components['schemas']['Asset'];
/** 整批导入参数，资产存在不扩大授权。 */
export type AssetImportPayload = components['schemas']['AssetImportPayload'];
/** 服务端规范化预览；无效行必须整批修正。 */
export type AssetPreview = components['schemas']['AssetPreview'];
/** 保存后不可变的授权语义内容。 */
export type AuthorizationContent = components['schemas']['AuthorizationContent'];
/** 授权版本的正文与独立生效状态。 */
export type AuthorizationVersion = components['schemas']['AuthorizationVersion'];
/** 当前指针、递增策略版本和有效禁止规则。 */
export type ProjectPolicy = components['schemas']['ProjectPolicy'];
/** 历史静态决策；没有执行许可或令牌。 */
export type PolicyDecision = components['schemas']['PolicyDecision'];
/** 同一幂等命名空间中的三种 Web 写命令。 */
export type ScopeCommandRequest = components['schemas']['ScopeCommandRequest'];
/** 冻结的原始命令回执；恢复不会重新运行检查。 */
export type ScopeCommandReceipt = components['schemas']['ScopeCommandReceipt'];
/** 列表筛选属于服务器请求合同，目标搜索仅保留内存。 */
export type AssetFilters = NonNullable<
  paths['/api/v1/projects/{project_id}/assets']['get']['parameters']['query']
>;
/** 稳定 seek 分页参数。 */
export type PageFilters = NonNullable<
  paths['/api/v1/projects/{project_id}/prechecks']['get']['parameters']['query']
>;

const validateAssetPage = schemaValidator<components['schemas']['AssetPage']>('AssetPage');
const validatePreview = schemaValidator<AssetPreview>('AssetPreview');
const validateVersion = schemaValidator<AuthorizationVersion>('AuthorizationVersion');
const validateVersionPage = schemaValidator<components['schemas']['AuthorizationVersionPage']>(
  'AuthorizationVersionPage',
);
const validatePolicy = schemaValidator<ProjectPolicy>('ProjectPolicy');
const validateDecision = schemaValidator<PolicyDecision>('PolicyDecision');
const validatePrecheckPage = schemaValidator<components['schemas']['PrecheckPage']>('PrecheckPage');
const validateScopeReceipt = schemaValidator<ScopeCommandReceipt>('ScopeCommandReceipt');
const scopeCommandPath = '/api/v1/projects/{project_id}/scope-commands/{command_id}';

function budgetSignal(signal?: AbortSignal) {
  return signal
    ? AbortSignal.any([signal, AbortSignal.timeout(15_000)])
    : AbortSignal.timeout(15_000);
}

function csrfHeader() {
  const csrf = currentSession?.csrf_token;
  if (!csrf) throw new CommandAPIError(401, null);
  return { 'X-CSRF-Token': csrf };
}

function checkedResponse<T>(
  data: unknown,
  error: unknown,
  response: Response,
  validate: (value: unknown) => value is T,
  matches: (value: T) => boolean,
  statuses: readonly number[] = [200],
): T {
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (!statuses.includes(response.status) || !validate(data) || !matches(data))
    throw new CommandAPIError(0, null);
  return data;
}

function validTime(value: string) {
  return Number.isFinite(Date.parse(value));
}
function validAsset(value: Asset, projectId: string) {
  return (
    value.project_id === projectId &&
    validTime(value.created_at) &&
    value.sources.every((source) => validTime(source.created_at))
  );
}
function validVersion(value: AuthorizationVersion, projectId: string) {
  return (
    value.project_id === projectId &&
    value.entry_ids.length === value.content.entries.length &&
    validTime(value.created_at) &&
    validTime(value.content.valid_from) &&
    validTime(value.content.expires_at) &&
    Date.parse(value.content.valid_from) < Date.parse(value.content.expires_at)
  );
}
function validDecision(value: PolicyDecision, projectId: string) {
  return value.project_id === projectId && validTime(value.checked_at);
}
function validPage(value: {
  items: readonly unknown[];
  page_count: string;
  has_more: boolean;
  next_cursor: string | null;
}) {
  return (
    value.page_count === String(value.items.length) &&
    (value.has_more ? Boolean(value.next_cursor) : value.next_cursor === null)
  );
}

/** 读取本项目资产；逐行校验项目归属，不能展示其他项目的来源。 */
export async function listAssets(projectId: string, filters: AssetFilters, signal: AbortSignal) {
  const { data, error, response } = await client.GET('/api/v1/projects/{project_id}/assets', {
    credentials: 'include',
    params: { path: { project_id: projectId }, query: filters },
    signal: budgetSignal(signal),
  });
  return checkedResponse(
    data,
    error,
    response,
    validateAssetPage,
    (page) => validPage(page) && page.items.every((asset) => validAsset(asset, projectId)),
  );
}

/** 只预览规范化，不创建资产；调用者取消编辑中的过期预览。 */
export async function previewAssets(
  projectId: string,
  body: AssetImportPayload,
  signal: AbortSignal,
) {
  const { data, error, response } = await client.POST(
    '/api/v1/projects/{project_id}/assets/preview',
    {
      credentials: 'include',
      params: { path: { project_id: projectId }, header: csrfHeader() },
      body,
      signal: budgetSignal(signal),
    },
  );
  return checkedResponse(
    data,
    error,
    response,
    validatePreview,
    (preview) =>
      preview.rows.length === body.entries.length &&
      preview.input_count === String(body.entries.length) &&
      preview.rows.every((row, index) => row.row === index + 1),
  );
}

/** 服务端分页读取不可变版本；生效状态以当前查询时刻为准。 */
export async function listAuthorizationVersions(
  projectId: string,
  filters: PageFilters,
  signal: AbortSignal,
) {
  const { data, error, response } = await client.GET(
    '/api/v1/projects/{project_id}/authorization-versions',
    {
      credentials: 'include',
      params: { path: { project_id: projectId }, query: filters },
      signal: budgetSignal(signal),
    },
  );
  return checkedResponse(
    data,
    error,
    response,
    validateVersionPage,
    (page) => validPage(page) && page.items.every((version) => validVersion(version, projectId)),
  );
}

/** 查询指定版本，并复核请求 ID 和项目。 */
export async function getAuthorizationVersion(
  projectId: string,
  versionId: string,
  signal: AbortSignal,
) {
  const { data, error, response } = await client.GET(
    '/api/v1/projects/{project_id}/authorization-versions/{version_id}',
    {
      credentials: 'include',
      params: { path: { project_id: projectId, version_id: versionId } },
      signal: budgetSignal(signal),
    },
  );
  return checkedResponse(
    data,
    error,
    response,
    validateVersion,
    (version) => version.id === versionId && validVersion(version, projectId),
  );
}

/** 当前策略不是静态决策的授权令牌，页面须区分历史检查与当前状态。 */
export async function getProjectPolicy(projectId: string, signal: AbortSignal) {
  const { data, error, response } = await client.GET('/api/v1/projects/{project_id}/policy', {
    credentials: 'include',
    params: { path: { project_id: projectId } },
    signal: budgetSignal(signal),
  });
  return checkedResponse(
    data,
    error,
    response,
    validatePolicy,
    (policy) =>
      policy.project_id === projectId &&
      policy.denies.every(
        (deny) =>
          (deny.project_id === null || deny.project_id === projectId) && validTime(deny.created_at),
      ),
  );
}

/** 分页读取历史预检查；不会发起任何目标连接。 */
export async function listPrechecks(projectId: string, filters: PageFilters, signal: AbortSignal) {
  const { data, error, response } = await client.GET('/api/v1/projects/{project_id}/prechecks', {
    credentials: 'include',
    params: { path: { project_id: projectId }, query: filters },
    signal: budgetSignal(signal),
  });
  return checkedResponse(
    data,
    error,
    response,
    validatePrecheckPage,
    (page) => validPage(page) && page.items.every((decision) => validDecision(decision, projectId)),
  );
}

/** 查询持久化解释，原 checked_at/revision 保持不变。 */
export async function getPrecheck(projectId: string, decisionId: string, signal: AbortSignal) {
  const { data, error, response } = await client.GET(
    '/api/v1/projects/{project_id}/prechecks/{decision_id}',
    {
      credentials: 'include',
      params: { path: { project_id: projectId, decision_id: decisionId } },
      signal: budgetSignal(signal),
    },
  );
  return checkedResponse(
    data,
    error,
    response,
    validateDecision,
    (decision) => decision.id === decisionId && validDecision(decision, projectId),
  );
}

function matchingScopeReceipt(
  receipt: ScopeCommandReceipt,
  projectId: string,
  commandId: string,
  expected: ScopeCommandRequest | null,
) {
  if (
    receipt.project_id !== projectId ||
    receipt.command_id !== commandId ||
    !validTime(receipt.accepted_at) ||
    (expected && receipt.kind !== expected.kind)
  )
    return false;
  switch (receipt.result.type) {
    case 'assets_imported':
      return (
        receipt.kind === 'import_assets' &&
        receipt.result.assets.every((asset) => validAsset(asset, projectId)) &&
        (!expected ||
          expected.kind !== 'import_assets' ||
          receipt.result.input_count === String(expected.payload.entries.length))
      );
    case 'authorization_version':
      return (
        receipt.kind === 'create_authorization_version' &&
        validVersion(receipt.result.version, projectId)
      );
    case 'precheck': {
      const decision = receipt.result.decision;
      return (
        receipt.kind === 'precheck_target' &&
        validDecision(decision, projectId) &&
        decision.command_id === commandId &&
        (!expected ||
          expected.kind !== 'precheck_target' ||
          (decision.authorization_version_id === expected.payload.authorization_version_id &&
            decision.operation === expected.payload.operation &&
            decision.parameters.port === expected.payload.parameters.port))
      );
    }
    case 'policy_changed':
      return [
        'activate_authorization_version',
        'revoke_authorization_version',
        'add_scope_deny',
        'revoke_scope_deny',
      ].includes(receipt.kind);
  }
}

/** 发送前 ID/参数已锁定；依赖失败、无效响应或断线后只能查询原命令。 */
export async function acceptScopeCommand(
  projectId: string,
  commandId: string,
  body: ScopeCommandRequest,
  signal: AbortSignal,
) {
  const { data, error, response } = await client.PUT(scopeCommandPath, {
    credentials: 'include',
    params: { path: { project_id: projectId, command_id: commandId }, header: csrfHeader() },
    body,
    signal: budgetSignal(signal),
  });
  return checkedResponse(
    data,
    error,
    response,
    validateScopeReceipt,
    (receipt) => matchingScopeReceipt(receipt, projectId, commandId, body),
    [200, 201],
  );
}

/** 人工恢复原 ID；404 仅表示尚未确认，不触发 PUT，也不解锁未知提交。 */
export async function getScopeCommandReceipt(
  projectId: string,
  commandId: string,
  signal: AbortSignal,
  expected: ScopeCommandRequest | null = null,
) {
  const { data, error, response } = await client.GET(scopeCommandPath, {
    credentials: 'include',
    params: { path: { project_id: projectId, command_id: commandId } },
    signal: budgetSignal(signal),
  });
  return checkedResponse(data, error, response, validateScopeReceipt, (receipt) =>
    matchingScopeReceipt(receipt, projectId, commandId, expected),
  );
}
