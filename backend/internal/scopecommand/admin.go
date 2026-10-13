package scopecommand

import (
	"context"
	"encoding/json"
	"strconv"

	"mousehole/backend/internal/access"
	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/store/sqlc"

	"github.com/google/uuid"
)

// AdminAccept 以受控 CLI 来源受理管理命令；没有经过认证的人填引用只作审计说明。
// 全局 deny 锁 global 排他行，项目变更锁 global 共享→project 排他行；重放先于 CAS。
func (s *Service) AdminAccept(ctx context.Context, project, commandID, operatorRef string, r AdminRequest) (Receipt, bool, error) {
	if !command.ValidID(commandID) || project != "" && !command.ValidID(project) {
		return Receipt{}, false, ErrInvalidRequest
	}
	operatorRef, e := asset.Text(operatorRef, 1, 200)
	if e != nil {
		return Receipt{}, false, ErrInvalidRequest
	}
	expected, e := strconv.ParseInt(r.ExpectedPolicyRevision, 10, 64)
	if e != nil || expected < 1 || strconv.FormatInt(expected, 10) != r.ExpectedPolicyRevision {
		return Receipt{}, false, ErrInvalidRequest
	}
	switch r.Kind {
	case "activate_authorization_version", "revoke_authorization_version":
		if project == "" || !command.ValidID(r.VersionID) || r.Deny != nil || r.DenyID != "" {
			return Receipt{}, false, ErrInvalidRequest
		}
	case "add_scope_deny":
		if r.Deny == nil || r.VersionID != "" || r.DenyID != "" {
			return Receipt{}, false, ErrInvalidRequest
		}
		d, err := authorization.NormalizeDeny(*r.Deny)
		if err != nil {
			return Receipt{}, false, ErrInvalidRequest
		}
		r.Deny = &d
	case "revoke_scope_deny":
		if !command.ValidID(r.DenyID) || r.Deny != nil || r.VersionID != "" {
			return Receipt{}, false, ErrInvalidRequest
		}
	default:
		return Receipt{}, false, ErrInvalidRequest
	}
	tx, q, e := begin(ctx, s.pool)
	if e != nil {
		return Receipt{}, false, e
	}
	defer rollback(tx)
	if project != "" {
		if _, e = q.LockAccessProject(ctx, access.ID(project)); e != nil {
			return Receipt{}, false, mapped(e)
		}
	}
	global, p, e := lockPolicy(ctx, q, project, true, project == "")
	if e != nil {
		return Receipt{}, false, e
	}
	at, e := q.ScopeClock(ctx)
	if e != nil {
		return Receipt{}, false, unavailable(e)
	}
	payload, _ := json.Marshal(r)
	row, replay, e := insertOrReplay(ctx, q, "", project, commandID, "admin_cli", operatorRef, r.Kind, payload, at)
	if e != nil {
		return Receipt{}, false, e
	}
	if replay {
		receipt, err := decodeReceipt(row.Receipt)
		return receipt, false, err
	}
	current := p.Revision
	if project == "" {
		current = global
	}
	if current != expected {
		return Receipt{}, false, ErrRevisionConflict
	}
	objectID := ""
	changed := true
	switch r.Kind {
	case "activate_authorization_version":
		v, err := readVersion(ctx, q, project, r.VersionID, at.Time)
		if err != nil {
			return Receipt{}, false, err
		}
		if v.State != "candidate" || v.DisplayState != "candidate" {
			return Receipt{}, false, ErrInvalidRequest
		}
		if p.ActiveVersionID.Valid {
			if e = q.SetAuthorizationState(ctx, sqlc.SetAuthorizationStateParams{ProjectID: row.ProjectID, VersionID: p.ActiveVersionID, State: "superseded"}); e != nil {
				return Receipt{}, false, unavailable(e)
			}
		}
		if e = q.SetAuthorizationState(ctx, sqlc.SetAuthorizationStateParams{ProjectID: row.ProjectID, VersionID: access.ID(v.ID), State: "active"}); e != nil {
			return Receipt{}, false, unavailable(e)
		}
		p.ActiveVersionID = access.ID(v.ID)
		objectID = v.ID
	case "revoke_authorization_version":
		v, err := readVersion(ctx, q, project, r.VersionID, at.Time)
		if err != nil {
			return Receipt{}, false, err
		}
		changed = v.State != "revoked"
		if changed {
			if e = q.SetAuthorizationState(ctx, sqlc.SetAuthorizationStateParams{ProjectID: row.ProjectID, VersionID: access.ID(v.ID), State: "revoked"}); e != nil {
				return Receipt{}, false, unavailable(e)
			}
		}
		if p.ActiveVersionID == access.ID(v.ID) {
			p.ActiveVersionID = nullableID("")
		}
		objectID = v.ID
	case "add_scope_deny":
		objectID = uuid.NewString()
		d := r.Deny
		if e = q.InsertScopeDeny(ctx, sqlc.InsertScopeDenyParams{ID: access.ID(objectID), ProjectID: row.ProjectID, Kind: d.Kind, Value: d.Value, IncludeSubdomains: d.IncludeSubdomains, Reason: d.Reason, CreatedAt: at}); e != nil {
			return Receipt{}, false, unavailable(e)
		}
	case "revoke_scope_deny":
		d, err := q.GetScopeDeny(ctx, sqlc.GetScopeDenyParams{ID: access.ID(r.DenyID), ProjectID: row.ProjectID})
		if err != nil {
			return Receipt{}, false, mapped(err)
		}
		objectID = r.DenyID
		changed = d.Active
		if changed {
			if e = q.RevokeScopeDeny(ctx, d.ID); e != nil {
				return Receipt{}, false, unavailable(e)
			}
		}
	}
	if changed {
		if project == "" {
			if e = q.BumpGlobalPolicy(ctx); e != nil {
				return Receipt{}, false, unavailable(e)
			}
			global++
		} else {
			if e = q.SetProjectPolicy(ctx, sqlc.SetProjectPolicyParams{ProjectID: row.ProjectID, ActiveVersionID: p.ActiveVersionID}); e != nil {
				return Receipt{}, false, unavailable(e)
			}
			p.Revision++
		}
	}
	result := PolicyChangeResult{Type: "policy_changed", ObjectID: objectID, GlobalRevision: strconv.FormatInt(global, 10), ProjectRevision: strconv.FormatInt(p.Revision, 10)}
	receipt, e := finish(ctx, q, row, result, global, p.Revision)
	if e != nil {
		return Receipt{}, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Receipt{}, false, unavailable(e)
	}
	return receipt, true, nil
}

