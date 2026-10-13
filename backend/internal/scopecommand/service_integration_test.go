//go:build integration

package scopecommand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"mousehole/backend/db/migrations"
	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/identity"
	"mousehole/backend/internal/policy"
)

type fixture struct {
	pool           *pgxpool.Pool
	config         *pgxpool.Config
	actor, project string
	s              *Service
	legacy         command.Receipt
}

func database(t *testing.T) fixture {
	return databaseWithNamespace(t, false)
}

func databaseWithNamespace(t *testing.T, preinstalled bool) fixture {
	t.Helper()
	dsn := os.Getenv("MOUSEHOLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("必需真实 PostgreSQL 测试 DSN，不能跳过")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal("测试数据库配置无效")
	}
	if e = admin.Ping(ctx); e != nil {
		admin.Close()
		t.Fatal("真实测试数据库不可达")
	}
	schema := "scope_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal("配置无效")
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	config.MaxConns = 32
	pool, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, e := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error("清理本测试随机 schema 失败")
		}
		admin.Close()
	})
	db := stdlib.OpenDB(*config.ConnConfig)
	defer db.Close()
	provider, e := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = provider.UpTo(ctx, 2); e != nil {
		t.Fatal(e)
	}
	actor, project := uuid.NewString(), uuid.NewString()
	if _, e = pool.Exec(ctx, `INSERT INTO users(id,display_name) VALUES($1,'合成分析员');`, actor); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO projects(id,name) VALUES($1,'合成项目')`, project); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO project_memberships(project_id,user_id,role) VALUES($1,$2,'analyst')`, project, actor); e != nil {
		t.Fatal(e)
	}
	legacy, _, e := command.NewService(pool).Accept(ctx, actor, project, uuid.NewString(), command.Request{Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "升级前历史草稿"}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = provider.UpTo(ctx, 3); e != nil {
		t.Fatal(e)
	}
	if preinstalled {
		// 模拟早期开发库已安装的同一保护，前向迁移不能重复 CREATE 或移除它。
		_, e = pool.Exec(ctx, `CREATE FUNCTION verify_scope_namespace() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NOT EXISTS(SELECT 1 FROM scope_command_namespaces WHERE id=NEW.namespace_id AND project_id IS NOT DISTINCT FROM NEW.project_id) THEN RAISE EXCEPTION '命令命名空间与项目不一致'; END IF; RETURN NEW; END; $$; CREATE TRIGGER scope_namespace_consistent BEFORE INSERT ON scope_commands FOR EACH ROW EXECUTE FUNCTION verify_scope_namespace();`)
		if e != nil {
			t.Fatal(e)
		}
	}
	if _, e = provider.Up(ctx); e != nil {
		t.Fatal(e)
	}
	again, e := command.NewService(pool).Get(ctx, actor, project, legacy.CommandID)
	if e != nil || again != legacy {
		t.Fatal("00003 必须逐字段保留旧草稿、身份和回执")
	}
	return fixture{pool: pool, config: config, actor: actor, project: project, s: NewService(pool, []byte("固定合成游标密钥")), legacy: legacy}
}
func request(kind string, payload any) Request {
	data, _ := json.Marshal(payload)
	return Request{kind, data}
}
func importRequest() Request {
	return request("import_assets", asset.Import{Entries: []asset.Input{{Kind: "domain", Value: "EXAMPLE.INVALID."}, {Kind: "domain", Value: "example.invalid"}, {Kind: "ip", Value: "::ffff:10.42.0.9"}, {Kind: "url", Value: "HTTPS://example.invalid:443"}}, Source: "manual", Notes: "项目来源", ExternalRef: "工单"})
}
func candidate(t *testing.T, f fixture) authorization.Version {
	t.Helper()
	c := authorization.Content{Title: "非破坏性合成授权", BasisRef: "测试工单", Contact: "合成联系人", ValidFrom: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), AlgorithmVersion: "target-v1", PolicyVersion: "policy-v1", CatalogVersion: "precheck-v1", MaxOperationLevel: "non_destructive", Budget: authorization.Budget{MaxRequests: "100", MaxConcurrency: "2", MaxRequestsPerSecond: "5"}, RedirectPolicy: "deny", CredentialPolicy: "none", Entries: []authorization.Entry{{Kind: "cidr", Value: "10.42.0.0/16", Operations: []string{"tcp_connect"}, Ports: []int32{443}, ResolvedIPCIDRs: []string{}}}}
	r, _, e := f.s.Accept(context.Background(), f.actor, f.project, uuid.NewString(), request("create_authorization_version", c))
	if e != nil {
		t.Fatal(e)
	}
	var result VersionResult
	if e = json.Unmarshal(r.Result, &result); e != nil {
		t.Fatal(e)
	}
	return result.Version
}
func precheckRequest(version string) Request {
	port := 443
	return request("precheck_target", policy.Request{AuthorizationVersionID: version, Operation: "tcp_connect", Target: "10.42.0.9", Parameters: policy.Parameters{Port: &port}})
}
func decision(t *testing.T, r Receipt) policy.Decision {
	t.Helper()
	var result PrecheckResult
	if e := json.Unmarshal(r.Result, &result); e != nil {
		t.Fatal(e)
	}
	return result.Decision
}
func accept(t *testing.T, f fixture, r Request) Receipt {
	t.Helper()
	result, created, e := f.s.Accept(context.Background(), f.actor, f.project, uuid.NewString(), r)
	if e != nil || !created {
		t.Fatalf("受理失败：%v", e)
	}
	return result
}
func activate(t *testing.T, f fixture, v string, expected string) Receipt {
	t.Helper()
	r, created, e := f.s.AdminAccept(context.Background(), f.project, uuid.NewString(), "测试管理", AdminRequest{Kind: "activate_authorization_version", VersionID: v, ExpectedPolicyRevision: expected})
	if e != nil || !created {
		t.Fatalf("激活失败：%v", e)
	}
	return r
}
func tableCount(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if e := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestScopeImportAtomicReplayAndProjectIsolation(t *testing.T) {
	f := database(t)
	id := uuid.NewString()
	var wg sync.WaitGroup
	results := make(chan Receipt, 24)
	errs := make(chan error, 24)
	created := make(chan bool, 24)
	for i := 0; i < 24; i++ {
		wg.Go(func() {
			r, c, e := f.s.Accept(context.Background(), f.actor, f.project, id, importRequest())
			results <- r
			errs <- e
			created <- c
		})
	}
	wg.Wait()
	close(results)
	close(errs)
	close(created)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	won := 0
	for c := range created {
		if c {
			won++
		}
	}
	if won != 1 {
		t.Fatal("同键并发只能首次受理一次")
	}
	var original Receipt
	for r := range results {
		if original.CommandID == "" {
			original = r
		} else {
			a, _ := json.Marshal(original)
			b, _ := json.Marshal(r)
			if string(a) != string(b) {
				t.Fatal("重放必须原回执逐字段一致")
			}
		}
	}
	for table, want := range map[string]int{"assets": 3, "project_assets": 3, "asset_import_sources": 3, "scope_commands": 1, "scope_audit_events": 1, "scope_outbox_events": 1} {
		if n := tableCount(t, f.pool, table); n != want {
			t.Fatalf("%s 预期 %d 实际 %d", table, want, n)
		}
	}
	changed := importRequest()
	changed.Payload = json.RawMessage(strings.Replace(string(changed.Payload), "项目来源", "另一备注", 1))
	if _, _, e := f.s.Accept(context.Background(), f.actor, f.project, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatalf("异参数应冲突：%v", e)
	}
	second := uuid.NewString()
	if e := identity.ProvisionWithPool(context.Background(), f.pool, identity.ProvisionRequest{Issuer: "https://idp.example.invalid", Subject: "同一合成主体", UserID: f.actor, DisplayName: "合成分析员", ProjectID: second, ProjectName: "第二项目", Role: "analyst"}); e != nil {
		t.Fatal("新项目预置应同事务补齐策略/命名空间：", e)
	}
	empty, e := f.s.ListAssets(context.Background(), f.actor, second, ListOptions{})
	if e != nil || len(empty.Items) != 0 {
		t.Fatal("其他项目资产存在不得泄露")
	}
	r, _, e := f.s.Accept(context.Background(), f.actor, second, uuid.NewString(), importRequest())
	if e != nil {
		t.Fatal(e)
	}
	var result ImportResult
	_ = json.Unmarshal(r.Result, &result)
	if result.ExistingCount != "0" || len(result.Assets[0].Sources) != 1 {
		t.Fatal("重复全企业 canonical 不得泄露其他项目来源或已存在标志")
	}
	if tableCount(t, f.pool, "assets") != 3 || tableCount(t, f.pool, "project_assets") != 6 {
		t.Fatal("共享自然键与项目关联分开去重")
	}
	otherActor := uuid.NewString()
	if _, e = f.pool.Exec(context.Background(), `INSERT INTO users(id,display_name) VALUES($1,'另一个成员')`, otherActor); e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(context.Background(), `INSERT INTO project_memberships(project_id,user_id,role) VALUES($1,$2,'viewer')`, f.project, otherActor); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.Get(context.Background(), otherActor, f.project, id); !errors.Is(e, ErrNotFound) {
		t.Fatal("viewer 不得探测原 actor 命令")
	}
	if _, _, e = f.s.Accept(context.Background(), otherActor, f.project, uuid.NewString(), importRequest()); !errors.Is(e, ErrForbidden) {
		t.Fatal("viewer 不得写")
	}
	preview, e := f.s.PreviewAssets(context.Background(), f.actor, f.project, asset.Import{Entries: []asset.Input{{Kind: "domain", Value: "example.invalid"}, {Kind: "ip", Value: "0127.0.0.1"}}, Source: "manual"})
	if e != nil || preview.Valid || preview.Rows[1].Row != 2 || preview.Rows[1].Error == "" {
		t.Fatal("预览应准确列出错误行，且不写命令")
	}
	bad := request("import_assets", asset.Import{Entries: []asset.Input{{Kind: "domain", Value: "new.invalid"}, {Kind: "ip", Value: "invalid"}}, Source: "manual"})
	before := tableCount(t, f.pool, "scope_commands")
	if _, _, e = f.s.Accept(context.Background(), f.actor, f.project, uuid.NewString(), bad); !errors.Is(e, ErrInvalidRequest) || tableCount(t, f.pool, "scope_commands") != before {
		t.Fatal("无效行整批受理前拒绝")
	}
}

func TestAuthorizationLifecycleCurrentDeniesAndFrozenHistory(t *testing.T) {
	f := database(t)
	v := candidate(t, f)
	candidateDecision := decision(t, accept(t, f, precheckRequest(v.ID)))
	if candidateDecision.Outcome != "deny" {
		t.Fatal("候选默认拒绝")
	}
	activation := activate(t, f, v.ID, "1")
	if _, _, e := f.s.AdminAccept(context.Background(), f.project, activation.CommandID, "测试管理", AdminRequest{Kind: "activate_authorization_version", VersionID: v.ID, ExpectedPolicyRevision: "1"}); e != nil {
		t.Fatal("成功旧 revision 重放须先于 CAS")
	}
	checked := accept(t, f, precheckRequest(v.ID))
	d := decision(t, checked)
	if d.Outcome != "requires_runtime_checks" || d.ProjectRevision != "2" || d.MatchedEntryID == nil {
		t.Fatal("明确私网应静态匹配且保留检查上下文")
	}
	if _, e := f.pool.Exec(context.Background(), `UPDATE authorization_versions SET content=content||'{"title":"篡改"}' WHERE id=$1`, v.ID); e == nil {
		t.Fatal("正文不可编辑")
	}
	if _, e := f.pool.Exec(context.Background(), `DELETE FROM authorization_scope_entries WHERE version_id=$1`, v.ID); e == nil {
		t.Fatal("范围不可删除")
	}
	second := candidate(t, f)
	if _, _, e := f.s.AdminAccept(context.Background(), f.project, uuid.NewString(), "测试管理", AdminRequest{Kind: "activate_authorization_version", VersionID: second.ID, ExpectedPolicyRevision: "1"}); !errors.Is(e, ErrRevisionConflict) {
		t.Fatal("CAS 冲突不得覆盖当前版本")
	}
	activate(t, f, second.ID, "2")
	old, e := f.s.GetVersion(context.Background(), f.actor, f.project, v.ID)
	if e != nil || old.State != "superseded" {
		t.Fatal("替代应保留只读旧版本")
	}
	if decision(t, accept(t, f, precheckRequest(v.ID))).Outcome != "deny" {
		t.Fatal("旧固定版本不能继续静态通过")
	}
	deny := authorization.DenyInput{Kind: "cidr", Value: "10.42.0.0/16", Reason: "紧急测试禁止"}
	denyID := uuid.NewString()
	denyReceipt, _, e := f.s.AdminAccept(context.Background(), "", denyID, "测试管理", AdminRequest{Kind: "add_scope_deny", ExpectedPolicyRevision: "1", Deny: &deny})
	if e != nil {
		t.Fatal(e)
	}
	blocked := decision(t, accept(t, f, precheckRequest(second.ID)))
	if blocked.Outcome != "deny" || blocked.ReasonCodes[0] != "scope_denied" || blocked.GlobalRevision != "2" {
		t.Fatal("当前全局禁止必须叠加")
	}
	var change PolicyChangeResult
	_ = json.Unmarshal(denyReceipt.Result, &change)
	if _, _, e = f.s.AdminAccept(context.Background(), "", uuid.NewString(), "测试管理", AdminRequest{Kind: "revoke_scope_deny", DenyID: change.ObjectID, ExpectedPolicyRevision: "2"}); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.s.AdminAccept(context.Background(), f.project, uuid.NewString(), "测试管理", AdminRequest{Kind: "revoke_authorization_version", VersionID: second.ID, ExpectedPolicyRevision: "3"}); e != nil {
		t.Fatal(e)
	}
	current, e := f.s.GetPolicy(context.Background(), f.actor, f.project)
	if e != nil || current.ActiveVersionID != nil || current.ProjectRevision != "4" {
		t.Fatal("撤销不得自动回退旧版本")
	}
	// 响应丢失/连接池重建后仍通过原 ID 恢复原决策，不重新计算当前禁止或时间。
	restarted, e := pgxpool.NewWithConfig(context.Background(), f.config.Copy())
	if e != nil {
		t.Fatal(e)
	}
	defer restarted.Close()
	again, e := NewService(restarted, []byte("固定合成游标密钥")).Get(context.Background(), f.actor, f.project, checked.CommandID)
	if e != nil || string(again.Result) != string(checked.Result) || again.AcceptedAt != checked.AcceptedAt {
		t.Fatal("重建连接池必须恢复原冻结回执")
	}
	replay, created, e := f.s.Accept(context.Background(), f.actor, f.project, checked.CommandID, precheckRequest(v.ID))
	if e != nil || created || string(replay.Result) != string(checked.Result) {
		t.Fatal("撤销/替代之后原成功命令应重放历史结果")
	}
	history, e := f.s.GetPrecheck(context.Background(), f.actor, f.project, d.ID)
	if e != nil || history.CheckedAt != d.CheckedAt || history.ProjectRevision != "2" {
		t.Fatal("历史结果不得重写成当前许可")
	}
	foreign := uuid.NewString()
	if _, e = f.s.GetVersion(context.Background(), f.actor, f.project, foreign); !errors.Is(e, ErrNotFound) {
		t.Fatal("外部版本统一不存在")
	}
	if _, e = f.s.AdminGet(context.Background(), "", denyID, "其他引用"); !errors.Is(e, ErrNotFound) {
		t.Fatal("CLI 查询必须核对操作引用")
	}
	if _, e = f.s.AdminGet(context.Background(), "", denyID, "  测试管理  "); e != nil {
		t.Fatal("CLI 原操作引用的首尾空格应与受理采用同一规范化")
	}
	var actors int
	if e = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM scope_commands WHERE operator_source='admin_cli' AND actor_id IS NOT NULL`).Scan(&actors); e != nil || actors != 0 {
		t.Fatal("CLI 不能伪造 OIDC actor")
	}
}

// 三种 Web kind 任一必需事件写失败均整体回滚，只比较本次事务增量。
func TestRequiredAuditOutboxRollback(t *testing.T) {
	for _, kind := range []string{"import_assets", "create_authorization_version", "precheck_target"} {
		for _, table := range []string{"scope_audit_events", "scope_outbox_events"} {
			t.Run(kind+"/"+table, func(t *testing.T) {
				f := database(t)
				r := importRequest()
				if kind != "import_assets" {
					v := candidate(t, f)
					if kind == "precheck_target" {
						r = precheckRequest(v.ID)
					} else {
						r = request(kind, v.Content)
					}
				}
				tables := []string{"assets", "project_assets", "asset_import_sources", "authorization_versions", "authorization_scope_entries", "authorization_version_states", "policy_prechecks", "scope_commands", "scope_audit_events", "scope_outbox_events"}
				before := map[string]int{}
				for _, name := range tables {
					before[name] = tableCount(t, f.pool, name)
				}
				sql := fmt.Sprintf(`CREATE FUNCTION fail_required() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '合成必需写入故障'; END; $$; CREATE TRIGGER fail_required BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION fail_required();`, pgx.Identifier{table}.Sanitize())
				if _, e := f.pool.Exec(context.Background(), sql); e != nil {
					t.Fatal(e)
				}
				if _, _, e := f.s.Accept(context.Background(), f.actor, f.project, uuid.NewString(), r); !errors.Is(e, ErrUnavailable) {
					t.Fatalf("必需写入失败应无法确认：%v", e)
				}
				for _, name := range tables {
					if tableCount(t, f.pool, name) != before[name] {
						t.Fatalf("%s 本次事务必须整体回滚", name)
					}
				}
			})
		}
	}
}

func TestStableSeekPaginationAndNamespaceConstraints(t *testing.T) {
	f := database(t)
	entries := []asset.Input{}
	for i := 0; i < 31; i++ {
		entries = append(entries, asset.Input{Kind: "domain", Value: fmt.Sprintf("host%d.example.invalid", i)})
	}
	accept(t, f, request("import_assets", asset.Import{Entries: entries, Source: "manual"}))
	seen := map[string]bool{}
	cursor := ""
	for {
		page, e := f.s.ListAssets(context.Background(), f.actor, f.project, ListOptions{PageSize: 7, Cursor: cursor})
		if e != nil {
			t.Fatal(e)
		}
		for _, a := range page.Items {
			if seen[a.ID] {
				t.Fatal("seek 不能重复资产")
			}
			seen[a.ID] = true
		}
		if !page.HasMore {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 31 {
		t.Fatal("大于一页的列表必须完整可翻查")
	}
	var namespace string
	if e := f.pool.QueryRow(context.Background(), `SELECT id FROM scope_command_namespaces WHERE project_id=$1`, f.project).Scan(&namespace); e != nil {
		t.Fatal(e)
	}
	_, e := f.pool.Exec(context.Background(), `INSERT INTO scope_commands(namespace_id,command_id,operator_source,operator_ref,kind,payload,payload_hash,accepted_at) VALUES($1,$2,'admin_cli','合成操作','add_scope_deny','{}',repeat('0',64),clock_timestamp())`, namespace, uuid.NewString())
	if e == nil {
		t.Fatal("NULL project 全局命令不能挂项目 namespace")
	}
}
