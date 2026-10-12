package command

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mousehole/backend/internal/store/sqlc"
)

// Service 用数据库事务保证命令原子性；可并发复用，不持有进程级幂等缓存。
type Service struct{ pool *pgxpool.Pool }

// NewService 创建命令服务；连接池生命周期由入口负责关闭。
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Accept 受理草稿命令，created 区分首次受理与原回执重放。
//
// 同项目和命令 ID 由 PostgreSQL 唯一键串行化；参数不同返回冲突。
// 四类记录同事务提交，不在事务中调用外部系统。提交失败可能结果未知，
// 调用方应保留命令 ID 查询，不生成新 ID 自动重发。
func (s *Service) Accept(ctx context.Context, actorID, projectID, commandID string, request Request) (Receipt, bool, error) {
	if err := validateAccess(actorID, projectID, commandID); err != nil {
		return Receipt{}, false, err
	}
	request, err := NormalizeRequest(request)
	if err != nil {
		return Receipt{}, false, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Receipt{}, false, unavailable(err)
	}
	defer rollback(tx)
	queries := sqlc.New(tx)
	taskID, err := uuid.NewRandom()
	if err != nil {
		return Receipt{}, false, fmt.Errorf("生成草稿标识: %w", err)
	}
	row, err := queries.InsertCommand(ctx, sqlc.InsertCommandParams{
		ProjectID: databaseID(projectID), CommandID: databaseID(commandID), ActorID: databaseID(actorID),
		Kind: request.Kind, TaskID: pgtype.UUID{Bytes: taskID, Valid: true}, TaskTitle: request.Payload.Title,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// READ COMMITTED 的下一条语句能看到唯一键竞争获胜者刚提交的记录。
		row, err = queries.GetCommand(ctx, sqlc.GetCommandParams{ProjectID: databaseID(projectID), CommandID: databaseID(commandID)})
		if err != nil {
			return Receipt{}, false, unavailable(err)
		}
		if row.ActorID != databaseID(actorID) || row.Kind != request.Kind || row.TaskTitle != request.Payload.Title {
			return Receipt{}, false, ErrConflict
		}
		return receipt(row), false, nil
	}
	if err != nil {
		return Receipt{}, false, unavailable(err)
	}
	if err = queries.InsertTask(ctx, sqlc.InsertTaskParams{TaskID: row.TaskID, ProjectID: row.ProjectID, CommandID: row.CommandID, Title: row.TaskTitle}); err != nil {
		return Receipt{}, false, unavailable(err)
	}
	if err = queries.InsertAuditEvent(ctx, sqlc.InsertAuditEventParams{ProjectID: row.ProjectID, CommandID: row.CommandID, ActorID: row.ActorID, TaskID: row.TaskID}); err != nil {
		return Receipt{}, false, unavailable(err)
	}
	if err = queries.InsertOutboxEvent(ctx, sqlc.InsertOutboxEventParams{ProjectID: row.ProjectID, CommandID: row.CommandID, TaskID: row.TaskID}); err != nil {
		return Receipt{}, false, unavailable(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Receipt{}, false, unavailable(err)
	}
	return receipt(row), true, nil
}

// Get 从数据库查询原命令，每次重验项目与身份；不存在不自动触发提交。
func (s *Service) Get(ctx context.Context, actorID, projectID, commandID string) (Receipt, error) {
	if err := validateAccess(actorID, projectID, commandID); err != nil {
		return Receipt{}, err
	}
	row, err := sqlc.New(s.pool).GetCommand(ctx, sqlc.GetCommandParams{ProjectID: databaseID(projectID), CommandID: databaseID(commandID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, unavailable(err)
	}
	if row.ActorID != databaseID(actorID) {
		return Receipt{}, ErrForbidden
	}
	return receipt(row), nil
}

func databaseID(value string) pgtype.UUID {
	// 所有调用者在进入存储边界前已通过 ValidID；内部不接受未验证的字符串。
	id, _ := uuid.Parse(value)
	return pgtype.UUID{Bytes: id, Valid: true}
}

func receipt(row sqlc.Command) Receipt {
	return Receipt{CommandID: uuid.UUID(row.CommandID.Bytes).String(), ProjectID: uuid.UUID(row.ProjectID.Bytes).String(),
		Kind: row.Kind, State: row.State, TaskID: uuid.UUID(row.TaskID.Bytes).String(), TaskTitle: row.TaskTitle,
		TaskState: row.TaskState, Version: strconv.FormatInt(row.Version, 10), AcceptedAt: row.AcceptedAt.Time.UTC().Format(time.RFC3339Nano)}
}

func unavailable(err error) error { return fmt.Errorf("%w: %w", ErrUnavailable, err) }

func rollback(tx pgx.Tx) {
	// 请求取消后也要释放连接上的事务；此上下文仅用于有上限的资源清理。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// pgx 在回滚失败时关闭连接；已提交事务的 ErrTxClosed 也无需再处理。
	_ = tx.Rollback(ctx)
}
