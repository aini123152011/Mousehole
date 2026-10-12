import type { CommandReceipt, DevelopmentSession } from '../api/client';

/** 合成固定开发身份，测试不会访问任何目标系统。 */
export const developmentSession: DevelopmentSession = {
  mode: 'development',
  actor_id: '00000000-0000-4000-8000-000000000001',
  project_id: '00000000-0000-4000-8000-000000000002',
  project_name: '本地演示项目',
};

/** 生成合成权威回执，允许测试绑定指定命令 ID 和标题。 */
export function receiptFor(commandId: string, title = '内部平台评估准备'): CommandReceipt {
  return {
    command_id: commandId,
    project_id: developmentSession.project_id,
    kind: 'create_task_draft',
    state: 'accepted',
    task_id: '00000000-0000-4000-8000-000000000099',
    task_title: title,
    task_state: 'draft',
    version: '1',
    accepted_at: '2026-10-12T01:02:03Z',
  };
}

/** 可控 Promise 用于制造响应迟到；测试依靠明确完成信号，不用固定等待。 */
export function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}
