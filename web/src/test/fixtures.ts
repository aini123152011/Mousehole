import type { CommandReceipt, Session } from '../api/client';
import type { components } from '../api/generated/schema';

/** 单独导出的稳定项目夹具，避免测试通过未检查索引访问数组。 */
export const analystProject: components['schemas']['ProjectAccess'] = {
  id: '10000000-0000-4000-8000-000000000002',
  name: '测试项目',
  role: 'analyst',
  capabilities: ['project:read', 'task_draft:create'],
  version: '1',
};

/** 合成企业会话，测试不会访问真实 IdP 或目标系统。 */
export const enterpriseSession: Session = {
  actor_id: '10000000-0000-4000-8000-000000000001',
  display_name: '测试分析员',
  email: 'analyst@example.invalid',
  projects: [analystProject],
  csrf_token: 'test-csrf-token-value-with-at-least-32-chars',
  expires_at: '2026-10-12T09:02:03Z',
};

/** 保留旧命令测试所需的项目字段别名。 */
export const developmentSession = {
  ...enterpriseSession,
  project_id: analystProject.id,
  project_name: analystProject.name,
};

/** 生成合成权威回执，允许测试绑定指定命令 ID 和标题。 */
export function receiptFor(commandId: string, title = '内部平台评估准备'): CommandReceipt {
  return {
    command_id: commandId,
    project_id: analystProject.id,
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
