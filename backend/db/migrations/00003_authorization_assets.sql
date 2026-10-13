-- 目的：独立资产、不可变授权及静态决策账本；保留 00001/00002 的全部历史合同。
-- 部署：先迁移后 API。回填项目稳定行，需要短暂 DDL 锁；上线前按项目数评估。
-- 事务：goose 同事务执行。Down 仅用于随机测试 schema，删除新历史且不能恢复。
-- +goose Up
CREATE TABLE scope_command_namespaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('project','global')),
    project_id UUID UNIQUE REFERENCES projects(id),
    CHECK ((kind = 'project' AND project_id IS NOT NULL) OR (kind = 'global' AND project_id IS NULL)),
    UNIQUE (id, project_id)
);
CREATE UNIQUE INDEX scope_namespace_global ON scope_command_namespaces(kind) WHERE kind = 'global';
INSERT INTO scope_command_namespaces(kind) VALUES ('global');
INSERT INTO scope_command_namespaces(kind,project_id) SELECT 'project',id FROM projects;

CREATE TABLE global_policy_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton),
    revision BIGINT NOT NULL DEFAULT 1 CHECK(revision > 0)
);
INSERT INTO global_policy_state DEFAULT VALUES;
CREATE TABLE authorization_letters (
    project_id UUID PRIMARY KEY REFERENCES projects(id),
    next_sequence BIGINT NOT NULL DEFAULT 1 CHECK(next_sequence > 0)
);
INSERT INTO authorization_letters(project_id) SELECT id FROM projects;
CREATE TABLE authorization_versions (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES authorization_letters(project_id),
    sequence BIGINT NOT NULL CHECK(sequence > 0),
    content JSONB NOT NULL CHECK(jsonb_typeof(content) = 'object'),
    content_hash TEXT NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
    valid_from TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK(expires_at > valid_from),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(project_id,sequence), UNIQUE(project_id,id)
);
CREATE TABLE authorization_version_states (
    project_id UUID NOT NULL,
    version_id UUID PRIMARY KEY,
    state TEXT NOT NULL CHECK(state IN ('candidate','active','superseded','revoked')),
    FOREIGN KEY(project_id,version_id) REFERENCES authorization_versions(project_id,id)
);
CREATE UNIQUE INDEX authorization_one_active ON authorization_version_states(project_id) WHERE state = 'active';
CREATE TABLE authorization_scope_entries (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    version_id UUID NOT NULL,
    ordinal INTEGER NOT NULL CHECK(ordinal >= 0),
    kind TEXT NOT NULL CHECK(kind IN ('domain','ip','cidr','url_exact')),
    value TEXT NOT NULL,
    include_subdomains BOOLEAN NOT NULL,
    operations TEXT[] NOT NULL,
    ports INTEGER[] NOT NULL,
    resolved_ip_cidrs TEXT[] NOT NULL,
    UNIQUE(version_id,ordinal), UNIQUE(project_id,version_id,id),
    FOREIGN KEY(project_id,version_id) REFERENCES authorization_versions(project_id,id),
    CHECK(kind = 'domain' OR NOT include_subdomains)
);
CREATE TABLE project_policy_state (
    project_id UUID PRIMARY KEY REFERENCES projects(id),
    revision BIGINT NOT NULL DEFAULT 1 CHECK(revision > 0),
    active_version_id UUID,
    FOREIGN KEY(project_id,active_version_id) REFERENCES authorization_versions(project_id,id) DEFERRABLE INITIALLY DEFERRED
);
INSERT INTO project_policy_state(project_id) SELECT id FROM projects;

