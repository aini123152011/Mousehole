//go:build integration

package command_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"mousehole/backend/db/migrations"
	"mousehole/backend/internal/command"
)

type databaseFixture struct {
	pool   *pgxpool.Pool
	config *pgxpool.Config
	schema string
}

func testDatabase(t *testing.T) databaseFixture {
	t.Helper()
	databaseURL := os.Getenv("MOUSEHOLE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("集成测试必须设置 MOUSEHOLE_TEST_DATABASE_URL；不能跳过真实 PostgreSQL 验证")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("测试数据库配置无效")
	}
	if err = admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatal("无法连接真实 PostgreSQL 测试数据库")
	}
	schema := "mousehole_test_" + uuid.NewString()
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatalf("创建隔离 schema 失败：%v", err)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("测试数据库配置无效")
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.MaxConns = 32
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("创建隔离连接池失败")
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		// 只删除本测试刚创建的随机 schema，不修改用户已有 schema 或数据库。
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("清理本次隔离 schema 失败：%v", err)
		}
		admin.Close()
	})
	db := stdlib.OpenDB(*config.ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatalf("迁移提供者失败：%v", err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatalf("编号迁移失败：%v", err)
	}
	if results, err := provider.Up(ctx); err != nil || len(results) != 0 {
		t.Fatalf("已应用编号迁移必须不重复执行，结果=%v，错误=%v", results, err)
	}
	return databaseFixture{pool: pool, config: config, schema: schema}
}

func assertCounts(t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	for _, table := range []string{"commands", "tasks", "audit_events", "outbox_events"} {
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&count); err != nil {
			t.Fatalf("读取 %s 失败：%v", table, err)
		}
		if count != want {
			t.Errorf("%s 记录数预期=%d，实际=%d", table, want, count)
		}
	}
}

func TestAcceptConcurrentIdempotencyAndConflict(t *testing.T) {
	// 准备：真实 PG 隔离 schema；所有参与者同一 ID，不用进程 mutex 参与受理。
	fixture := testDatabase(t)
	service := command.NewService(fixture.pool)
	id := uuid.NewString()
	request := command.Request{Kind: command.CreateTaskDraft, Payload: command.Payload{Title: " 并发中文草稿 "}}
	const participants = 24
	start := make(chan struct{})
	results := make(chan struct {
		receipt command.Receipt
		created bool
		err     error
	}, participants)
	var workers sync.WaitGroup
	for range participants {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			receipt, created, err := service.Accept(ctx, command.DevelopmentActorID, command.DevelopmentProjectID, id, request)
			results <- struct {
				receipt command.Receipt
				created bool
				err     error
			}{receipt, created, err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	var original command.Receipt
	createdCount := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("竞争受理失败：%v", result.err)
		}
		if result.created {
			createdCount++
		}
		if original.CommandID == "" {
			original = result.receipt
		} else if original != result.receipt {
			t.Fatalf("回放未返回同一原始回执：%+v / %+v", original, result.receipt)
		}
	}
	if createdCount != 1 {
		t.Fatalf("首次受理预期=1，实际=%d", createdCount)
	}
	if original.TaskTitle != "并发中文草稿" || original.State != "accepted" || original.TaskState != "draft" || original.Version != "1" {
		t.Fatalf("原回执不满足合同：%+v", original)
	}
	if parsed, err := time.Parse(time.RFC3339Nano, original.AcceptedAt); err != nil || parsed.Location() != time.UTC {
		t.Fatalf("受理时间未使用UTC RFC3339：%s", original.AcceptedAt)
	}
	assertCounts(t, fixture.pool, 1)
	// 规范化后相同的标题必须回放原记录，不把首尾空白当作新参数。
	request.Payload.Title = "并发中文草稿"
	if replayed, created, err := service.Accept(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id, request); err != nil || created || replayed != original {
		t.Fatalf("规范化参数未回放原回执，created=%v，错误=%v，回执=%+v", created, err, replayed)
	}

	// 不同规范化参数必须冲突，不能改变原草稿或新增审计/outbox。
	request.Payload.Title = "不同中文标题"
	if _, _, err := service.Accept(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id, request); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("预期幂等冲突，实际=%v", err)
	}
	loaded, err := service.Get(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id)
	if err != nil || loaded != original {
		t.Fatalf("冲突修改了原始回执：%+v，错误=%v", loaded, err)
	}
	assertCounts(t, fixture.pool, 1)
	var linkedCount int
	err = fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM commands c JOIN tasks t USING (project_id,command_id) JOIN audit_events a USING (project_id,command_id) JOIN outbox_events o USING (project_id,command_id) WHERE c.task_id=t.task_id AND c.task_id=a.task_id AND c.task_id=o.task_id AND c.actor_id=a.actor_id AND c.task_title=t.title`).Scan(&linkedCount)
	if err != nil || linkedCount != 1 {
		t.Fatalf("四类记录关联不一致：count=%d，错误=%v", linkedCount, err)
	}
}

func TestAcceptCommitFailureRollsBackAllRecords(t *testing.T) {
	fixture := testDatabase(t)
	// 延迟约束触发器在四类记录都写入后、提交时失败，验证最晚失败仍整体回滚。
	_, err := fixture.pool.Exec(context.Background(), `
CREATE FUNCTION reject_outbox_commit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '合成提交故障' USING ERRCODE = '23514'; END;
$$;
CREATE CONSTRAINT TRIGGER reject_outbox_commit AFTER INSERT ON outbox_events
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_outbox_commit();`)
	if err != nil {
		t.Fatalf("安装隔离故障注入失败：%v", err)
	}
	service := command.NewService(fixture.pool)
	id := uuid.NewString()
	_, _, err = service.Accept(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id, command.Request{Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "提交回滚草稿"}})
	if !errors.Is(err, command.ErrUnavailable) {
		t.Fatalf("提交失败应保留待确认语义，实际=%v", err)
	}
	assertCounts(t, fixture.pool, 0)
	if _, err = service.Get(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("失败提交不应留下回执，实际=%v", err)
	}
	if _, err = fixture.pool.Exec(context.Background(), "DROP TRIGGER reject_outbox_commit ON outbox_events"); err != nil {
		t.Fatal(err)
	}
	if _, created, err := service.Accept(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id, command.Request{Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "提交回滚草稿"}}); err != nil || !created {
		t.Fatalf("原 ID 在确认回滚后应可人工提交，created=%v，错误=%v", created, err)
	}
	assertCounts(t, fixture.pool, 1)
}

func TestReceiptSurvivesConnectionPoolReload(t *testing.T) {
	fixture := testDatabase(t)
	id := uuid.NewString()
	before, _, err := command.NewService(fixture.pool).Accept(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id, command.Request{Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "重启恢复草稿"}})
	if err != nil {
		t.Fatal(err)
	}
	fixture.pool.Close()
	// 完全关闭原服务连接池，新的服务只能从持久化数据库恢复，无进程内缓存。
	reloaded, err := pgxpool.NewWithConfig(context.Background(), fixture.config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	after, err := command.NewService(reloaded).Get(context.Background(), command.DevelopmentActorID, command.DevelopmentProjectID, id)
	if err != nil || before != after {
		t.Fatalf("重新连接后原回执无法恢复：前=%+v，后=%+v，错误=%v", before, after, err)
	}
	assertCounts(t, reloaded, 1)
	if _, err = command.NewService(reloaded).Get(context.Background(), command.DevelopmentActorID, uuid.NewString(), id); !errors.Is(err, command.ErrForbidden) {
		t.Fatalf("重新查询必须重验项目，实际=%v", err)
	}
}
