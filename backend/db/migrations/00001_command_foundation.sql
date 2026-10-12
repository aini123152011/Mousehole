-- 目的：建立本地草稿命令、业务草稿、审计和待投递事件的原子边界。
-- 依赖：空的应用 schema；先执行此迁移，再启动 API。
-- 兼容：仅新增表；建表持有 schema DDL 锁，不回填既有数据。
-- 事务：goose 默认在事务中执行本迁移；唯一约束串行化同一命令的竞争。
-- 验证：集成测试检查并发单份写入、约束拒绝和四类记录整体回滚。
-- 回滚：下迁移会永久删除本切片数据，仅可用于明确可丢弃的开发数据库。
-- +goose Up
CREATE TABLE commands (
    project_id UUID NOT NULL,
    command_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind = 'create_task_draft'),
    state TEXT NOT NULL DEFAULT 'accepted' CHECK (state = 'accepted'),
    task_id UUID NOT NULL UNIQUE,
    task_title TEXT NOT NULL CHECK (char_length(task_title) BETWEEN 1 AND 120),
    task_state TEXT NOT NULL DEFAULT 'draft' CHECK (task_state = 'draft'),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version = 1),
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, command_id)
);

CREATE TABLE tasks (
    task_id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    command_id UUID NOT NULL,
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    state TEXT NOT NULL DEFAULT 'draft' CHECK (state = 'draft'),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version = 1),
    UNIQUE (project_id, command_id),
    FOREIGN KEY (project_id, command_id) REFERENCES commands (project_id, command_id)
);

-- 延迟检查允许先占用命令唯一键、再创建草稿；提交时两者必须存在。
ALTER TABLE commands ADD CONSTRAINT commands_task_fk
    FOREIGN KEY (task_id) REFERENCES tasks (task_id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE audit_events (
    project_id UUID NOT NULL,
    command_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    task_id UUID NOT NULL REFERENCES tasks (task_id),
    event_type TEXT NOT NULL CHECK (event_type = 'task_draft_created'),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, command_id),
    FOREIGN KEY (project_id, command_id) REFERENCES commands (project_id, command_id)
);

CREATE TABLE outbox_events (
    project_id UUID NOT NULL,
    command_id UUID NOT NULL,
    task_id UUID NOT NULL REFERENCES tasks (task_id),
    event_type TEXT NOT NULL CHECK (event_type = 'task_draft_created'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, command_id),
    FOREIGN KEY (project_id, command_id) REFERENCES commands (project_id, command_id)
);

COMMENT ON TABLE commands IS '命令受理权威记录；已受理不表示任何目标行为已执行。';
COMMENT ON COLUMN commands.actor_id IS '受理时身份，与命令参数一起绑定幂等键。';
COMMENT ON COLUMN commands.task_title IS '受理时规范化标题快照，回放返回原值。';
COMMENT ON COLUMN commands.version IS '业务版本；API 用十进制字符串表示。';
COMMENT ON COLUMN commands.accepted_at IS '原始受理时刻；API 输出 UTC RFC3339。';
COMMENT ON TABLE tasks IS '仅保存任务草稿，不具有执行能力。';
COMMENT ON TABLE audit_events IS '与命令及草稿同事务提交的业务审计，不记录完整请求参数。';
COMMENT ON TABLE outbox_events IS '事务事件记录；本切片没有 dispatcher，不消费或触发外部行为。';

-- +goose Down
ALTER TABLE commands DROP CONSTRAINT commands_task_fk;
DROP TABLE outbox_events;
DROP TABLE audit_events;
DROP TABLE tasks;
DROP TABLE commands;
