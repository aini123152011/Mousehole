-- name: EnsureProjectScopeState :exec
-- provision 与项目同事务创建稳定行，幂等补齐但不重置策略或撤销历史。
WITH letter AS (
 INSERT INTO authorization_letters(project_id) VALUES ($1) ON CONFLICT DO NOTHING
), state AS (
 INSERT INTO project_policy_state(project_id) VALUES ($1) ON CONFLICT DO NOTHING
)
INSERT INTO scope_command_namespaces(kind,project_id) VALUES ('project',$1) ON CONFLICT DO NOTHING;

-- name: GetScopeNamespace :one
SELECT id FROM scope_command_namespaces WHERE project_id = sqlc.narg(project_id) OR (kind = 'global' AND sqlc.narg(project_id)::uuid IS NULL);

-- name: InsertScopeCommand :one
INSERT INTO scope_commands(namespace_id,command_id,project_id,actor_id,operator_source,operator_ref,kind,payload,payload_hash,accepted_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
-- 命令有主键与组合 FK 所需唯一键，全部 unique 都可能在并发竞争中先报告冲突。
ON CONFLICT DO NOTHING RETURNING *;

-- name: GetScopeCommand :one
SELECT * FROM scope_commands WHERE namespace_id = $1 AND command_id = $2;

-- name: FreezeScopeCommand :exec
UPDATE scope_commands SET receipt = $3 WHERE namespace_id = $1 AND command_id = $2;

-- name: InsertScopeAudit :exec
INSERT INTO scope_audit_events(namespace_id,command_id,actor_id,operator_source,event_type,summary,created_at) VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: InsertScopeOutbox :exec
INSERT INTO scope_outbox_events(namespace_id,command_id,event_type,summary,created_at) VALUES($1,$2,$3,$4,$5);