CREATE TABLE scope_commands (
    namespace_id UUID NOT NULL REFERENCES scope_command_namespaces(id),
    command_id UUID NOT NULL,
    project_id UUID REFERENCES projects(id),
    actor_id UUID REFERENCES users(id),
    operator_source TEXT NOT NULL CHECK(operator_source IN ('web','admin_cli')),
    operator_ref TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('import_assets','create_authorization_version','precheck_target','activate_authorization_version','revoke_authorization_version','add_scope_deny','revoke_scope_deny')),
    payload JSONB NOT NULL,
    payload_hash TEXT NOT NULL CHECK(payload_hash ~ '^[0-9a-f]{64}$'),
    accepted_at TIMESTAMPTZ NOT NULL,
    receipt JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY(namespace_id,command_id), UNIQUE(namespace_id,command_id,project_id),
    FOREIGN KEY(namespace_id,project_id) REFERENCES scope_command_namespaces(id,project_id),
    CHECK((operator_source = 'web' AND actor_id IS NOT NULL AND project_id IS NOT NULL AND operator_ref = '') OR (operator_source = 'admin_cli' AND actor_id IS NULL AND length(operator_ref) > 0))
);
CREATE TABLE assets (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL CHECK(kind IN ('domain','ip','url')),
    canonical_value TEXT NOT NULL,
    algorithm_version TEXT NOT NULL CHECK(algorithm_version = 'target-v1'),
    UNIQUE(kind,canonical_value,algorithm_version)
);
CREATE TABLE project_assets (
    project_id UUID NOT NULL REFERENCES projects(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(project_id,asset_id)
);
CREATE TABLE asset_import_sources (
    project_id UUID NOT NULL,
    asset_id UUID NOT NULL,
    namespace_id UUID NOT NULL,
    command_id UUID NOT NULL,
    source TEXT NOT NULL CHECK(source IN ('manual','api_import')),
    notes TEXT NOT NULL,
    external_ref TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(project_id,asset_id,command_id),
    FOREIGN KEY(project_id,asset_id) REFERENCES project_assets(project_id,asset_id),
    FOREIGN KEY(namespace_id,command_id,project_id) REFERENCES scope_commands(namespace_id,command_id,project_id)
);
CREATE TABLE scope_denies (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id),
    kind TEXT NOT NULL CHECK(kind IN ('domain','ip','cidr','url_exact')),
    value TEXT NOT NULL,
    include_subdomains BOOLEAN NOT NULL,
    reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 500),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    CHECK(kind = 'domain' OR NOT include_subdomains)
);
CREATE TABLE policy_prechecks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    version_id UUID NOT NULL,
    matched_entry_id UUID,
    namespace_id UUID NOT NULL,
    command_id UUID NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL,
    decision JSONB NOT NULL,
    UNIQUE(namespace_id,command_id),
    FOREIGN KEY(project_id,version_id) REFERENCES authorization_versions(project_id,id),
    FOREIGN KEY(project_id,version_id,matched_entry_id) REFERENCES authorization_scope_entries(project_id,version_id,id),
    FOREIGN KEY(namespace_id,command_id,project_id) REFERENCES scope_commands(namespace_id,command_id,project_id)
);
CREATE TABLE scope_audit_events (
    namespace_id UUID NOT NULL,
    command_id UUID NOT NULL,
    actor_id UUID REFERENCES users(id),
    operator_source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    summary JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(namespace_id,command_id),
    FOREIGN KEY(namespace_id,command_id) REFERENCES scope_commands(namespace_id,command_id)
);
CREATE TABLE scope_outbox_events (
    namespace_id UUID NOT NULL,
    command_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    summary JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(namespace_id,command_id),
    FOREIGN KEY(namespace_id,command_id) REFERENCES scope_commands(namespace_id,command_id)
);
CREATE INDEX project_asset_seek ON project_assets(project_id,created_at DESC,asset_id DESC);
CREATE INDEX authorization_version_seek ON authorization_versions(project_id,created_at DESC,id DESC);
CREATE INDEX policy_precheck_seek ON policy_prechecks(project_id,checked_at DESC,id DESC);
CREATE INDEX scope_deny_current ON scope_denies(project_id) WHERE active;

