-- name: LockAccessUser :one
-- 稳定 user→project→membership 锁序第一步；检查状态由同事务领域边界负责。
SELECT status FROM users WHERE id = $1 FOR SHARE;

-- name: LockAccessProject :one
SELECT status FROM projects WHERE id = $1 FOR SHARE;

-- name: LockAccessMembership :one
SELECT state, role FROM project_memberships WHERE project_id = $1 AND user_id = $2 FOR SHARE;
