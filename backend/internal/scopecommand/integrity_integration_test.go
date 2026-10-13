//go:build integration

package scopecommand

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"mousehole/backend/internal/identity"
)

// 固定 command ID 同时保存或预检查只产生一个版本/决策与一套必需账本。
func TestCandidateAndPrecheckSameCommandConcurrency(t *testing.T) {
	for _, precheck := range []bool{false, true} {
		t.Run(map[bool]string{false: "candidate", true: "precheck"}[precheck], func(t *testing.T) {
			f := database(t)
			v := candidate(t, f)
			var r Request
			if precheck {
				activate(t, f, v.ID, "1")
				r = precheckRequest(v.ID)
			} else {
				r = request("create_authorization_version", v.Content)
			}
			before := tableCount(t, f.pool, "scope_commands")
			id := uuid.NewString()
			results := make(chan concurrentResult, 12)
			creates := make(chan bool, 12)
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Go(func() {
					receipt, c, e := f.s.Accept(context.Background(), f.actor, f.project, id, r)
					results <- concurrentResult{receipt, e}
					creates <- c
				})
			}
			wg.Wait()
			close(results)
			close(creates)
			won := 0
			for c := range creates {
				if c {
					won++
				}
			}
			if won != 1 {
				t.Fatal("同 ID 只能第一次受理")
			}
			var original Receipt
			for result := range results {
				if result.err != nil {
					t.Fatal(result.err)
				}
				if original.CommandID == "" {
					original = result.receipt
				} else if original.AcceptedAt != result.receipt.AcceptedAt || string(original.Result) != string(result.receipt.Result) {
					t.Fatal("并发重放应返回同一冻结对象和时间")
				}
			}
			for _, table := range []string{"scope_commands", "scope_audit_events", "scope_outbox_events"} {
				if tableCount(t, f.pool, table) != before+1 {
					t.Fatalf("%s 只能追加一份", table)
				}
			}
			if precheck {
				if tableCount(t, f.pool, "policy_prechecks") != 1 {
					t.Fatal("预检查不能重复执行并记录")
				}
			} else if tableCount(t, f.pool, "authorization_versions") != 2 {
				t.Fatal("候选重放不能创建额外版本")
			}
		})
	}
}

func expectDatabaseError(t *testing.T, e error, code string) {
	t.Helper()
	var databaseError *pgconn.PgError
	if !errors.As(e, &databaseError) || databaseError.Code != code {
		t.Fatalf("数据库应由 %s 约束拒绝，实际=%v", code, e)
	}
}

