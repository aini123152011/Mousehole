package scopecommand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"mousehole/backend/internal/access"
	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/policy"
	"mousehole/backend/internal/store/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) readTx(ctx context.Context, actor, project string, write bool) (pgx.Tx, *sqlc.Queries, error) {
	if !command.ValidID(actor) || !command.ValidID(project) {
		return nil, nil, ErrInvalidRequest
	}
	tx, q, e := begin(ctx, s.pool)
	if e != nil {
		return nil, nil, e
	}
	if e = access.Lock(ctx, q, actor, project, write); e != nil {
		rollback(tx)
		return nil, nil, mapped(e)
	}
	return tx, q, nil
}
func commitRead(ctx context.Context, tx pgx.Tx) error {
	if e := tx.Commit(ctx); e != nil {
		return unavailable(e)
	}
	return nil
}

// PreviewAssets 仅显示本项目已有数量；错误行正常返回，且不写资产或命令。
func (s *Service) PreviewAssets(ctx context.Context, actor, project string, r asset.Import) (asset.Preview, error) {
	tx, q, e := s.readTx(ctx, actor, project, true)
	if e != nil {
		return asset.Preview{}, e
	}
	defer rollback(tx)
	normalized, preview, e := asset.Normalize(r)
	if e != nil && len(preview.Rows) == 0 {
		return preview, ErrInvalidRequest
	}
	existing := 0
	for _, input := range asset.Unique(normalized.Entries) {
		_, e = q.FindProjectAsset(ctx, sqlc.FindProjectAssetParams{ProjectID: access.ID(project), Kind: input.Kind, CanonicalValue: input.Value})
		if e == nil {
			existing++
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return preview, unavailable(e)
		}
	}
	preview.ExistingCount = fmt.Sprint(existing)
	return preview, commitRead(ctx, tx)
}

// GetPolicy 对稳定策略行取共享锁后公开当前禁止；没有稳定行视为依赖故障。
func (s *Service) GetPolicy(ctx context.Context, actor, project string) (Policy, error) {
	tx, q, e := s.readTx(ctx, actor, project, false)
	if e != nil {
		return Policy{}, e
	}
	defer rollback(tx)
	g, p, e := lockPolicy(ctx, q, project, false, false)
	if e != nil {
		return Policy{}, e
	}
	denies, e := readDenies(ctx, q, project)
	if e != nil {
		return Policy{}, e
	}
	out := Policy{ProjectID: project, GlobalRevision: strconv.FormatInt(g, 10), ProjectRevision: strconv.FormatInt(p.Revision, 10), ActiveVersionID: optionalID(p.ActiveVersionID), Denies: denies}
	return out, commitRead(ctx, tx)
}

// GetVersion 只从当前项目读取版本，展示状态按读取时刻计算，冻结回执仍保持原值。
func (s *Service) GetVersion(ctx context.Context, actor, project, id string) (authorization.Version, error) {
	if !command.ValidID(id) {
		return authorization.Version{}, ErrInvalidRequest
	}
	tx, q, e := s.readTx(ctx, actor, project, false)
	if e != nil {
		return authorization.Version{}, e
	}
	defer rollback(tx)
	at, e := q.ScopeClock(ctx)
	if e != nil {
		return authorization.Version{}, unavailable(e)
	}
	v, e := readVersion(ctx, q, project, id, at.Time)
	if e != nil {
		return v, e
	}
	return v, commitRead(ctx, tx)
}

// GetPrecheck 读取冻结决策，权限每次重新检查；历史 checked_at 不更新。
func (s *Service) GetPrecheck(ctx context.Context, actor, project, id string) (policy.Decision, error) {
	if !command.ValidID(id) {
		return policy.Decision{}, ErrInvalidRequest
	}
	tx, q, e := s.readTx(ctx, actor, project, false)
	if e != nil {
		return policy.Decision{}, e
	}
	defer rollback(tx)
	raw, e := q.GetPolicyPrecheck(ctx, sqlc.GetPolicyPrecheckParams{ProjectID: access.ID(project), ID: access.ID(id)})
	if e != nil {
		return policy.Decision{}, mapped(e)
	}
	var d policy.Decision
	if e = json.Unmarshal(raw, &d); e != nil {
		return d, unavailable(e)
	}
	return d, commitRead(ctx, tx)
}

func cursorParams(c *cursorData) (pgtype.Timestamptz, pgtype.UUID) {
	if c == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	at, _ := time.Parse(time.RFC3339Nano, c.At)
	return stamp(at), access.ID(c.ID)
}
func makePage[T any](s *Service, project, resource string, o ListOptions, items []T, at, id func(T) string) Page[T] {
	more := len(items) > o.PageSize
	if more {
		items = items[:o.PageSize]
	}
	out := Page[T]{Items: items, HasMore: more, PageCount: fmt.Sprint(len(items))}
	if more {
		last := items[len(items)-1]
		cursor := s.cursor(project, resource, o, at(last), id(last))
		out.NextCursor = &cursor
	}
	return out
}

