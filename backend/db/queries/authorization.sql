-- name: NextAuthorizationSequence :one
UPDATE authorization_letters SET next_sequence = next_sequence + 1 WHERE project_id = $1 RETURNING (next_sequence - 1)::bigint AS sequence;

-- name: InsertAuthorizationVersion :exec
INSERT INTO authorization_versions(id,project_id,sequence,content,content_hash,valid_from,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: InsertAuthorizationState :exec
INSERT INTO authorization_version_states(project_id,version_id,state) VALUES($1,$2,'candidate');

-- name: InsertAuthorizationEntry :exec
INSERT INTO authorization_scope_entries(id,project_id,version_id,ordinal,kind,value,include_subdomains,operations,ports,resolved_ip_cidrs) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);

-- name: GetAuthorizationVersion :one
SELECT v.*,s.state FROM authorization_versions v JOIN authorization_version_states s ON s.version_id = v.id WHERE v.project_id = $1 AND v.id = $2;

-- name: ListAuthorizationVersions :many
SELECT v.*,s.state FROM authorization_versions v JOIN authorization_version_states s ON s.version_id = v.id
WHERE v.project_id = $1 AND (sqlc.narg(before_at)::timestamptz IS NULL OR (v.created_at,v.id) < (sqlc.narg(before_at),sqlc.narg(before_id)::uuid))
ORDER BY v.created_at DESC,v.id DESC LIMIT $2;

-- name: ListAuthorizationEntryIDs :many
SELECT id FROM authorization_scope_entries WHERE project_id = $1 AND version_id = $2 ORDER BY ordinal;

-- name: SetAuthorizationState :exec
UPDATE authorization_version_states SET state = $3 WHERE project_id = $1 AND version_id = $2;
-- name: LockAuthorizationLetter :one
-- candidate 分配序号前按策略→letter→command 顺序锁定，回放不改变序号。
SELECT project_id FROM authorization_letters WHERE project_id = $1 FOR UPDATE;
