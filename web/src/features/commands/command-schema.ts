import { z } from 'zod';

/** 与 Go strings.TrimSpace 使用同一 Unicode White_Space 集合，保留 U+FEFF。 */
export function normalizeTaskTitle(title: string): string {
  return title.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
}

/** 表单按 Unicode 字符计数，避免把中文扩展字符或 emoji 当作两个字。 */
export const taskDraftSchema = z.object({
  title: z
    .string()
    .transform(normalizeTaskTitle)
    .refine((title) => Array.from(title).length > 0, '请输入任务标题。')
    .refine((title) => Array.from(title).length <= 120, '任务标题最多 120 个字符。'),
});

/** 恢复查询使用完整小写 UUID，不替用户换 ID 或重新发送命令。 */
export const commandLookupSchema = z.object({
  commandId: z
    .string()
    .trim()
    .regex(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
      '请输入完整的小写命令 ID。',
    ),
});