// 应用遗漏校验也不能跨项目绑定或追加已保存范围；使用实际 FK/延迟约束反例。
func TestDatabaseHistoryAndProjectIntegrity(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	v := candidate(t, f)
	project := uuid.NewString()
	if e := identity.ProvisionWithPool(ctx, f.pool, identity.ProvisionRequest{Issuer: "https://idp.example.invalid", Subject: "完整性测试", UserID: f.actor, DisplayName: "合成分析员", ProjectID: project, ProjectName: "隔离项目", Role: "analyst"}); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.GetVersion(ctx, f.actor, project, v.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("服务端不得读取跨项目版本")
	}
	if _, _, e := f.s.Accept(ctx, f.actor, project, uuid.NewString(), precheckRequest(v.ID)); !errors.Is(e, ErrNotFound) {
		t.Fatal("服务端不得预检查外项目版本")
	}
	_, e := f.pool.Exec(ctx, `INSERT INTO authorization_scope_entries(id,project_id,version_id,ordinal,kind,value,include_subdomains,operations,ports,resolved_ip_cidrs) SELECT $1,$2,version_id,50,kind,value,include_subdomains,operations,ports,resolved_ip_cidrs FROM authorization_scope_entries WHERE version_id=$3 LIMIT 1`, uuid.NewString(), project, v.ID)
	expectDatabaseError(t, e, "23503")
	_, e = f.pool.Exec(ctx, `INSERT INTO authorization_scope_entries(id,project_id,version_id,ordinal,kind,value,include_subdomains,operations,ports,resolved_ip_cidrs) SELECT $1,project_id,version_id,50,kind,value,include_subdomains,operations,ports,resolved_ip_cidrs FROM authorization_scope_entries WHERE version_id=$2 LIMIT 1`, uuid.NewString(), v.ID)
	expectDatabaseError(t, e, "P0001")
	if tableCount(t, f.pool, "authorization_scope_entries") != 1 {
		t.Fatal("未匹配正文的新条目必须回滚")
	}
	_, e = f.pool.Exec(ctx, `INSERT INTO authorization_versions(id,project_id,sequence,content,content_hash,valid_from,expires_at,created_at) SELECT $1,project_id,50,jsonb_set(content,'{budget,max_requests}','"0"'),content_hash,valid_from,expires_at,created_at FROM authorization_versions WHERE id=$2`, uuid.NewString(), v.ID)
	expectDatabaseError(t, e, "23514")
	for _, tc := range []struct{ path, value string }{{"max_requests", "9223372036854775808"}, {"max_requests_per_second", "9999999999999999999"}, {"max_concurrency", "101"}} {
		_, e = f.pool.Exec(ctx, `INSERT INTO authorization_versions(id,project_id,sequence,content,content_hash,valid_from,expires_at,created_at) SELECT $1,project_id,50,jsonb_set(content,ARRAY['budget',$2],to_jsonb($3::text)),content_hash,valid_from,expires_at,created_at FROM authorization_versions WHERE id=$4`, uuid.NewString(), tc.path, tc.value, v.ID)
		expectDatabaseError(t, e, "23514")
	}
	var namespace string
	if e = f.pool.QueryRow(ctx, `SELECT id FROM scope_command_namespaces WHERE project_id=$1`, f.project).Scan(&namespace); e != nil {
		t.Fatal(e)
	}
	_, e = f.pool.Exec(ctx, `INSERT INTO scope_commands(namespace_id,command_id,project_id,actor_id,operator_source,operator_ref,kind,payload,payload_hash,accepted_at,receipt) VALUES($1,$2,$3,$4,'web','','import_assets','{}',repeat('0',64),clock_timestamp(),'{"state":"accepted"}')`, namespace, uuid.NewString(), f.project, f.actor)
	expectDatabaseError(t, e, "P0001")
	// 另一个合法命令也不能作为跨项目决策/entry 的归属载体。
	checked := accept(t, f, precheckRequest(v.ID))
	d := decision(t, checked)
	_, e = f.pool.Exec(ctx, `INSERT INTO policy_prechecks(id,project_id,version_id,matched_entry_id,namespace_id,command_id,checked_at,decision) SELECT $1,$2,version_id,matched_entry_id,namespace_id,$3,checked_at,decision FROM policy_prechecks WHERE id=$4`, uuid.NewString(), project, uuid.NewString(), d.ID)
	expectDatabaseError(t, e, "23503")
	if _, e = f.s.GetPrecheck(ctx, f.actor, project, d.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("服务端不得读取跨项目决策")
	}
	if _, e = f.s.Get(ctx, f.actor, project, checked.CommandID); !errors.Is(e, ErrNotFound) {
		t.Fatal("另一 namespace 不得查询本项目回执")
	}
}

func TestActivationRejectsFutureAndExpiredWindows(t *testing.T) {
	f := database(t)
	template := candidate(t, f).Content
	for _, future := range []bool{false, true} {
		c := template
		if future {
			c.ValidFrom = time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
			c.ExpiresAt = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano)
		} else {
			c.ValidFrom = time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
			c.ExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
		}
		r := accept(t, f, request("create_authorization_version", c))
		var version VersionResult
		if e := json.Unmarshal(r.Result, &version); e != nil {
			t.Fatal(e)
		}
		before := tableCount(t, f.pool, "scope_commands")
		if _, _, e := f.s.AdminAccept(context.Background(), f.project, uuid.NewString(), "时窗测试", AdminRequest{Kind: "activate_authorization_version", VersionID: version.Version.ID, ExpectedPolicyRevision: "1"}); !errors.Is(e, ErrInvalidRequest) {
			t.Fatalf("无效时窗激活须拒绝：%v", e)
		}
		if tableCount(t, f.pool, "scope_commands") != before {
			t.Fatal("激活拒绝不得留下命令占位")
		}
		state, e := f.s.GetPolicy(context.Background(), f.actor, f.project)
		if e != nil || state.ProjectRevision != "1" || state.ActiveVersionID != nil {
			t.Fatal("拒绝激活不得改变当前策略")
		}
	}
}