// ListAssets 仅按本项目关联分页；cursor 签名绑定项目和筛选摘要，搜索不会进入地址栏。
func (s *Service) ListAssets(ctx context.Context, actor, project string, o ListOptions) (Page[asset.Asset], error) {
	o, c, e := s.parseCursor(project, "assets", o)
	if e != nil {
		return Page[asset.Asset]{}, e
	}
	tx, q, e := s.readTx(ctx, actor, project, false)
	if e != nil {
		return Page[asset.Asset]{}, e
	}
	defer rollback(tx)
	at, id := cursorParams(c)
	rows, e := q.ListProjectAssets(ctx, sqlc.ListProjectAssetsParams{ProjectID: access.ID(project), Column2: o.Kind, Column3: escapedSearch(o.Search), Limit: int32(o.PageSize + 1), BeforeAt: at, BeforeID: id})
	if e != nil {
		return Page[asset.Asset]{}, unavailable(e)
	}
	items := []asset.Asset{}
	for _, row := range rows {
		a, e := readAsset(ctx, q, project, idString(row.ID))
		if e != nil {
			return Page[asset.Asset]{}, e
		}
		items = append(items, a)
	}
	page := makePage(s, project, "assets", o, items, func(a asset.Asset) string { return a.CreatedAt }, func(a asset.Asset) string { return a.ID })
	return page, commitRead(ctx, tx)
}

// ListVersions 使用不可变版本创建时间的稳定 seek，无未经计算的全量计数。
func (s *Service) ListVersions(ctx context.Context, actor, project string, o ListOptions) (Page[authorization.Version], error) {
	if o.Kind != "" || o.Search != "" {
		return Page[authorization.Version]{}, ErrInvalidRequest
	}
	o, c, e := s.parseCursor(project, "versions", o)
	if e != nil {
		return Page[authorization.Version]{}, e
	}
	tx, q, e := s.readTx(ctx, actor, project, false)
	if e != nil {
		return Page[authorization.Version]{}, e
	}
	defer rollback(tx)
	at, id := cursorParams(c)
	rows, e := q.ListAuthorizationVersions(ctx, sqlc.ListAuthorizationVersionsParams{ProjectID: access.ID(project), Limit: int32(o.PageSize + 1), BeforeAt: at, BeforeID: id})
	if e != nil {
		return Page[authorization.Version]{}, unavailable(e)
	}
	now, e := q.ScopeClock(ctx)
	if e != nil {
		return Page[authorization.Version]{}, unavailable(e)
	}
	items := []authorization.Version{}
	for _, row := range rows {
		v, e := readVersion(ctx, q, project, idString(row.ID), now.Time)
		if e != nil {
			return Page[authorization.Version]{}, e
		}
		items = append(items, v)
	}
	page := makePage(s, project, "versions", o, items, func(v authorization.Version) string { return v.CreatedAt }, func(v authorization.Version) string { return v.ID })
	return page, commitRead(ctx, tx)
}

// ListPrechecks 分页冻结历史；更新当前规则不会重写先前 decision。
func (s *Service) ListPrechecks(ctx context.Context, actor, project string, o ListOptions) (Page[policy.Decision], error) {
	if o.Kind != "" || o.Search != "" {
		return Page[policy.Decision]{}, ErrInvalidRequest
	}
	o, c, e := s.parseCursor(project, "prechecks", o)
	if e != nil {
		return Page[policy.Decision]{}, e
	}
	tx, q, e := s.readTx(ctx, actor, project, false)
	if e != nil {
		return Page[policy.Decision]{}, e
	}
	defer rollback(tx)
	at, id := cursorParams(c)
	rows, e := q.ListPolicyPrechecks(ctx, sqlc.ListPolicyPrechecksParams{ProjectID: access.ID(project), Limit: int32(o.PageSize + 1), BeforeAt: at, BeforeID: id})
	if e != nil {
		return Page[policy.Decision]{}, unavailable(e)
	}
	items := []policy.Decision{}
	for _, raw := range rows {
		var d policy.Decision
		if e = json.Unmarshal(raw, &d); e != nil {
			return Page[policy.Decision]{}, unavailable(e)
		}
		items = append(items, d)
	}
	page := makePage(s, project, "prechecks", o, items, func(d policy.Decision) string { return d.CheckedAt }, func(d policy.Decision) string { return d.ID })
	return page, commitRead(ctx, tx)
}
