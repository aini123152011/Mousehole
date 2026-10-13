import type {
  AuthorizationContent,
  AuthorizationVersion,
  PolicyDecision,
  ScopeCommandReceipt,
} from '../api/client';
import { analystProject } from './fixtures';

/** 全部为保留测试地址；fixture 不会连接目标或假称已执行。 */
export const scopeIds = {
  version: '00000000-0000-4000-8000-000000000041',
  entry: '00000000-0000-4000-8000-000000000042',
  decision: '00000000-0000-4000-8000-000000000043',
  command: '00000000-0000-4000-8000-000000000044',
} as const;
/** 包含全部固定语义的合成授权内容，预算保持十进制字符串。 */
export const authorizationContent: AuthorizationContent = {
  title: '合成测试范围',
  basis_ref: '测试依据',
  contact: '测试联系人',
  valid_from: '2026-10-12T00:00:00Z',
  expires_at: '2026-10-13T00:00:00Z',
  algorithm_version: 'target-v1',
  policy_version: 'policy-v1',
  catalog_version: 'precheck-v1',
  max_operation_level: 'non_destructive',
  budget: { max_requests: '100', max_concurrency: '2', max_requests_per_second: '1' },
  redirect_policy: 'deny',
  credential_policy: 'none',
  entries: [
    {
      kind: 'domain',
      value: 'example.invalid',
      include_subdomains: true,
      operations: ['dns_lookup'],
      ports: [],
      resolved_ip_cidrs: [],
    },
  ],
};
/** 保存后的版本快照；复制表单允许编辑新草稿，不编辑此对象。 */
export const authorizationVersion: AuthorizationVersion = {
  id: scopeIds.version,
  project_id: analystProject.id,
  sequence: '9007199254740993',
  content_hash: 'a'.repeat(64),
  content: authorizationContent,
  entry_ids: [scopeIds.entry],
  state: 'candidate',
  display_state: 'candidate',
  created_at: '2026-10-12T01:00:00Z',
};
/** 静态匹配仍然包含待检查项，不提供执行令牌。 */
export const policyDecision: PolicyDecision = {
  id: scopeIds.decision,
  project_id: analystProject.id,
  command_id: scopeIds.command,
  authorization_version_id: scopeIds.version,
  operation: 'dns_lookup',
  canonical_target: 'example.invalid',
  parameters: {},
  outcome: 'requires_runtime_checks',
  reason_codes: ['scope_matched'],
  message: '静态范围已匹配，运行时仍需复核。',
  matched_entry_id: scopeIds.entry,
  checked_at: '2026-10-12T02:00:00Z',
  content_hash: 'a'.repeat(64),
  global_revision: '1',
  project_revision: '2',
  algorithm_version: 'target-v1',
  policy_version: 'policy-v1',
  catalog_version: 'precheck-v1',
  runtime_checks: [
    { code: 'current_authorization', status: 'required', message: '重新验证当前授权与成员权限' },
    { code: 'final_ip', status: 'not_applicable', message: 'DNS-only 操作不直接连接最终地址' },
  ],
};
/** 构造可恢复的原回执，同 ID 保持检查时刻不变。 */
export function precheckReceipt(commandId: string = scopeIds.command): ScopeCommandReceipt {
  return {
    command_id: commandId,
    project_id: analystProject.id,
    kind: 'precheck_target',
    state: 'accepted',
    accepted_at: policyDecision.checked_at,
    result: { type: 'precheck', decision: { ...policyDecision, command_id: commandId } },
  };
}
