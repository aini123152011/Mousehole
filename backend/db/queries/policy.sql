-- name: LockGlobalPolicyRead :one
SELECT revision FROM global_policy_state WHERE singleton FOR SHARE;

-- name: LockGlobalPolicyWrite :one
SELECT revision FROM global_policy_state WHERE singleton FOR UPDATE;

-- name: LockProjectPolicyRead :one
SELECT * FROM project_policy_state WHERE project_id = $1 FOR SHARE;

-- name: LockProjectPolicyWrite :one
SELECT * FROM project_policy_state WHERE project_id = $1 FOR UPDATE;

-- name: ScopeClock :one
-- 拿齐权限与策略锁后取唯一权威时刻，历史决策不重新解释为当前许可。
SELECT clock_timestamp()::timestamptz AS checked_at;

-- name: SetProjectPolicy :exec
UPDATE project_policy_state SET revision = revision + 1,active_version_id = $2 WHERE project_id = $1;

-- name: BumpGlobalPolicy :exec
UPDATE global_policy_state SET revision = revision + 1 WHERE singleton;

-- name: InsertScopeDeny :exec
INSERT INTO scope_denies(id,project_id,kind,value,include_subdomains,reason,created_at) VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: GetScopeDeny :one
SELECT * FROM scope_denies WHERE id = $1 AND (project_id = sqlc.narg(project_id) OR (project_id IS NULL AND sqlc.narg(project_id)::uuid IS NULL));

-- name: RevokeScopeDeny :exec
UPDATE scope_denies SET active = FALSE WHERE id = $1;

-- name: ListCurrentDenies :many
SELECT * FROM scope_denies WHERE active AND (project_id IS NULL OR project_id = $1) ORDER BY created_at,id;

-- name: InsertPolicyPrecheck :exec
INSERT INTO policy_prechecks(id,project_id,version_id,matched_entry_id,namespace_id,command_id,checked_at,decision) VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: GetPolicyPrecheck :one
SELECT decision FROM policy_prechecks WHERE project_id = $1 AND id = $2;

-- name: ListPolicyPrechecks :many
SELECT decision FROM policy_prechecks WHERE project_id = $1
AND (sqlc.narg(before_at)::timestamptz IS NULL OR (checked_at,id) < (sqlc.narg(before_at),sqlc.narg(before_id)::uuid))
ORDER BY checked_at DESC,id DESC LIMIT $2;
