-- name: InsertCommand :one
-- 唯一键争用等待对方提交；无返回行表示已存在，随后重新读取原记录。
INSERT INTO commands (project_id, command_id, actor_id, kind, task_id, task_title)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (project_id, command_id) DO NOTHING
RETURNING *;

-- name: GetCommand :one
-- 按项目与命令读取原始回执，身份及参数比较由命令服务统一完成。
SELECT * FROM commands WHERE project_id = $1 AND command_id = $2;

-- name: InsertTask :exec
INSERT INTO tasks (task_id, project_id, command_id, title)
VALUES ($1, $2, $3, $4);

-- name: InsertAuditEvent :exec
INSERT INTO audit_events (project_id, command_id, actor_id, task_id, event_type)
VALUES ($1, $2, $3, $4, 'task_draft_created');

-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (project_id, command_id, task_id, event_type)
VALUES ($1, $2, $3, 'task_draft_created');
