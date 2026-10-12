import createClient from 'openapi-fetch';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import type { ValidateFunction } from 'ajv';
import type { components, paths } from './generated/schema';
import schemas from './generated/schemas.json';

/** 唯一 HTTP 契约来源是 api/openapi.yaml，下面只提供查询与校验边界。 */
export type DevelopmentSession = components['schemas']['DevelopmentSession'];
/** 已持久化的受理回执，不代表任何执行已完成。 */
export type CommandReceipt = components['schemas']['CommandReceipt'];
/** 与稳定命令 ID 绑定的创建参数。 */
export type CreateTaskDraftCommand = components['schemas']['CreateTaskDraftCommand'];
type APIError = components['schemas']['APIError'];

const validator = new Ajv2020({ allErrors: true, strict: true });
addFormats(validator);
validator.addSchema(schemas, 'mousehole');

function schemaValidator<T>(name: keyof components['schemas']): ValidateFunction<T> {
  return validator.compile<T>({ $ref: `mousehole#/$defs/${name}` });
}

const validateSession = schemaValidator<DevelopmentSession>('DevelopmentSession');
const validateReceipt = schemaValidator<CommandReceipt>('CommandReceipt');
const validateError = schemaValidator<APIError>('APIError');
const client = createClient<paths>({ baseUrl: window.location.origin });
const developmentHeader = { 'X-Mousehole-Development': 'true' } as const;
const commandPath = '/api/v1/projects/{project_id}/commands/{command_id}';

/** API 错误只携带已校验的公开信息，未知网络失败不会被解释为未受理。 */
export class CommandAPIError extends Error {
  readonly status: number;
  readonly code: APIError['code'] | 'unconfirmed_response';
  readonly requestId: string | null;

  constructor(status: number, body: unknown) {
    const valid = validateError(body);
    super(valid ? body.message : '服务响应暂无法确认，请保留命令 ID 查询。');
    this.name = 'CommandAPIError';
    this.status = status;
    this.code = valid ? body.code : 'unconfirmed_response';
    this.requestId = valid ? body.request_id : null;
  }
}

function parseReceipt(value: unknown, projectId: string, commandId: string): CommandReceipt {
  // RFC3339 闰秒可通过 schema，但浏览器无法表示；展示前还须验证日期可解析。
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

/** 查询显式开发身份；超时只影响页面连接状态。 */
export async function getDevelopmentSession(signal: AbortSignal): Promise<DevelopmentSession> {
  const { data, error, response } = await client.GET('/api/v1/session', {
    params: { header: developmentHeader },
    signal: AbortSignal.any([signal, AbortSignal.timeout(15_000)]),
  });
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (response.status !== 200) throw new CommandAPIError(0, null);
  if (!validateSession(data)) throw new CommandAPIError(0, null);
  return data;
}

/**
 * 仅发送一次 PUT。调用者必须在进入本函数前保存 ID 和规范化参数。
 * Promise 拒绝不证明事务未提交，5xx、超时及无效响应均需查询原 ID。
 */
export async function acceptCommand(
  projectId: string,
  commandId: string,
  body: CreateTaskDraftCommand,
): Promise<CommandReceipt> {
  const { data, error, response } = await client.PUT(commandPath, {
    params: { path: { project_id: projectId, command_id: commandId }, header: developmentHeader },
    body,
    signal: AbortSignal.timeout(15_000),
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
    params: { path: { project_id: projectId, command_id: commandId }, header: developmentHeader },
    signal: AbortSignal.any([signal, AbortSignal.timeout(15_000)]),
  });
  if (!response.ok) throw new CommandAPIError(response.status, error);
  if (response.status !== 200) throw new CommandAPIError(0, null);
  const receipt = parseReceipt(data, projectId, commandId);
  if (expectedTitle !== null && receipt.task_title !== expectedTitle) {
    throw new CommandAPIError(0, null);
  }
  return receipt;
}

/** 仅明确的请求拒绝可解锁新操作；依赖失败始终保留未知结果。 */
export function isDefinitiveRejection(error: unknown): error is CommandAPIError {
  if (!(error instanceof CommandAPIError)) return false;
  switch (error.code) {
    case 'invalid_request':
      return error.status === 400;
    case 'project_forbidden':
    case 'development_only':
      return error.status === 403;
    case 'idempotency_conflict':
      return error.status === 409;
    case 'request_too_large':
      return error.status === 413;
    case 'unsupported_media_type':
      return error.status === 415;
    default:
      return false;
  }
}
