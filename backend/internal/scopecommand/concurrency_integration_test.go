//go:build integration

package scopecommand

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/identity"
)

type concurrentResult struct {
	receipt Receipt
	err     error
}

func runAsync(fn func() (Receipt, error)) <-chan concurrentResult {
	done := make(chan concurrentResult, 1)
	go func() { r, e := fn(); done <- concurrentResult{r, e} }()
	return done
}

func joined(t *testing.T, done <-chan concurrentResult) concurrentResult {
	t.Helper()
	select {
	case r := <-done:
		return r
	case <-time.After(15 * time.Second):
		t.Fatal("受控并发操作未在期限内完成")
		return concurrentResult{}
	}
}

// advisoryGate 在真实事务已持权限/策略锁之后暂停，不以 sleep 推断提交顺序。
type advisoryGate struct {
	conn *pgxpool.Conn
	key  int64
	pool *pgxpool.Pool
	ctx  context.Context
}

func newGate(t *testing.T, f fixture, ctx context.Context) *advisoryGate {
	t.Helper()
	id := uuid.New()
	g := &advisoryGate{key: int64(binary.BigEndian.Uint32(id[:4]) & 0x7fffffff), pool: f.pool, ctx: ctx}
	var e error
	g.conn, e = f.pool.Acquire(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = g.conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, g.key); e != nil {
		g.conn.Release()
		t.Fatal(e)
	}
	t.Cleanup(func() {
		// 失败路径也解除本测试拥有的会话锁，避免连接池关闭等待泄漏协程。
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = g.conn.Exec(cleanup, `SELECT pg_advisory_unlock($1)`, g.key)
		g.conn.Release()
	})
	return g
}

func (g *advisoryGate) install(t *testing.T, table, condition string) {
	t.Helper()
	// 表名来自固定测试枚举，条件只拼接本测试生成的标准 UUID。
	sql := fmt.Sprintf(`CREATE FUNCTION test_command_gate() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF %s THEN PERFORM pg_advisory_xact_lock(%d); END IF; RETURN NEW; END; $$; CREATE TRIGGER test_command_gate BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION test_command_gate();`, condition, g.key, pgx.Identifier{table}.Sanitize())
	if _, e := g.pool.Exec(g.ctx, sql); e != nil {
		t.Fatal(e)
	}
}

