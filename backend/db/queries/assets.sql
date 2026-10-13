-- name: UpsertAsset :one
-- 调用方按 canonical key 排序，冲突保持原资产身份，不泄露其他项目存在性。
INSERT INTO assets(id,kind,canonical_value,algorithm_version) VALUES($1,$2,$3,'target-v1')
ON CONFLICT(kind,canonical_value,algorithm_version) DO UPDATE SET canonical_value = EXCLUDED.canonical_value RETURNING id;

-- name: InsertProjectAsset :execrows
INSERT INTO project_assets(project_id,asset_id,created_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;

-- name: InsertAssetSource :exec
INSERT INTO asset_import_sources(project_id,asset_id,namespace_id,command_id,source,notes,external_ref,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: ListProjectAssets :many
-- seek 使用项目关联时间，不暴露全企业首次录入时间；参数转义 LIKE 特殊字符。
SELECT a.id,a.kind,a.canonical_value,a.algorithm_version,pa.created_at
FROM project_assets pa JOIN assets a ON a.id = pa.asset_id
WHERE pa.project_id = $1 AND ($2::text = '' OR a.kind = $2)
 AND ($3::text = '' OR a.canonical_value ILIKE '%' || $3 || '%' ESCAPE '\')
 AND (sqlc.narg(before_at)::timestamptz IS NULL OR (pa.created_at,a.id) < (sqlc.narg(before_at),sqlc.narg(before_id)::uuid))
ORDER BY pa.created_at DESC,a.id DESC LIMIT $4;

-- name: GetProjectAsset :one
SELECT a.id,a.kind,a.canonical_value,a.algorithm_version,pa.created_at FROM project_assets pa JOIN assets a ON a.id = pa.asset_id WHERE pa.project_id = $1 AND pa.asset_id = $2;

-- name: FindProjectAsset :one
SELECT a.id FROM project_assets pa JOIN assets a ON a.id = pa.asset_id WHERE pa.project_id = $1 AND a.kind = $2 AND a.canonical_value = $3 AND a.algorithm_version = 'target-v1';

-- name: ListAssetSources :many
SELECT command_id,source,notes,external_ref,created_at FROM asset_import_sources WHERE project_id = $1 AND asset_id = $2 ORDER BY created_at,command_id;
