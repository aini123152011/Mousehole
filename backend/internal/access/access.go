package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mousehole/backend/internal/store/sqlc"
)

// ErrForbidden 表示当前用户、项目或成员不再活动，或角色不足。
var ErrForbidden = errors.New("项目权限不足")

// ID 仅供已验证标准 UUID 的内部存储调用使用。
func ID(value string) pgtype.UUID {
	id, _ := uuid.Parse(value)
	return pgtype.UUID{Bytes: id, Valid: true}
}

// LockParents 按稳定顺序锁定父对象；管理员即使面对禁用用户仍能撤销成员。
func LockParents(ctx context.Context, q *sqlc.Queries, actor, project string) (string, string, error) {
	user, err := q.LockAccessUser(ctx, ID(actor))
	if err != nil {
		return "", "", err
	}
	state, err := q.LockAccessProject(ctx, ID(project))
	return user, state, err
}

// Lock 保持三类共享行锁到调用方提交；禁用/撤销先提交则在等待后重新检查状态。
func Lock(ctx context.Context, q *sqlc.Queries, actor, project string, write bool) error {
	user, state, err := LockParents(ctx, q, actor, project)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if user != "active" || state != "active" {
		return ErrForbidden
	}
	member, err := q.LockAccessMembership(ctx, sqlc.LockAccessMembershipParams{ProjectID: ID(project), UserID: ID(actor)})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if member.State != "active" || member.Role != "viewer" && member.Role != "analyst" || write && member.Role != "analyst" {
		return ErrForbidden
	}
	return nil
}