func (g *advisoryGate) pausedPID(t *testing.T) int32 {
	t.Helper()
	var pid int32
	for {
		e := g.pool.QueryRow(g.ctx, `SELECT pid FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objid=$1::oid LIMIT 1`, g.key).Scan(&pid)
		if e == nil {
			return pid
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			t.Fatal(e)
		}
		select {
		case <-g.ctx.Done():
			t.Fatal("事务未到达本测试 advisory gate")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (g *advisoryGate) waitBlocked(t *testing.T, blocker int32) {
	t.Helper()
	for {
		var waiting bool
		if e := g.pool.QueryRow(g.ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid)) AND wait_event_type='Lock')`, blocker).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			return
		}
		select {
		case <-g.ctx.Done():
			t.Fatal("竞争事务未等待已持有的真实行锁")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (g *advisoryGate) release(t *testing.T) {
	t.Helper()
	if _, e := g.conn.Exec(g.ctx, `SELECT pg_advisory_unlock($1)`, g.key); e != nil {
		t.Fatal(e)
	}
}

// 用户、项目、成员失效均与新旧命令比较两种提交顺序；撤销走真实管理服务。
func TestPermissionChangesSerializeOldAndScopeCommands(t *testing.T) {
	for _, change := range []string{"user", "project", "membership"} {
		for _, old := range []bool{false, true} {
			for _, changeFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/old=%v/change_first=%v", change, old, changeFirst), func(t *testing.T) {
					f := database(t)
					ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
					defer cancel()
					g := newGate(t, f, ctx)
					id := uuid.NewString()
					legacy := command.NewService(f.pool)
					issue := func() (Receipt, error) {
						if old {
							_, _, e := legacy.Accept(ctx, f.actor, f.project, id, command.Request{Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "并发权限测试"}})
							return Receipt{}, e
						}
						r, _, e := f.s.Accept(ctx, f.actor, f.project, id, importRequest())
						return r, e
					}
					mutate := func() (Receipt, error) {
						switch change {
						case "user":
							_, e := identity.DisableUserWithPool(ctx, f.pool, f.actor)
							return Receipt{}, e
						case "membership":
							_, e := identity.RevokeMembershipWithPool(ctx, f.pool, f.actor, f.project)
							return Receipt{}, e
						default:
							// 目前没有项目禁用 CLI；夹具显式更新同一权威行以验证未来管理入口锁合同。
							tx, e := f.pool.Begin(ctx)
							if e != nil {
								return Receipt{}, e
							}
							defer rollback(tx)
							if _, e = tx.Exec(ctx, `UPDATE projects SET status='disabled' WHERE id=$1`, f.project); e != nil {
								return Receipt{}, e
							}
							if changeFirst {
								if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, g.key); e != nil {
									return Receipt{}, e
								}
							}
							return Receipt{}, tx.Commit(ctx)
						}
					}
					var commandDone, mutationDone <-chan concurrentResult
					if changeFirst {
						if change != "project" {
							object := f.actor
							if change == "membership" {
								object = f.project + ":" + f.actor
							}
							g.install(t, "access_audit_events", fmt.Sprintf("NEW.object_id='%s'", object))
						}
						mutationDone = runAsync(mutate)
						pid := g.pausedPID(t)
						commandDone = runAsync(issue)
						g.waitBlocked(t, pid)
					} else {
						table := "scope_commands"
						if old {
							table = "commands"
						}
						g.install(t, table, fmt.Sprintf("NEW.command_id='%s'::uuid", id))
						commandDone = runAsync(issue)
						pid := g.pausedPID(t)
						mutationDone = runAsync(mutate)
						g.waitBlocked(t, pid)
					}
					g.release(t)
					c, m := joined(t, commandDone), joined(t, mutationDone)
					if m.err != nil {
						t.Fatalf("权限变更失败：%v", m.err)
					}
					if changeFirst {
						want := ErrForbidden
						if old {
							want = command.ErrForbidden
						}
						if !errors.Is(c.err, want) {
							t.Fatalf("失效先提交应拒绝命令：%v", c.err)
						}
					} else if c.err != nil {
						t.Fatalf("命令先提交应完整成功：%v", c.err)
					}
					want := 0
					if !changeFirst {
						want = 1
					}
					for _, table := range []string{"scope_commands", "scope_audit_events", "scope_outbox_events"} {
						count := tableCount(t, f.pool, table)
						if old {
							if count != 0 {
								t.Fatal("旧命令不得写新领域账本")
							}
						} else if count != want {
							t.Fatalf("%s 未保持完整原子记录", table)
						}
					}
					for _, table := range []string{"commands", "tasks", "audit_events", "outbox_events"} {
						count := tableCount(t, f.pool, table)
						expected := 1
						if old {
							expected += want
						}
						if count != expected {
							t.Fatalf("%s 旧历史或并发原子记录不一致", table)
						}
					}
					if _, e := f.s.Get(ctx, f.actor, f.project, id); !errors.Is(e, ErrForbidden) {
						t.Fatal("失效后新领域历史查询仍应拒绝")
					}
					if _, e := legacy.Get(ctx, f.actor, f.project, id); !errors.Is(e, command.ErrForbidden) {
						t.Fatal("失效后旧领域历史查询仍应拒绝")
					}
				})
			}
		}
	}
}

// 空禁止表也通过稳定 policy 行串行化，先检查的历史 revision 永不被覆写。
func TestPolicyMutationsSerializeWithPrechecks(t *testing.T) {
	for _, change := range []string{"activate", "revoke", "project_deny", "global_deny"} {
		for _, changeFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/change_first=%v", change, changeFirst), func(t *testing.T) {
				f := database(t)
				v := candidate(t, f)
				before := "1"
				beforeOutcome := "deny"
				if change != "activate" {
					activate(t, f, v.ID, "1")
					before = "2"
					beforeOutcome = "requires_runtime_checks"
				}
				if tableCount(t, f.pool, "scope_denies") != 0 {
					t.Fatal("测试必须从空 deny 表开始")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				g := newGate(t, f, ctx)
				adminID, checkID := uuid.NewString(), uuid.NewString()
				project := f.project
				mutation := AdminRequest{ExpectedPolicyRevision: before, VersionID: v.ID, Kind: "activate_authorization_version"}
				switch change {
				case "revoke":
					mutation.Kind = "revoke_authorization_version"
				case "project_deny", "global_deny":
					mutation.Kind = "add_scope_deny"
					mutation.VersionID = ""
					mutation.Deny = &authorization.DenyInput{Kind: "cidr", Value: "10.42.0.0/16", Reason: "受控并发禁止"}
					if change == "global_deny" {
						project = ""
						mutation.ExpectedPolicyRevision = "1"
					}
				}
				mutate := func() (Receipt, error) {
					r, _, e := f.s.AdminAccept(ctx, project, adminID, "并发管理", mutation)
					return r, e
				}
				check := func() (Receipt, error) {
					r, _, e := f.s.Accept(ctx, f.actor, f.project, checkID, precheckRequest(v.ID))
					return r, e
				}
				firstID := checkID
				if changeFirst {
					firstID = adminID
				}
				g.install(t, "scope_commands", fmt.Sprintf("NEW.command_id='%s'::uuid", firstID))
				var checks, mutations <-chan concurrentResult
				if changeFirst {
					mutations = runAsync(mutate)
					pid := g.pausedPID(t)
					checks = runAsync(check)
					g.waitBlocked(t, pid)
				} else {
					checks = runAsync(check)
					pid := g.pausedPID(t)
					mutations = runAsync(mutate)
					g.waitBlocked(t, pid)
				}
				g.release(t)
				c, m := joined(t, checks), joined(t, mutations)
				if c.err != nil || m.err != nil {
					t.Fatalf("并发预检查/管理失败：%v / %v", c.err, m.err)
				}
				d := decision(t, c.receipt)
				wantOutcome, wantProject, wantGlobal := beforeOutcome, before, "1"
				if changeFirst {
					if change == "activate" {
						wantOutcome = "requires_runtime_checks"
					} else {
						wantOutcome = "deny"
					}
					if change == "global_deny" {
						wantGlobal = "2"
					} else if before == "1" {
						wantProject = "2"
					} else {
						wantProject = "3"
					}
				}
				if d.Outcome != wantOutcome || d.ProjectRevision != wantProject || d.GlobalRevision != wantGlobal {
					t.Fatalf("先提交/历史 revision 语义错误：%s %s/%s", d.Outcome, d.GlobalRevision, d.ProjectRevision)
				}
				history, e := f.s.GetPrecheck(ctx, f.actor, f.project, d.ID)
				if e != nil || history.CheckedAt != d.CheckedAt || history.ProjectRevision != d.ProjectRevision || history.Outcome != d.Outcome {
					t.Fatal("后续管理不得覆写历史决策")
				}
				fresh := decision(t, accept(t, f, precheckRequest(v.ID)))
				if change == "activate" && fresh.Outcome != "requires_runtime_checks" || change != "activate" && fresh.Outcome != "deny" {
					t.Fatal("随后新检查必须读取最新规则")
				}
			})
		}
	}
}