-- +goose StatementBegin
CREATE FUNCTION reject_scope_history_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '不可变历史禁止修改'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER authorization_body_immutable BEFORE UPDATE OR DELETE ON authorization_versions FOR EACH ROW EXECUTE FUNCTION reject_scope_history_change();
CREATE TRIGGER authorization_entry_immutable BEFORE UPDATE OR DELETE ON authorization_scope_entries FOR EACH ROW EXECUTE FUNCTION reject_scope_history_change();
CREATE TRIGGER policy_precheck_immutable BEFORE UPDATE OR DELETE ON policy_prechecks FOR EACH ROW EXECUTE FUNCTION reject_scope_history_change();
CREATE TRIGGER scope_audit_immutable BEFORE UPDATE OR DELETE ON scope_audit_events FOR EACH ROW EXECUTE FUNCTION reject_scope_history_change();
CREATE TRIGGER scope_outbox_immutable BEFORE UPDATE OR DELETE ON scope_outbox_events FOR EACH ROW EXECUTE FUNCTION reject_scope_history_change();
CREATE TRIGGER asset_source_immutable BEFORE UPDATE OR DELETE ON asset_import_sources FOR EACH ROW EXECUTE FUNCTION reject_scope_history_change();
-- +goose StatementBegin
CREATE FUNCTION freeze_scope_command() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR OLD.receipt <> '{}'::jsonb THEN RAISE EXCEPTION '冻结回执禁止修改'; END IF;
    IF (to_jsonb(NEW) - 'receipt') <> (to_jsonb(OLD) - 'receipt') OR NEW.receipt = '{}'::jsonb THEN RAISE EXCEPTION '命令只可冻结一次回执'; END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER scope_command_freeze BEFORE UPDATE OR DELETE ON scope_commands FOR EACH ROW EXECUTE FUNCTION freeze_scope_command();
-- +goose StatementBegin
CREATE FUNCTION verify_scope_command_complete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS(SELECT 1 FROM scope_commands WHERE namespace_id = NEW.namespace_id AND command_id = NEW.command_id AND receipt = '{}'::jsonb) THEN RAISE EXCEPTION '受理回执尚未冻结'; END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER scope_command_complete AFTER INSERT ON scope_commands DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION verify_scope_command_complete();
-- +goose StatementBegin
CREATE FUNCTION verify_authorization_active() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p UUID; pointer UUID; actual UUID;
BEGIN
    p := COALESCE(NEW.project_id, OLD.project_id);
    SELECT active_version_id INTO pointer FROM project_policy_state WHERE project_id = p;
    SELECT version_id INTO actual FROM authorization_version_states WHERE project_id = p AND state = 'active';
    IF pointer IS DISTINCT FROM actual THEN RAISE EXCEPTION '活动指针与版本状态不一致'; END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER authorization_pointer_consistent AFTER INSERT OR UPDATE ON project_policy_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION verify_authorization_active();
CREATE CONSTRAINT TRIGGER authorization_state_consistent AFTER INSERT OR UPDATE OR DELETE ON authorization_version_states DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION verify_authorization_active();
COMMENT ON TABLE policy_prechecks IS '纯本地静态决策历史，不是执行许可或已验证观测。';
COMMENT ON COLUMN project_policy_state.revision IS '授权与禁止变更修订号，API 十进制字符串；候选保存不递增。';
COMMENT ON TABLE scope_outbox_events IS '与受理同事务追加；本切片无消费者，不触发目标执行。';

-- +goose Down
DROP TABLE scope_outbox_events,scope_audit_events,policy_prechecks,scope_denies,asset_import_sources,project_assets,assets,scope_commands,project_policy_state,authorization_scope_entries,authorization_version_states,authorization_versions,authorization_letters,global_policy_state,scope_command_namespaces CASCADE;
DROP FUNCTION verify_authorization_active(),verify_scope_command_complete(),freeze_scope_command(),reject_scope_history_change();
