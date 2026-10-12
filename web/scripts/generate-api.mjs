import { readFile, writeFile, mkdir, readdir } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import openapiTS, { astToString } from 'openapi-typescript';
import { parse } from 'yaml';

// 类型与运行时校验 schema 都来自同一 OpenAPI，避免手写字段副本漂移。
const source = new URL('../../api/openapi.yaml', import.meta.url);
const output = new URL('../src/api/generated/', import.meta.url);
const document = parse(await readFile(source, 'utf8'));
const types = astToString(await openapiTS(source));
const definitions = JSON.parse(
  JSON.stringify(document.components.schemas).replaceAll('#/components/schemas/', '#/$defs/'),
);
const runtime = JSON.stringify(
  { $schema: 'https://json-schema.org/draft/2020-12/schema', $defs: definitions },
  null,
  2,
);
const files = {
  'schema.ts': `// 此文件由 pnpm run generate 生成，请修改 api/openapi.yaml。\n${types}`,
  'schemas.json': `${runtime}\n`,
};
const checkOnly = process.argv.includes('--check');
if (checkOnly) {
  // 检查完整文件集合，防止旧文件残留；检查模式不创建或覆盖输出目录。
  const entries = await readdir(output, { recursive: true, withFileTypes: true }).catch(() => []);
  const actualNames = entries
    .filter((entry) => !entry.isDirectory())
    .map((entry) => relative(fileURLToPath(output), join(entry.parentPath, entry.name)))
    .sort();
  if (JSON.stringify(actualNames) !== JSON.stringify(Object.keys(files).sort())) {
    throw new Error('生成文件集合已变化，请清理多余文件后运行 pnpm run generate。');
  }
} else {
  await mkdir(output, { recursive: true });
}
for (const [name, value] of Object.entries(files)) {
  const target = new URL(name, output);
  if (checkOnly) {
    const current = await readFile(target, 'utf8').catch(() => '');
    if (current !== value) throw new Error(`生成结果已变化：${name}，请运行 pnpm run generate。`);
  } else {
    await writeFile(target, value, 'utf8');
  }
}
console.log(checkOnly ? 'OpenAPI 生成结果一致。' : 'OpenAPI 生成完成。');
