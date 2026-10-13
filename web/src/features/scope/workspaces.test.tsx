import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AssetPreview, ProjectPolicy } from '../../api/client';
import { createQueryClient } from '../../app/query-client';
import { analystProject, enterpriseSession } from '../../test/fixtures';
import {
  authorizationContent,
  authorizationVersion,
  policyDecision,
  scopeIds,
} from '../../test/scope-fixtures';
import { AssetImportForm } from '../assets/asset-import-form';
import { AssetsWorkspace } from '../assets/assets-workspace';
import { useAssetSearchState } from '../assets/use-asset-search-state';
import { AuthorizationForm } from '../authorization/authorization-form';
import { AuthorizationVersionDetail } from '../authorization/authorization-version-detail';
import { PrecheckForm } from '../prechecks/precheck-form';
import { PrecheckDetail } from '../prechecks/precheck-detail';
import type { ScopeCommandWorkspace } from './use-scope-command';

const api = vi.hoisted(() => ({ previewAssets: vi.fn(), listAssets: vi.fn() }));
vi.mock('../../api/client', async (original) => ({
  ...(await original<typeof import('../../api/client')>()),
  ...api,
}));
function command(): ScopeCommandWorkspace {
  return {
    selection: null,
    phase: 'idle',
    notice: null,
    validationErrors: [],
    receipt: undefined,
    isQuerying: false,
    isLocked: false,
    canLookup: true,
    canReset: false,
    submit: vi.fn().mockResolvedValue(undefined),
    lookup: vi.fn().mockResolvedValue(undefined),
    reset: vi.fn(),
  };
}
function mount(children: ReactNode) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>,
  );
}
const validPreview: AssetPreview = {
  valid: true,
  rows: [{ row: 1, kind: 'domain', canonical_value: 'example.invalid', error: '' }],
  input_count: '1',
  unique_count: '1',
  duplicate_count: '0',
  existing_count: '0',
};
const policy: ProjectPolicy = {
  project_id: analystProject.id,
  active_version_id: scopeIds.version,
  project_revision: '3',
  global_revision: '1',
  denies: [],
};
beforeEach(() => {
  api.previewAssets.mockReset();
  api.listAssets.mockReset();
});

