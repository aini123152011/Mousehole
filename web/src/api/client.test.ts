import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { analystProject, enterpriseSession, receiptFor } from '../test/fixtures';

const commandId = '00000000-0000-4000-8000-000000000003';
const projectId = analystProject.id;
const body = { kind: 'create_task_draft' as const, payload: { title: '内部平台评估准备' } };
let fetchMock = vi.fn<typeof fetch>();

function response(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

beforeEach(() => {
  // openapi-fetch 在创建时绑定 fetch，因此先安装模拟边界，再重新载入模块。
  vi.resetModules();
  fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('OpenAPI 运行时边界', () => {
  it.each([200, 201])('接受 HTTP %i 回执并发送 Cookie 凭据、CSRF 和稳定 ID', async (status) => {
    fetchMock.mockResolvedValue(response(receiptFor(commandId), status));
    const api = await import('./client');
    api.setSessionMemory(enterpriseSession);
    await expect(api.acceptCommand(projectId, commandId, body)).resolves.toEqual(
      receiptFor(commandId),
    );
    const request = fetchMock.mock.calls[0]?.[0];
    expect(request).toBeInstanceOf(Request);
    if (!(request instanceof Request)) throw new Error('预期客户端生成 Request。');
    expect(request.url).toContain(`/projects/${projectId}/commands/${commandId}`);
    expect(request.method).toBe('PUT');
    expect(request.credentials).toBe('include');
    expect(request.headers.get('X-CSRF-Token')).toBe(enterpriseSession.csrf_token);
    expect(await request.json()).toEqual(body);
  });

  it.each([
    { value: { ...receiptFor(commandId), task_state: 'running' }, status: 201 },
    { value: { ...receiptFor(commandId), extra: true }, status: 201 },
    {
      value: { ...receiptFor(commandId), command_id: '00000000-0000-4000-8000-000000000004' },
      status: 201,
    },
    {
      value: { ...receiptFor(commandId), project_id: '00000000-0000-4000-8000-000000000004' },
      status: 201,
    },
    { value: { ...receiptFor(commandId), task_title: '另一个标题' }, status: 201 },
    { value: { ...receiptFor(commandId), accepted_at: '2026-10-12T14:00:00+08:00' }, status: 201 },
    { value: { ...receiptFor(commandId), accepted_at: '2026-10-12T06:00:00+00:00' }, status: 201 },
    { value: { ...receiptFor(commandId), accepted_at: '不是时间' }, status: 201 },
    { value: { ...receiptFor(commandId), accepted_at: '2026-10-12T23:59:60Z' }, status: 201 },
    { value: receiptFor(commandId), status: 202 },
  ])(
    '拒绝无效字段、身份、参数和非合同成功状态：$status / $value.task_state',
    async ({ value, status }) => {
      fetchMock.mockResolvedValue(response(value, status));
      const api = await import('./client');
      api.setSessionMemory(enterpriseSession);
      await expect(api.acceptCommand(projectId, commandId, body)).rejects.toMatchObject({
        code: 'unconfirmed_response',
      });
      expect(fetchMock).toHaveBeenCalledTimes(1);
    },
  );

  it('恢复本页提交时拒绝标题失配的查询回执', async () => {
    fetchMock.mockResolvedValue(response(receiptFor(commandId, '其他标题')));
    const api = await import('./client');
    await expect(
      api.getCommandReceipt(projectId, commandId, new AbortController().signal, body.payload.title),
    ).rejects.toMatchObject({ code: 'unconfirmed_response' });
  });

  it('网络失败只发送一次，不在传输层重试 PUT', async () => {
    fetchMock.mockRejectedValue(new TypeError('合成网络中断'));
    const api = await import('./client');
    api.setSessionMemory(enterpriseSession);
    await expect(api.acceptCommand(projectId, commandId, body)).rejects.toThrow('合成网络中断');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('错误码和状态码同时匹配才算明确拒绝', async () => {
    const api = await import('./client');
    const makeError = (status: number, code: string) =>
      new api.CommandAPIError(status, { code, message: '合成拒绝', request_id: commandId });
    expect(api.isDefinitiveRejection(makeError(409, 'idempotency_conflict'))).toBe(true);
    expect(api.isDefinitiveRejection(makeError(400, 'invalid_request'))).toBe(true);
    expect(api.isDefinitiveRejection(makeError(400, 'internal_error'))).toBe(false);
    expect(api.isDefinitiveRejection(makeError(500, 'invalid_request'))).toBe(false);
    expect(api.isDefinitiveRejection(new api.CommandAPIError(400, {}))).toBe(false);
  });

  it('session 字段来源于合同并拒绝未知字段', async () => {
    fetchMock
      .mockResolvedValueOnce(response(enterpriseSession))
      .mockResolvedValueOnce(response({ ...enterpriseSession, token: '合成无效字段' }));
    const api = await import('./client');
    await expect(api.getSession(new AbortController().signal)).resolves.toEqual(enterpriseSession);
    await expect(api.getSession(new AbortController().signal)).rejects.toMatchObject({
      code: 'unconfirmed_response',
    });
  });
});
