-- 目的：建立企业身份、项目成员、一次性 OIDC 登录尝试和服务端会话权威状态。
-- 依赖：00001 已创建命令、草稿、审计与 outbox 表。
-- 兼容：保留既有固定 actor/project 原值，并建立不可登录的历史关联记录。
-- 事务：goose 默认在单事务中执行；成员撤销与命令权限锁依赖同一成员行。
-- 回滚：下迁移会删除身份数据和外键，只允许可丢弃开发库；生产使用前向修复。
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
    email TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE external_identities (
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users (id),
    email TEXT,
    display_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issuer, subject),
    UNIQUE (user_id, issuer),
    CHECK (issuer <> '' AND subject <> '')
);

CREATE TABLE projects (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE project_memberships (
    project_id UUID NOT NULL REFERENCES projects (id),
    user_id UUID NOT NULL REFERENCES users (id),
    role TEXT NOT NULL CHECK (role IN ('viewer', 'analyst')),
    state TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'revoked')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (project_id, user_id),
    CHECK ((state = 'active' AND revoked_at IS NULL) OR (state = 'revoked' AND revoked_at IS NOT NULL))
);

CREATE TABLE oidc_login_attempts (
    state_digest BYTEA PRIMARY KEY CHECK (octet_length(state_digest) = 32),
    browser_digest BYTEA NOT NULL CHECK (octet_length(browser_digest) = 32),
    nonce_digest BYTEA NOT NULL CHECK (octet_length(nonce_digest) = 32),
    pkce_verifier_encrypted BYTEA NOT NULL,
    return_path TEXT NOT NULL CHECK (return_path = '/'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);

CREATE TABLE sessions (
    token_digest BYTEA PRIMARY KEY CHECK (octet_length(token_digest) = 32),
    user_id UUID NOT NULL REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    idle_expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (idle_expires_at > created_at AND absolute_expires_at > created_at)
);

CREATE INDEX sessions_user_active_idx ON sessions (user_id, absolute_expires_at)
WHERE revoked_at IS NULL;

CREATE TABLE access_audit_events (
    id UUID PRIMARY KEY,
    actor_id UUID REFERENCES users (id),
    operator_source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    request_id UUID,
    summary JSONB NOT NULL DEFAULT '{}'::JSONB,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 旧固定身份只用于保持已发布命令外键完整性，不能创建会话或获得活动权限。
INSERT INTO users (id, display_name, status)
VALUES ('00000000-0000-4000-8000-000000000001', '历史本地开发身份', 'disabled');

INSERT INTO projects (id, name, status)
VALUES ('00000000-0000-4000-8000-000000000002', '历史本地演示项目', 'disabled');

INSERT INTO project_memberships (project_id, user_id, role, state, revoked_at)
VALUES (
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000001',
    'viewer',
    'revoked',
    CURRENT_TIMESTAMP
);

ALTER TABLE commands ADD CONSTRAINT commands_project_fk FOREIGN KEY (project_id) REFERENCES projects (id);
ALTER TABLE commands ADD CONSTRAINT commands_actor_fk FOREIGN KEY (actor_id) REFERENCES users (id);
ALTER TABLE audit_events ADD CONSTRAINT audit_events_actor_fk FOREIGN KEY (actor_id) REFERENCES users (id);

COMMENT ON TABLE users IS '平台稳定用户；外部身份资料变化不会改变 actor UUID。';
COMMENT ON TABLE external_identities IS '精确 issuer 与 subject 的预置绑定；email 和名称不参与授权。';
COMMENT ON TABLE projects IS '单企业部署内的项目访问边界。';
COMMENT ON TABLE project_memberships IS '项目角色权威；撤销保留原行并递增 version，供事务锁串行化。';
COMMENT ON TABLE oidc_login_attempts IS '一次性登录尝试；摘要防止数据库泄露可重放 state 或浏览器绑定值。';
COMMENT ON COLUMN oidc_login_attempts.pkce_verifier_encrypted IS '使用 auth key 派生子密钥加密的短期 PKCE verifier。';
COMMENT ON TABLE sessions IS '服务端会话；只保存随机 Cookie 原值的 SHA-256 摘要。';
COMMENT ON TABLE access_audit_events IS '身份与成员管理审计；summary 只能保存非秘密的稳定摘要。';

-- +goose Down
ALTER TABLE audit_events DROP CONSTRAINT audit_events_actor_fk;
ALTER TABLE commands DROP CONSTRAINT commands_actor_fk;
ALTER TABLE commands DROP CONSTRAINT commands_project_fk;
DROP TABLE access_audit_events;
DROP TABLE sessions;
DROP TABLE oidc_login_attempts;
DROP TABLE project_memberships;
DROP TABLE projects;
DROP TABLE external_identities;
DROP TABLE users;