describe('资产与范围中文交互', () => {
  it('预览对应原输入；改输入、无效行和再次预览失败均不能导入旧结果', async () => {
    api.previewAssets.mockResolvedValueOnce(validPreview);
    const workspace = command();
    mount(
      <AssetImportForm
        actorId={enterpriseSession.actor_id}
        projectId={analystProject.id}
        canImport
        command={workspace}
      />,
    );
    const user = userEvent.setup();
    const input = screen.getByRole('textbox', { name: '资产列表（每行一个）' });
    const importButton = screen.getByRole('button', { name: '确认导入' });
    await user.type(input, 'example.invalid');
    await user.click(screen.getByRole('button', { name: '预览规范化' }));
    await waitFor(() => expect(importButton).toBeEnabled());
    await user.type(input, '\nother.invalid');
    expect(importButton).toBeDisabled();
    expect(screen.queryByRole('region', { name: '规范化预览' })).not.toBeInTheDocument();
    await user.clear(input);
    await user.type(input, 'example.invalid');
    await waitFor(() => expect(importButton).toBeEnabled());
    api.previewAssets.mockRejectedValueOnce(new TypeError('合成预览失败'));
    await user.click(screen.getByRole('button', { name: '预览规范化' }));
    await screen.findByText('预览暂时不可用，请修正输入或重新预览。');
    expect(importButton).toBeDisabled();
    api.previewAssets.mockResolvedValueOnce({
      ...validPreview,
      valid: false,
      rows: [{ row: 1, kind: 'domain', canonical_value: '', error: '合成错误行' }],
    });
    await user.click(screen.getByRole('button', { name: '预览规范化' }));
    await screen.findByText('合成错误行');
    expect(importButton).toBeDisabled();
    expect(workspace.submit).not.toHaveBeenCalled();
  });

  it('候选复制保留完整语义及大整数预算，编辑和提交不改变原快照', async () => {
    const original = structuredClone(authorizationContent);
    const workspace = command();
    mount(<AuthorizationForm initial={original} canCreate command={workspace} />);
    const user = userEvent.setup();
    const title = screen.getByRole('textbox', { name: '授权标题' });
    await user.clear(title);
    await user.type(title, '复制后的新候选');
    const budget = screen.getByRole('textbox', { name: '请求总量上限' });
    await user.clear(budget);
    await user.type(budget, '9007199254740993');
    await user.click(screen.getByRole('button', { name: '保存新候选版本' }));
    await waitFor(() => expect(workspace.submit).toHaveBeenCalledTimes(1));
    expect(workspace.submit).toHaveBeenCalledWith({
      kind: 'create_authorization_version',
      payload: {
        ...authorizationContent,
        title: '复制后的新候选',
        budget: { ...authorizationContent.budget, max_requests: '9007199254740993' },
        valid_from: new Date(authorizationContent.valid_from).toISOString(),
        expires_at: new Date(authorizationContent.expires_at).toISOString(),
      },
    });
    expect(original).toEqual(authorizationContent);
    expect(title).toHaveValue('复制后的新候选');
  });

  it('非法预算保留输入且中文说明与字段关联，不触发 BigInt 异常或写请求', async () => {
    const workspace = command();
    mount(<AuthorizationForm initial={authorizationContent} canCreate command={workspace} />);
    const user = userEvent.setup();
    const budget = screen.getByRole('textbox', { name: '请求总量上限' });
    await user.clear(budget);
    await user.type(budget, 'NaN');
    await user.click(screen.getByRole('button', { name: '保存新候选版本' }));
    await screen.findByText('请输入正整数字符串。');
    expect(budget).toHaveValue('NaN');
    expect(budget).toHaveAccessibleDescription('请输入正整数字符串。');
    expect(workspace.submit).not.toHaveBeenCalled();
  });

  it('服务器 CIDR 主机位错误定位到原字段，保留原输入与规范网络建议', async () => {
    const firstEntry = authorizationContent.entries[0];
    if (!firstEntry) throw new Error('合成授权缺少首条范围。');
    const content = {
      ...authorizationContent,
      entries: [
        {
          ...firstEntry,
          kind: 'cidr' as const,
          value: '10.42.7.8/16',
          include_subdomains: false,
          operations: ['tcp_connect' as const],
          ports: [443],
        },
      ],
    };
    const workspace: ScopeCommandWorkspace = {
      ...command(),
      phase: 'rejected',
      isLocked: true,
      selection: {
        commandId: scopeIds.command,
        source: 'submission',
        body: { kind: 'create_authorization_version', payload: content },
      },
      validationErrors: [
        { field: 'payload.entries.0.value', message: 'CIDR 含主机位，请改用 10.42.0.0/16。' },
      ],
    };
    mount(<AuthorizationForm canCreate command={workspace} />);
    const target = screen.getByRole('textbox', { name: '范围目标 1' });
    await waitFor(() =>
      expect(target).toHaveAccessibleDescription('CIDR 含主机位，请改用 10.42.0.0/16。'),
    );
    expect(target).toHaveValue('10.42.7.8/16');
    expect(target).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByRole('alert', { name: '候选字段校验说明' })).toHaveTextContent(
      '原输入已保留',
    );
  });

  it('只读成员无法写入，未知命令锁在三个页面都生效', () => {
    const workspace = command();
    const locked = { ...workspace, isLocked: true };
    const { rerender } = mount(
      <>
        <AssetImportForm
          actorId={enterpriseSession.actor_id}
          projectId={analystProject.id}
          canImport={false}
          command={workspace}
        />
        <AuthorizationForm canCreate={false} command={workspace} />
        <PrecheckForm currentVersionId={scopeIds.version} canCheck={false} command={workspace} />
      </>,
    );
    expect(screen.getByRole('button', { name: '确认导入' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '保存新候选版本' })).toBeDisabled();
    expect(screen.getByRole('textbox', { name: '候选目标' })).toBeDisabled();
    rerender(
      <QueryClientProvider client={createQueryClient()}>
        <PrecheckForm currentVersionId={scopeIds.version} canCheck command={locked} />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('textbox', { name: '候选目标' })).toBeDisabled();
  });

  it('历史详情完整显示只读范围和待检项，当前策略变化不会改写原时刻或提供执行按钮', () => {
    mount(
      <>
        <AuthorizationVersionDetail version={authorizationVersion} canCreate={false} />
        <PrecheckDetail decision={policyDecision} currentPolicy={policy} />
      </>,
    );
    expect(screen.getByText('当前规则已变化，下方为原检查快照')).toBeVisible();
    expect(screen.getByText('9007199254740993', { exact: false })).toBeVisible();
    expect(screen.getByText('尚待检查')).toBeVisible();
    expect(screen.getByText('该操作不适用')).toBeVisible();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /扫描|执行|激活|撤销/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: '复制为新候选' })).not.toBeInTheDocument();
    expect(policyDecision.project_revision).toBe('2');
  });

  it('搜索不进入 URL，非首页详情返回沿用原搜索和 cursor，换筛选清除 cursor', async () => {
    const assetPage = {
      items: [
        {
          id: scopeIds.entry,
          project_id: analystProject.id,
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
    };
    api.listAssets.mockImplementation((_project: string, filters: { cursor?: string }) =>
      Promise.resolve({
        ...assetPage,
        has_more: !filters.cursor,
        next_cursor: filters.cursor ? null : 'opaque-cursor',
      }),
    );
    const workspace = command();
    function Pages() {
      const searchState = useAssetSearchState();
      const location = useLocation();
      const navigate = useNavigate();
      return (
        <>
          <button onClick={() => void navigate(-1)}>浏览器返回</button>
          <output aria-label="当前地址">
            {location.pathname}
            {location.search}
          </output>
          <Routes>
            <Route
              path="/assets/:assetId?"
              element={
                <AssetsWorkspace
                  actorId={enterpriseSession.actor_id}
                  project={analystProject}
                  command={workspace}
                  searchState={searchState}
                />
              }
            />
          </Routes>
        </>
      );
    }
    render(
      <QueryClientProvider client={createQueryClient()}>
        <MemoryRouter initialEntries={['/assets?kind=domain']}>
          <Pages />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await screen.findByRole('link', { name: '查看来源详情' });
    await user.type(screen.getByRole('textbox', { name: '规范值搜索' }), 'example.invalid');
    await user.click(screen.getByRole('button', { name: '应用筛选' }));
    await waitFor(() => expect(screen.getByRole('button', { name: '下一页' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: '下一页' }));
    await waitFor(() => expect(screen.getByRole('button', { name: '回到首页' })).toBeEnabled());
    await user.click(await screen.findByRole('link', { name: '查看来源详情' }));
    await screen.findByRole('heading', { name: '资产来源详情' });
    await user.click(screen.getByRole('link', { name: '返回资产列表' }));
    expect(screen.getByRole('textbox', { name: '规范值搜索' })).toHaveValue('example.invalid');
    expect(screen.getByLabelText('当前地址')).toHaveTextContent(
      '/assets?kind=domain&cursor=opaque-cursor',
    );
    expect(screen.getByLabelText('当前地址')).not.toHaveTextContent('example.invalid');
    expect(api.listAssets).toHaveBeenLastCalledWith(
      analystProject.id,
      { page_size: 25, kind: 'domain', search: 'example.invalid', cursor: 'opaque-cursor' },
      expect.any(AbortSignal),
    );
    await user.type(screen.getByRole('textbox', { name: '规范值搜索' }), '-changed');
    await user.click(screen.getByRole('button', { name: '应用筛选' }));
    await waitFor(() =>
      expect(screen.getByLabelText('当前地址')).toHaveTextContent('/assets?kind=domain'),
    );
    expect(api.listAssets).toHaveBeenLastCalledWith(
      analystProject.id,
      { page_size: 25, kind: 'domain', search: 'example.invalid-changed' },
      expect.any(AbortSignal),
    );
    await user.click(screen.getByRole('button', { name: '浏览器返回' }));
    await waitFor(() =>
      expect(screen.getByRole('textbox', { name: '规范值搜索' })).toHaveValue('example.invalid'),
    );
    expect(screen.getByLabelText('当前地址')).toHaveTextContent(
      '/assets?kind=domain&cursor=opaque-cursor',
    );
    expect(api.listAssets).toHaveBeenLastCalledWith(
      analystProject.id,
      { page_size: 25, kind: 'domain', search: 'example.invalid', cursor: 'opaque-cursor' },
      expect.any(AbortSignal),
    );
  });
});
