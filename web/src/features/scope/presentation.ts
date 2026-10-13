import type { AuthorizationVersion, Asset, PolicyDecision } from '../../api/client';

/** 服务端 UTC 时间经客户端验证后才格式化，保留明确的本地时区标识。 */
export function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { timeZoneName: 'short' });
}
/** 机器状态的中文显示，不改变后台状态枚举。 */
export const versionLabels: Record<AuthorizationVersion['display_state'], string> = {
  candidate: '候选，尚未激活',
  active: '当前激活',
  superseded: '已被替代',
  revoked: '已撤销',
  expired: '已过期',
  not_yet_valid: '尚未到有效时间',
};
/** 资产类型使用中文，标识符在 API 保持英文。 */
export const assetLabels: Record<Asset['kind'], string> = {
  domain: '域名',
  ip: 'IP 地址',
  url: '精确 URL',
};
/** 预检查目录描述静态语义，不代表已审核或执行工具。 */
export const operationLabels: Record<PolicyDecision['operation'], string> = {
  dns_lookup: 'DNS 名称检查',
  tcp_connect: 'TCP 连接条件',
  http_metadata: 'HTTP 元数据条件（HEAD）',
};
