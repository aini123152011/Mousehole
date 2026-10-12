import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CommandAPIError } from '../../api/client';
import { createQueryClient } from '../../app/query-client';
import { developmentSession, receiptFor } from '../../test/fixtures';
import { CommandWorkspace } from './command-workspace';

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
  render(
    <QueryClientProvider client={createQueryClient()}>
      <CommandWorkspace session={developmentSession} />
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
});
