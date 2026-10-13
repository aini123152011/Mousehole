-- 目的：前向补强 00003 的可空 namespace 绑定、范围正文与命令完整性。
-- 部署：保持已执行的 00003 不变；已有历史先校验，再安装提交时约束。
-- 事务：全部操作由 goose 原子提交，异常数据导致升级失败，不清理或改写历史。
-- +goose Up
-- MATCH SIMPLE 遇到 NULL 会跳过复合 FK，必须显式比较全局/项目 namespace。
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS(SELECT 1 FROM scope_commands c JOIN scope_command_namespaces n ON n.id=c.namespace_id WHERE n.project_id IS DISTINCT FROM c.project_id) THEN RAISE EXCEPTION '历史命令命名空间与项目不一致'; END IF;
END;
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION verify_scope_namespace() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS(SELECT 1 FROM scope_command_namespaces WHERE id=NEW.namespace_id AND project_id IS NOT DISTINCT FROM NEW.project_id) THEN RAISE EXCEPTION '命令命名空间与项目不一致'; END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
-- 开发验收中 00003 可能已带同一约束；前向兼容该状态，不移除已安装的保护。
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='scope_commands'::regclass AND tgname='scope_namespace_consistent') THEN
        CREATE TRIGGER scope_namespace_consistent BEFORE INSERT ON scope_commands FOR EACH ROW EXECUTE FUNCTION verify_scope_namespace();
    ELSIF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='scope_commands'::regclass AND tgname='scope_namespace_consistent' AND tgfoid='verify_scope_namespace()'::regprocedure AND tgtype=7 AND tgenabled='O') THEN
        RAISE EXCEPTION '已有命名空间触发器与预期合同不同';
    END IF;
END;
$$;
-- +goose StatementEnd

-- 固定版本与正整数预算在数据库也校验；JSON 缺字段不能利用 CHECK 的 NULL 通过。
ALTER TABLE authorization_versions ADD CONSTRAINT authorization_content_contract CHECK(COALESCE(
    content->>'algorithm_version'='target-v1' AND content->>'policy_version'='policy-v1' AND content->>'catalog_version'='precheck-v1'
    AND content->>'max_operation_level'='non_destructive' AND content->>'redirect_policy'='deny' AND content->>'credential_policy'='none'
    AND jsonb_typeof(content->'entries')='array' AND jsonb_array_length(content->'entries') BETWEEN 1 AND 100
    AND jsonb_typeof(content->'budget')='object'
    AND jsonb_typeof(content->'budget'->'max_requests')='string' AND content->'budget'->>'max_requests' ~ '^[1-9][0-9]{0,18}$'
    AND jsonb_typeof(content->'budget'->'max_concurrency')='string' AND content->'budget'->>'max_concurrency' ~ '^[1-9][0-9]{0,18}$'
    AND jsonb_typeof(content->'budget'->'max_requests_per_second')='string' AND content->'budget'->>'max_requests_per_second' ~ '^[1-9][0-9]{0,18}$'
    AND (content->'budget'->>'max_requests')::NUMERIC <= 9223372036854775807
    AND (content->'budget'->>'max_concurrency')::NUMERIC <= 9223372036854775807
    AND (content->'budget'->>'max_requests_per_second')::NUMERIC <= 9223372036854775807
    AND (content->'budget'->>'max_concurrency')::NUMERIC <= (content->'budget'->>'max_requests')::NUMERIC
,FALSE));
ALTER TABLE authorization_scope_entries ADD CONSTRAINT authorization_entry_contract CHECK(
    cardinality(operations)>0 AND operations <@ ARRAY['dns_lookup','tcp_connect','http_metadata']::TEXT[]
    AND 0 < ALL(ports) AND 65536 > ALL(ports)
);

-- 正文不可变后，逐条 INSERT 仍可能扩大范围；提交时核对完整有序集合封住该入口。
-- +goose StatementBegin
CREATE FUNCTION authorization_entries_match(v UUID) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
    SELECT COALESCE((SELECT content->'entries' FROM authorization_versions WHERE id=v)=
        (SELECT COALESCE(jsonb_agg(jsonb_build_object('kind',kind,'value',value,'include_subdomains',include_subdomains,'operations',operations,'ports',ports,'resolved_ip_cidrs',resolved_ip_cidrs) ORDER BY ordinal),'[]'::jsonb) FROM authorization_scope_entries WHERE version_id=v),FALSE);
$$;
-- +goose StatementEnd
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS(SELECT 1 FROM authorization_versions WHERE NOT authorization_entries_match(id)) THEN RAISE EXCEPTION '历史授权条目与正文不一致'; END IF;
END;
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION verify_authorization_entries() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v UUID;
BEGIN
    IF TG_TABLE_NAME='authorization_versions' THEN v:=NEW.id; ELSE v:=NEW.version_id; END IF;
    IF NOT authorization_entries_match(v) THEN RAISE EXCEPTION '授权条目与不可变正文不一致'; END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER authorization_body_entries_consistent AFTER INSERT ON authorization_versions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION verify_authorization_entries();
CREATE CONSTRAINT TRIGGER authorization_saved_entries_consistent AFTER INSERT ON authorization_scope_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION verify_authorization_entries();

-- 任意受理路径均需冻结回执并追加必需记录，不能只靠调用顺序保证原子边界。
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS(SELECT 1 FROM scope_commands c WHERE c.receipt='{}'::jsonb OR NOT EXISTS(SELECT 1 FROM scope_audit_events a WHERE a.namespace_id=c.namespace_id AND a.command_id=c.command_id) OR NOT EXISTS(SELECT 1 FROM scope_outbox_events o WHERE o.namespace_id=c.namespace_id AND o.command_id=c.command_id)) THEN RAISE EXCEPTION '历史受理记录不完整'; END IF;
END;
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION verify_scope_command_complete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS(SELECT 1 FROM scope_commands WHERE namespace_id=NEW.namespace_id AND command_id=NEW.command_id AND receipt='{}'::jsonb)
    OR NOT EXISTS(SELECT 1 FROM scope_audit_events WHERE namespace_id=NEW.namespace_id AND command_id=NEW.command_id)
    OR NOT EXISTS(SELECT 1 FROM scope_outbox_events WHERE namespace_id=NEW.namespace_id AND command_id=NEW.command_id) THEN RAISE EXCEPTION '受理、审计与 outbox 记录不完整'; END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- 仅可丢弃随机 schema 使用 Down；生产保留历史并优先前向修复。
-- +goose Down
DROP TRIGGER scope_namespace_consistent ON scope_commands;
DROP TRIGGER authorization_body_entries_consistent ON authorization_versions;
DROP TRIGGER authorization_saved_entries_consistent ON authorization_scope_entries;
DROP FUNCTION verify_scope_namespace(),verify_authorization_entries(),authorization_entries_match(UUID);
ALTER TABLE authorization_versions DROP CONSTRAINT authorization_content_contract;
ALTER TABLE authorization_scope_entries DROP CONSTRAINT authorization_entry_contract;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION verify_scope_command_complete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS(SELECT 1 FROM scope_commands WHERE namespace_id=NEW.namespace_id AND command_id=NEW.command_id AND receipt='{}'::jsonb) THEN RAISE EXCEPTION '受理回执尚未冻结'; END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
