import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ScopeCommandRequest } from './client';
import { analystProject, enterpriseSession } from '../test/fixtures';
import {
  authorizationVersion,
  policyDecision,
  precheckReceipt,
  scopeIds,
} from '../test/scope-fixtures';

let fetchMock = vi.fn<typeof fetch>();
const projectId = analystProject.id;
const request: ScopeCommandRequest = {
  kind: 'precheck_target',
  payload: {
    authorization_version_id: scopeIds.version,
    operation: 'dns_lookup',
    target: 'example.invalid',
    parameters: {},
  },
};
function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}
beforeEach(() => {
  vi.resetModules();
  fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('范围 API 契约与对象绑定', () => {
  it('写请求保持命令/参数和 CSRF，回执预算/版本字符串不会丢失精度', async () => {
    fetchMock.mockResolvedValue(response(precheckReceipt(), 201));
    const api = await import('./client');
    api.setSessionMemory(enterpriseSession);
    await expect(
      api.acceptScopeCommand(projectId, scopeIds.command, request, new AbortController().signal),
    ).resolves.toEqual(precheckReceipt());
    const sent = fetchMock.mock.calls[0]?.[0];
    if (!(sent instanceof Request)) throw new Error('预期真实 Request 对象。');
    expect(sent.method).toBe('PUT');
    expect(sent.credentials).toBe('include');
    expect(sent.headers.get('X-CSRF-Token') === enterpriseSession.csrf_token).toBe(true);
    expect(await sent.json()).toEqual(request);
    fetchMock.mockResolvedValue(response(authorizationVersion));
    const version = await api.getAuthorizationVersion(
      projectId,
      scopeIds.version,
      new AbortController().signal,
    );
    expect(version.sequence).toBe('9007199254740993');
  });

  it.each([
    { ...precheckReceipt(), kind: 'import_assets' },
    { ...precheckReceipt(), project_id: scopeIds.version },
    { ...precheckReceipt(), command_id: scopeIds.version },
    {
      ...precheckReceipt(),
      result: { type: 'precheck', decision: { ...policyDecision, project_id: scopeIds.version } },
    },
    {
      ...precheckReceipt(),
      result: { type: 'precheck', decision: { ...policyDecision, command_id: scopeIds.version } },
    },
    {
      ...precheckReceipt(),
      result: {
        type: 'precheck',
        decision: { ...policyDecision, operation: 'tcp_connect', parameters: { port: 443 } },
      },
    },
    {
      ...precheckReceipt(),
      result: {
        type: 'precheck',
        decision: { ...policyDecision, checked_at: '2026-10-12T23:59:60Z' },
      },
    },
  ])('类型合法但项目/命令/类型/原参数/浏览器时间错配仍不可确认', async (receipt) => {
    fetchMock.mockResolvedValue(response(receipt));
    const api = await import('./client');
    await expect(
      api.getScopeCommandReceipt(
        projectId,
        scopeIds.command,
        new AbortController().signal,
        request,
      ),
    ).rejects.toMatchObject({ code: 'unconfirmed_response' });
  });

  it('嵌套资产来源跨项目、版本条目数量和分页游标矛盾均拒绝', async () => {
    const api = await import('./client');
    fetchMock.mockResolvedValueOnce(response({ ...authorizationVersion, entry_ids: [] }));
    await expect(
      api.getAuthorizationVersion(projectId, scopeIds.version, new AbortController().signal),
    ).rejects.toMatchObject({ code: 'unconfirmed_response' });
    fetchMock.mockResolvedValueOnce(
      response({ items: [], has_more: true, next_cursor: null, page_count: '0' }),
    );
    await expect(api.listAssets(projectId, {}, new AbortController().signal)).rejects.toMatchObject(
      { code: 'unconfirmed_response' },
    );
    fetchMock.mockResolvedValueOnce(
      response({
        items: [
          {
            id: scopeIds.version,
            project_id: scopeIds.version,
            kind: 'domain',
            canonical_value: 'example.invalid',
            algorithm_version: 'target-v1',
            created_at: policyDecision.checked_at,
            sources: [],
          },
        ],
        has_more: false,
        next_cursor: null,
        page_count: '1',
      }),
    );
    await expect(api.listAssets(projectId, {}, new AbortController().signal)).rejects.toMatchObject(
      { code: 'unconfirmed_response' },
    );
  });

  it('错误 status/code 配对正确才解锁；503、未知 409 和无效响应继续原 ID 查询', async () => {
    const api = await import('./client');
    const error = (status: number, code: string) =>
      new api.CommandAPIError(status, { code, message: '合成错误', request_id: scopeIds.command });
    expect(api.isDefinitiveRejection(error(409, 'command_conflict'))).toBe(true);
    expect(api.isDefinitiveRejection(error(409, 'policy_revision_conflict'))).toBe(true);
    expect(api.isDefinitiveRejection(error(503, 'command_conflict'))).toBe(false);
    expect(api.isDefinitiveRejection(error(409, 'unavailable'))).toBe(false);
    expect(api.isDefinitiveRejection(new api.CommandAPIError(409, {}))).toBe(false);
  });

  it('结构化字段错误只在400 invalid_request接受，额外字段和错配仍待确认', async () => {
    const api = await import('./client');
    const body = {
      code: 'invalid_request',
      message: '范围参数无效',
      request_id: scopeIds.command,
      validation_errors: [
        { field: 'payload.entries.0.value', message: 'CIDR 含主机位，请改用 10.42.0.0/16。' },
      ],
    };
    const parsed = new api.CommandAPIError(400, body);
    expect(parsed.validationErrors).toEqual(body.validation_errors);
    expect(api.isDefinitiveRejection(parsed)).toBe(true);
    expect(new api.CommandAPIError(503, body).code).toBe('unconfirmed_response');
    expect(
      new api.CommandAPIError(400, {
        ...body,
        validation_errors: [{ ...body.validation_errors[0], target: '不允许额外字段' }],
      }).code,
    ).toBe('unconfirmed_response');
    expect(new api.CommandAPIError(400, { ...body, code: 'forbidden' }).validationErrors).toEqual(
      [],
    );
  });
});
