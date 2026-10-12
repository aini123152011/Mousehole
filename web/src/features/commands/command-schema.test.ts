import { describe, expect, it } from 'vitest';
import { commandLookupSchema, normalizeTaskTitle, taskDraftSchema } from './command-schema';

describe('中文草稿参数边界', () => {
  it('去除 Unicode 空白，并与后端一致保留 U+FEFF', () => {
    expect(normalizeTaskTitle('\u0085\u2003 标题 \u3000')).toBe('标题');
    expect(taskDraftSchema.parse({ title: '\uFEFF' }).title).toBe('\uFEFF');
    expect(taskDraftSchema.safeParse({ title: '\u0085\u2003\u3000' }).success).toBe(false);
  });

  it('按 Unicode 字符接受 120 个 emoji，拒绝 121 个字符', () => {
    expect(taskDraftSchema.parse({ title: ` ${'😀'.repeat(120)} ` }).title).toHaveLength(240);
    expect(taskDraftSchema.safeParse({ title: '😀'.repeat(121) }).success).toBe(false);
  });

  it('恢复查询只接受完整小写 UUID', () => {
    expect(
      commandLookupSchema.safeParse({ commandId: '00000000-0000-4000-8000-00000000000a' }).success,
    ).toBe(true);
    expect(
      commandLookupSchema.safeParse({ commandId: '00000000-0000-4000-8000-00000000000A' }).success,
    ).toBe(false);
    expect(commandLookupSchema.safeParse({ commandId: '新的命令' }).success).toBe(false);
  });
});