// AdminGet 只读 CLI 原回执，并要求相同来源及操作引用；不伪造 Web actor。
func (s *Service) AdminGet(ctx context.Context, project, commandID, operatorRef string) (Receipt, error) {
	if !command.ValidID(commandID) || project != "" && !command.ValidID(project) {
		return Receipt{}, ErrInvalidRequest
	}
	operatorRef, e := asset.Text(operatorRef, 1, 200)
	if e != nil {
		return Receipt{}, ErrInvalidRequest
	}
	tx, q, e := begin(ctx, s.pool)
	if e != nil {
		return Receipt{}, e
	}
	defer rollback(tx)
	if project != "" {
		if _, e = q.LockAccessProject(ctx, access.ID(project)); e != nil {
			return Receipt{}, mapped(e)
		}
	}
	namespace, e := q.GetScopeNamespace(ctx, nullableID(project))
	if e != nil {
		return Receipt{}, unavailable(e)
	}
	row, e := q.GetScopeCommand(ctx, sqlc.GetScopeCommandParams{NamespaceID: namespace, CommandID: access.ID(commandID)})
	if e != nil {
		return Receipt{}, mapped(e)
	}
	if row.OperatorSource != "admin_cli" || row.OperatorRef != operatorRef {
		return Receipt{}, ErrNotFound
	}
	if e = tx.Commit(ctx); e != nil {
		return Receipt{}, unavailable(e)
	}
	return decodeReceipt(row.Receipt)
}
