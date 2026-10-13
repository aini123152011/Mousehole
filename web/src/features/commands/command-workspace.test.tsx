import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CommandAPIError } from '../../api/client';
import { createQueryClient } from '../../app/query-client';
import { analystProject, receiptFor } from '../../test/fixtures';
import { CommandWorkspace } from './command-workspace';
import { useCommandWorkspace } from './use-command-workspace';

const apiMock = vi.hoisted(() => ({ acceptCommand: vi.fn(), getCommandReceipt: vi.fn() }));
vi.mock('../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/client')>()),
  ...apiMock,
}));
const commandId = '00000000-0000-4000-8000-000000000003';

beforeEach(() => {
  apiMock.acceptCommand.mockReset();
  apiMock.getCommandReceipt.mockReset();
  vi.spyOn(crypto, 'randomUUID').mockReturnValue(commandId);
});

function setup() {
  function Workspace() {
    const workspace = useCommandWorkspace(
      '10000000-0000-4000-8000-000000000001',
      analystProject.id,
    );
    return <CommandWorkspace project={analystProject} workspace={workspace} />;
  }
  render(
    <QueryClientProvider client={createQueryClient()}>
      <Workspace />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe('中文命令工作台', () => {
  it('响应丢失后锁定表单，404 不解锁，查询成功展示草稿回执', async () => {
    apiMock.acceptCommand.mockRejectedValue(new TypeError('合成丢包'));
    apiMock.getCommandReceipt
      .mockRejectedValueOnce(
        new CommandAPIError(404, {
          code: 'command_not_found',
          message: '未找到',
          request_id: commandId,
        }),
      )
      .mockResolvedValueOnce(receiptFor(commandId));
    const user = setup();
    await user.type(screen.getByRole('textbox', { name: '任务标题' }), '内部平台评估准备');
    await user.click(screen.getByRole('button', { name: '创建草稿' }));
    expect(await screen.findByRole('heading', { name: '受理待确认' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '任务标题' })).toBeDisabled();
    expect(screen.getByRole('textbox', { name: '已有命令 ID' })).toBeDisabled();
    expect(screen.getByTestId('current-command-id')).toHaveTextContent(commandId);
    await user.click(screen.getByRole('button', { name: '查询原命令' }));
    expect(await screen.findByText(/暂未查询到回执/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '新建另一份草稿' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '查询原命令' }));
    expect(await screen.findByRole('heading', { name: '已受理' })).toBeInTheDocument();
    expect(screen.getByText('已受理表示草稿已保存，任务尚未执行。')).toBeInTheDocument();
    expect(apiMock.acceptCommand).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole('button', { name: '新建另一份草稿' }));
    expect(screen.getByRole('textbox', { name: '任务标题' })).toHaveFocus();
    expect(screen.getByRole('textbox', { name: '任务标题' })).toHaveValue('');
  });

  it('刷新后人工输入旧 ID 恢复，不产生新提交', async () => {
    apiMock.getCommandReceipt.mockResolvedValue(receiptFor(commandId));
    const user = setup();
    await user.type(screen.getByRole('textbox', { name: '已有命令 ID' }), commandId);
    await user.click(screen.getByRole('button', { name: '查询回执' }));
    expect(await screen.findByRole('heading', { name: '已受理' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '内部平台评估准备' })).toBeInTheDocument();
    expect(apiMock.acceptCommand).not.toHaveBeenCalled();
  });

  it('空标题显示关联中文错误，不调用 API', async () => {
    const user = setup();
    await user.click(screen.getByRole('button', { name: '创建草稿' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('请输入任务标题。');
    expect(screen.getByRole('textbox', { name: '任务标题' })).toHaveAttribute(
      'aria-describedby',
      'title-error',
    );
    expect(apiMock.acceptCommand).not.toHaveBeenCalled();
  });

  it('未知草稿跨功能导航后仍锁定原 ID 和标题，仅查询恢复不再 PUT', async () => {
    apiMock.acceptCommand.mockRejectedValue(new TypeError('合成丢包'));
    apiMock.getCommandReceipt.mockResolvedValue(receiptFor(commandId));
    function Pages() {
      const workspace = useCommandWorkspace(
        '10000000-0000-4000-8000-000000000001',
        analystProject.id,
      );
      return (
        <>
          <Link to="/">任务工作台</Link>
          <Link to="/assets">资产</Link>
          <Routes>
            <Route
              path="/"
              element={<CommandWorkspace project={analystProject} workspace={workspace} />}
            />
            <Route path="/assets" element={<p>合成资产页</p>} />
          </Routes>
        </>
      );
    }
    render(
      <QueryClientProvider client={createQueryClient()}>
        <MemoryRouter>
          <Pages />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.type(screen.getByRole('textbox', { name: '任务标题' }), '内部平台评估准备');
    await user.click(screen.getByRole('button', { name: '创建草稿' }));
    await screen.findByRole('heading', { name: '受理待确认' });
    await user.click(screen.getByRole('link', { name: '资产' }));
    await user.click(screen.getByRole('link', { name: '任务工作台' }));
    expect(screen.getByRole('textbox', { name: '任务标题' })).toHaveValue('内部平台评估准备');
    expect(screen.getByRole('textbox', { name: '任务标题' })).toBeDisabled();
    expect(screen.getByTestId('current-command-id')).toHaveTextContent(commandId);
    expect(screen.getByRole('button', { name: '创建草稿' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '查询原命令' }));
    await screen.findByRole('heading', { name: '已受理' });
    expect(apiMock.acceptCommand).toHaveBeenCalledTimes(1);
    expect(apiMock.getCommandReceipt).toHaveBeenCalledWith(
      analystProject.id,
      commandId,
      expect.any(AbortSignal),
      '内部平台评估准备',
    );
  });
});
