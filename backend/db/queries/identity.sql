-- name: InsertUser :execrows
INSERT INTO users (id, display_name, email)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: InsertExternalIdentity :execrows
INSERT INTO external_identities (issuer, subject, user_id, email, display_name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (issuer, subject) DO NOTHING;

-- name: GetExternalIdentity :one
SELECT * FROM external_identities WHERE issuer = $1 AND subject = $2;

-- name: InsertProject :execrows
INSERT INTO projects (id, name)
VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: InsertProjectMembership :execrows
INSERT INTO project_memberships (project_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, user_id) DO NOTHING;

-- name: GetProjectMembership :one
SELECT * FROM project_memberships WHERE project_id = $1 AND user_id = $2;

-- name: InsertAccessAuditEvent :exec
INSERT INTO access_audit_events (
    id, actor_id, operator_source, event_type, object_type, object_id, request_id, summary
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertLoginAttempt :exec
INSERT INTO oidc_login_attempts (
    state_digest, browser_digest, nonce_digest, pkce_verifier_encrypted, return_path, expires_at
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ConsumeLoginAttempt :one
UPDATE oidc_login_attempts
SET consumed_at = CURRENT_TIMESTAMP
WHERE state_digest = $1
  AND browser_digest = $2
  AND consumed_at IS NULL
  AND expires_at > CURRENT_TIMESTAMP
RETURNING *;

-- name: GetProvisionedIdentity :one
SELECT
    u.id,
    u.display_name,
    u.email
FROM external_identities AS ei
JOIN users AS u ON u.id = ei.user_id
WHERE ei.issuer = $1
  AND ei.subject = $2
  AND u.status = 'active';

-- name: InsertSession :exec
INSERT INTO sessions (
    token_digest, user_id, idle_expires_at, absolute_expires_at
) VALUES ($1, $2, $3, $4);

-- name: GetActiveSession :one
SELECT
    s.user_id,
    s.last_seen_at,
    s.idle_expires_at,
    s.absolute_expires_at,
    u.display_name,
    u.email
FROM sessions AS s
JOIN users AS u ON u.id = s.user_id
WHERE s.token_digest = $1
  AND s.revoked_at IS NULL
  AND s.idle_expires_at > CURRENT_TIMESTAMP
  AND s.absolute_expires_at > CURRENT_TIMESTAMP
  AND u.status = 'active';

-- name: TouchSession :exec
UPDATE sessions
SET
    last_seen_at = CURRENT_TIMESTAMP,
    idle_expires_at = LEAST(absolute_expires_at, CURRENT_TIMESTAMP + sqlc.arg(idle_ttl)::INTERVAL)
WHERE token_digest = sqlc.arg(token_digest)
  AND last_seen_at <= CURRENT_TIMESTAMP - INTERVAL '5 minutes';

-- name: ListActiveProjects :many
SELECT
    p.id,
    p.name,
    pm.role,
    pm.version
FROM project_memberships AS pm
JOIN projects AS p ON p.id = pm.project_id
WHERE pm.user_id = $1
  AND pm.state = 'active'
  AND p.status = 'active'
ORDER BY p.name, p.id;

-- name: RevokeSession :execrows
UPDATE sessions
SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP)
WHERE token_digest = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :execrows
UPDATE sessions
SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP)
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: LockUserForAdmin :one
-- 管理变更先锁定稳定用户行，用于区分目标不存在与幂等重放。
SELECT status
FROM users
WHERE id = $1
FOR UPDATE;

-- name: DisableUser :execrows
UPDATE users
SET status = 'disabled', updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'active';

-- name: LockProjectMembership :one
-- FOR SHARE 保持成员行到命令事务结束；撤销使用 FOR UPDATE 锁定同一行。
SELECT pm.role
FROM project_memberships AS pm
JOIN users AS u ON u.id = pm.user_id
JOIN projects AS p ON p.id = pm.project_id
WHERE pm.project_id = $1
  AND pm.user_id = $2
  AND pm.state = 'active'
  AND u.status = 'active'
  AND p.status = 'active'
FOR SHARE OF pm;

-- name: RevokeProjectMembership :one
UPDATE project_memberships
SET
    state = 'revoked',
    version = version + 1,
    revoked_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE project_id = $1 AND user_id = $2 AND state = 'active'
RETURNING *;

-- name: LockProjectMembershipForAdmin :one
-- 已撤销的成员行仍会返回，使管理命令可以安全地幂等重放。
SELECT state, role
FROM project_memberships
WHERE project_id = $1 AND user_id = $2
FOR UPDATE;