// 同项目另一个 analyst 也不能通过原 ID 查询或不同参数 PUT 探测原 principal。
func TestAnalystCommandOwnershipHidesAllKinds(t *testing.T) {
	for _, kind := range []string{"import_assets", "create_authorization_version", "precheck_target"} {
		t.Run(kind, func(t *testing.T) {
			f := database(t)
			ctx := context.Background()
			r := importRequest()
			if kind != "import_assets" {
				v := candidate(t, f)
				if kind == "precheck_target" {
					r = precheckRequest(v.ID)
				} else {
					r = request(kind, v.Content)
				}
			}
			original := accept(t, f, r)
			other := uuid.NewString()
			if _, e := f.pool.Exec(ctx, `INSERT INTO users(id,display_name) VALUES($1,'另一合成分析员')`, other); e != nil {
				t.Fatal(e)
			}
			if _, e := f.pool.Exec(ctx, `INSERT INTO project_memberships(project_id,user_id,role) VALUES($1,$2,'analyst')`, f.project, other); e != nil {
				t.Fatal(e)
			}
			before := map[string]int{}
			for _, table := range []string{"assets", "project_assets", "asset_import_sources", "authorization_versions", "authorization_scope_entries", "policy_prechecks", "scope_commands", "scope_audit_events", "scope_outbox_events"} {
				before[table] = tableCount(t, f.pool, table)
			}
			if receipt, e := f.s.Get(ctx, other, f.project, original.CommandID); !errors.Is(e, ErrNotFound) || receipt.CommandID != "" || receipt.Result != nil {
				t.Fatal("其他 analyst GET 必须无回执统一不存在")
			}
			changed := r
			var payload map[string]any
			if e := json.Unmarshal(r.Payload, &payload); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "import_assets":
				payload["notes"] = "不同声明"
			case "create_authorization_version":
				payload["title"] = "不同候选"
			default:
				payload["target"] = "10.42.0.10"
			}
			changed.Payload, _ = json.Marshal(payload)
			for _, proposed := range []Request{r, changed} {
				receipt, created, e := f.s.Accept(ctx, other, f.project, original.CommandID, proposed)
				if !errors.Is(e, ErrNotFound) || created || receipt.CommandID != "" || receipt.Result != nil {
					t.Fatal("其他 principal PUT 同/异参数必须统一不存在，无原参数/结果")
				}
			}
			for table, count := range before {
				if tableCount(t, f.pool, table) != count {
					t.Fatalf("%s 隐藏原命令时不可改变任何记录", table)
				}
			}
		})
	}
}

func TestDistinctImportCommandsConcurrentNaturalKey(t *testing.T) {
	f := database(t)
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			_, created, e := f.s.Accept(context.Background(), f.actor, f.project, uuid.NewString(), importRequest())
			if e == nil && !created {
				e = errors.New("各自命令应首次受理")
			}
			results <- e
		})
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	for table, count := range map[string]int{"assets": 3, "project_assets": 3, "asset_import_sources": 36, "scope_commands": 12, "scope_audit_events": 12, "scope_outbox_events": 12} {
		if tableCount(t, f.pool, table) != count {
			t.Fatalf("%s 同资产多命令并发去重/来源错误", table)
		}
	}
}

func TestForwardUpgradeWithExistingNamespaceProtection(t *testing.T) {
	f := databaseWithNamespace(t, true)
	accept(t, f, importRequest())
	var valid bool
	if e := f.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='scope_commands'::regclass AND tgname='scope_namespace_consistent' AND tgtype=7 AND tgenabled='O')`).Scan(&valid); e != nil || !valid {
		t.Fatal("00004应原子保留已有namespace保护并补齐约束")
	}
}

func TestScopeCommandKindConflict(t *testing.T) {
	f := database(t)
	v := candidate(t, f)
	original := accept(t, f, importRequest())
	before := tableCount(t, f.pool, "scope_commands")
	if _, _, e := f.s.Accept(context.Background(), f.actor, f.project, original.CommandID, request("create_authorization_version", v.Content)); !errors.Is(e, ErrConflict) {
		t.Fatal("同principal同ID改kind应冲突")
	}
	if tableCount(t, f.pool, "scope_commands") != before || tableCount(t, f.pool, "authorization_versions") != 1 {
		t.Fatal("跨kind冲突不得改变原账本或写业务")
	}
}
