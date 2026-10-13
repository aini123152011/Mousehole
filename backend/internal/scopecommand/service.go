package scopecommand

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mousehole/backend/internal/access"
	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/policy"
	"mousehole/backend/internal/store/sqlc"
)

// Service 持有连接池及用途隔离的 cursor 签名密钥；无权限/幂等进程缓存。
type Service struct {
	pool      *pgxpool.Pool
	cursorKey []byte
}

// NewService 从部署的 32 字节 auth key 派生 cursor 签名；CLI 不调用分页时传 nil。
func NewService(pool *pgxpool.Pool, key []byte) *Service {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("mousehole:scope-cursor:v1"))
	return &Service{pool: pool, cursorKey: mac.Sum(nil)}
}

func begin(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, *sqlc.Queries, error) {
	tx, e := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, nil, unavailable(e)
	}
	return tx, sqlc.New(tx), nil
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func unavailable(err error) error { return fmt.Errorf("%w: %w", ErrUnavailable, err) }
func mapped(err error) error {
	if errors.Is(err, access.ErrForbidden) {
		return ErrForbidden
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err == nil {
		return nil
	}
	return unavailable(err)
}
func stamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func idString(id pgtype.UUID) string       { return uuid.UUID(id.Bytes).String() }
func optionalID(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	v := idString(id)
	return &v
}
func nullableID(value string) pgtype.UUID {
	if value == "" {
		return pgtype.UUID{}
	}
	return access.ID(value)
}
func timeString(t pgtype.Timestamptz) string { return t.Time.UTC().Format(time.RFC3339Nano) }

func lockPolicy(ctx context.Context, q *sqlc.Queries, project string, write, globalWrite bool) (int64, sqlc.ProjectPolicyState, error) {
	var g int64
	var err error
	if globalWrite {
		g, err = q.LockGlobalPolicyWrite(ctx)
	} else {
		g, err = q.LockGlobalPolicyRead(ctx)
	}
	if err != nil {
		return 0, sqlc.ProjectPolicyState{}, unavailable(err)
	}
	if project == "" {
		return g, sqlc.ProjectPolicyState{}, nil
	}
	var p sqlc.ProjectPolicyState
	if write {
		p, err = q.LockProjectPolicyWrite(ctx, access.ID(project))
	} else {
		p, err = q.LockProjectPolicyRead(ctx, access.ID(project))
	}
	if err != nil {
		return 0, p, unavailable(err)
	}
	return g, p, nil
}

// Accept 先持权限/策略锁，再以 namespace+ID 唯一键受理；业务/审计/outbox 原子提交。
// 重放先核对 principal 和规范参数，不因旧 revision/过期而重做历史业务。
func (s *Service) Accept(ctx context.Context, actor, project, commandID string, r Request) (Receipt, bool, error) {
	if !command.ValidID(actor) || !command.ValidID(project) || !command.ValidID(commandID) {
		return Receipt{}, false, ErrInvalidRequest
	}
	r, err := NormalizeRequest(r)
	if err != nil {
		return Receipt{}, false, err
	}
	tx, q, err := begin(ctx, s.pool)
	if err != nil {
		return Receipt{}, false, err
	}
	defer rollback(tx)
	if err = access.Lock(ctx, q, actor, project, true); err != nil {
		return Receipt{}, false, mapped(err)
	}
	global, p, err := lockPolicy(ctx, q, project, false, false)
	if err != nil {
		return Receipt{}, false, err
	}
	if r.Kind == "create_authorization_version" {
		if _, err = q.LockAuthorizationLetter(ctx, access.ID(project)); err != nil {
			return Receipt{}, false, unavailable(err)
		}
	}
	checked, err := q.ScopeClock(ctx)
	if err != nil {
		return Receipt{}, false, unavailable(err)
	}
	row, replay, err := insertOrReplay(ctx, q, actor, project, commandID, "web", "", r.Kind, r.Payload, checked)
	if err != nil {
		return Receipt{}, false, err
	}
	if replay {
		receipt, e := decodeReceipt(row.Receipt)
		return receipt, false, e
	}
	var result any
	switch r.Kind {
	case "import_assets":
		var payload asset.Import
		_ = json.Unmarshal(r.Payload, &payload)
		result, err = importAssets(ctx, q, row, payload)
	case "create_authorization_version":
		var content authorization.Content
		_ = json.Unmarshal(r.Payload, &content)
		var version authorization.Version
		version, err = createVersion(ctx, q, row, content)
		result = VersionResult{Type: "authorization_version", Version: version}
	case "precheck_target":
		var request policy.Request
		_ = json.Unmarshal(r.Payload, &request)
		var version authorization.Version
		version, err = readVersion(ctx, q, project, request.AuthorizationVersionID, checked.Time)
		if err == nil {
			var denies []authorization.Deny
			denies, err = readDenies(ctx, q, project)
			if err == nil {
				decision := policy.Evaluate(request, version, optionalID(p.ActiveVersionID), denies, checked.Time)
				decision.ID = uuid.NewString()
				decision.ProjectID = project
				decision.CommandID = commandID
				decision.GlobalRevision = strconv.FormatInt(global, 10)
				decision.ProjectRevision = strconv.FormatInt(p.Revision, 10)
				encoded, _ := json.Marshal(decision)
				err = q.InsertPolicyPrecheck(ctx, sqlc.InsertPolicyPrecheckParams{ID: access.ID(decision.ID), ProjectID: access.ID(project), VersionID: access.ID(version.ID), MatchedEntryID: nullableID(pointerValue(decision.MatchedEntryID)), NamespaceID: row.NamespaceID, CommandID: row.CommandID, CheckedAt: checked, Decision: encoded})
				if err != nil {
					err = unavailable(err)
				}
				result = PrecheckResult{Type: "precheck", Decision: decision}
			}
		}
	}
	if err != nil {
		return Receipt{}, false, err
	}
	receipt, err := finish(ctx, q, row, result, global, p.Revision)
	if err != nil {
		return Receipt{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Receipt{}, false, unavailable(err)
	}
	return receipt, true, nil
}
func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func insertOrReplay(ctx context.Context, q *sqlc.Queries, actor, project, commandID, source, operatorRef, kind string, payload []byte, at pgtype.Timestamptz) (sqlc.ScopeCommand, bool, error) {
	namespace, err := q.GetScopeNamespace(ctx, nullableID(project))
	if err != nil {
		return sqlc.ScopeCommand{}, false, unavailable(err)
	}
	row, err := q.InsertScopeCommand(ctx, sqlc.InsertScopeCommandParams{NamespaceID: namespace, CommandID: access.ID(commandID), ProjectID: nullableID(project), ActorID: nullableID(actor), OperatorSource: source, OperatorRef: operatorRef, Kind: kind, Payload: payload, PayloadHash: authorization.Hash(json.RawMessage(payload)), AcceptedAt: at})
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = q.GetScopeCommand(ctx, sqlc.GetScopeCommandParams{NamespaceID: namespace, CommandID: access.ID(commandID)})
		if err != nil {
			return row, false, unavailable(err)
		}
		if row.ActorID != nullableID(actor) || row.OperatorSource != source {
			return row, false, ErrNotFound
		}
		var stored, proposed any
		_ = json.Unmarshal(row.Payload, &stored)
		_ = json.Unmarshal(payload, &proposed)
		a, _ := json.Marshal(stored)
		b, _ := json.Marshal(proposed)
		if row.Kind != kind || row.OperatorRef != operatorRef || string(a) != string(b) {
			return row, false, ErrConflict
		}
		return row, true, nil
	}
	if err != nil {
		return row, false, unavailable(err)
	}
	return row, false, nil
}

func finish(ctx context.Context, q *sqlc.Queries, row sqlc.ScopeCommand, result any, global, projectRevision int64) (Receipt, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return Receipt{}, unavailable(err)
	}
	r := Receipt{CommandID: idString(row.CommandID), ProjectID: optionalID(row.ProjectID), Kind: row.Kind, State: "accepted", AcceptedAt: timeString(row.AcceptedAt), Result: encoded}
	receipt, _ := json.Marshal(r)
	if err = q.FreezeScopeCommand(ctx, sqlc.FreezeScopeCommandParams{NamespaceID: row.NamespaceID, CommandID: row.CommandID, Receipt: receipt}); err != nil {
		return Receipt{}, unavailable(err)
	}
	// 审计只保存安全身份/数量/规则引用，不复制目标 query、联系人或命令正文。
	summary := map[string]any{"kind": row.Kind, "global_revision": strconv.FormatInt(global, 10), "project_revision": strconv.FormatInt(projectRevision, 10), "payload_hash": row.PayloadHash}
	switch v := result.(type) {
	case ImportResult:
		summary["unique_count"] = v.UniqueCount
	case VersionResult:
		summary["object_id"] = v.Version.ID
		summary["content_hash"] = v.Version.ContentHash
	case PrecheckResult:
		summary["object_id"] = v.Decision.ID
		summary["reason_codes"] = v.Decision.ReasonCodes
		summary["content_hash"] = v.Decision.ContentHash
	case PolicyChangeResult:
		summary["object_id"] = v.ObjectID
	}
	audit, _ := json.Marshal(summary)
	if err = q.InsertScopeAudit(ctx, sqlc.InsertScopeAuditParams{NamespaceID: row.NamespaceID, CommandID: row.CommandID, ActorID: row.ActorID, OperatorSource: row.OperatorSource, EventType: row.Kind, Summary: audit, CreatedAt: row.AcceptedAt}); err != nil {
		return Receipt{}, unavailable(err)
	}
	if err = q.InsertScopeOutbox(ctx, sqlc.InsertScopeOutboxParams{NamespaceID: row.NamespaceID, CommandID: row.CommandID, EventType: row.Kind, Summary: audit, CreatedAt: row.AcceptedAt}); err != nil {
		return Receipt{}, unavailable(err)
	}
	return decodeReceipt(receipt)
}
func decodeReceipt(data []byte) (Receipt, error) {
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return r, unavailable(err)
	}
	// JSONB 重新排列键和空白，不改变业务字段；首次与重放使用同一传输规范。
	var result any
	if err := json.Unmarshal(r.Result, &result); err != nil {
		return r, unavailable(err)
	}
	r.Result, _ = json.Marshal(result)
	return r, nil
}

// Get 每次在同事务重验权限并限制原 actor；暂未找到不等于未发生。
func (s *Service) Get(ctx context.Context, actor, project, commandID string) (Receipt, error) {
	if !command.ValidID(actor) || !command.ValidID(project) || !command.ValidID(commandID) {
		return Receipt{}, ErrInvalidRequest
	}
	tx, q, e := begin(ctx, s.pool)
	if e != nil {
		return Receipt{}, e
	}
	defer rollback(tx)
	if e = access.Lock(ctx, q, actor, project, false); e != nil {
		return Receipt{}, mapped(e)
	}
	namespace, e := q.GetScopeNamespace(ctx, access.ID(project))
	if e != nil {
		return Receipt{}, unavailable(e)
	}
	row, e := q.GetScopeCommand(ctx, sqlc.GetScopeCommandParams{NamespaceID: namespace, CommandID: access.ID(commandID)})
	if e != nil {
		return Receipt{}, mapped(e)
	}
	if row.ActorID != access.ID(actor) || row.OperatorSource != "web" {
		return Receipt{}, ErrNotFound
	}
	if e = tx.Commit(ctx); e != nil {
		return Receipt{}, unavailable(e)
	}
	return decodeReceipt(row.Receipt)
}

func importAssets(ctx context.Context, q *sqlc.Queries, row sqlc.ScopeCommand, payload asset.Import) (ImportResult, error) {
	unique := asset.Unique(payload.Entries)
	result := ImportResult{Type: "assets_imported", InputCount: fmt.Sprint(len(payload.Entries)), UniqueCount: fmt.Sprint(len(unique)), DuplicateCount: fmt.Sprint(len(payload.Entries) - len(unique)), Assets: []asset.Asset{}}
	existing := 0
	for _, input := range unique {
		assetID, err := q.UpsertAsset(ctx, sqlc.UpsertAssetParams{ID: access.ID(uuid.NewString()), Kind: input.Kind, CanonicalValue: input.Value})
		if err != nil {
			return result, unavailable(err)
		}
		added, err := q.InsertProjectAsset(ctx, sqlc.InsertProjectAssetParams{ProjectID: row.ProjectID, AssetID: assetID, CreatedAt: row.AcceptedAt})
		if err != nil {
			return result, unavailable(err)
		}
		if added == 0 {
			existing++
		}
		if err = q.InsertAssetSource(ctx, sqlc.InsertAssetSourceParams{ProjectID: row.ProjectID, AssetID: assetID, NamespaceID: row.NamespaceID, CommandID: row.CommandID, Source: payload.Source, Notes: payload.Notes, ExternalRef: payload.ExternalRef, CreatedAt: row.AcceptedAt}); err != nil {
			return result, unavailable(err)
		}
		a, err := readAsset(ctx, q, idString(row.ProjectID), idString(assetID))
		if err != nil {
			return result, err
		}
		result.Assets = append(result.Assets, a)
	}
	result.ExistingCount = fmt.Sprint(existing)
	return result, nil
}

func readAsset(ctx context.Context, q *sqlc.Queries, project, id string) (asset.Asset, error) {
	row, e := q.GetProjectAsset(ctx, sqlc.GetProjectAssetParams{ProjectID: access.ID(project), AssetID: access.ID(id)})
	if e != nil {
		return asset.Asset{}, mapped(e)
	}
	sources, e := q.ListAssetSources(ctx, sqlc.ListAssetSourcesParams{ProjectID: access.ID(project), AssetID: row.ID})
	if e != nil {
		return asset.Asset{}, unavailable(e)
	}
	out := asset.Asset{ID: id, ProjectID: project, Kind: row.Kind, CanonicalValue: row.CanonicalValue, AlgorithmVersion: row.AlgorithmVersion, CreatedAt: timeString(row.CreatedAt), Sources: []asset.Source{}}
	for _, v := range sources {
		out.Sources = append(out.Sources, asset.Source{CommandID: idString(v.CommandID), Source: v.Source, Notes: v.Notes, ExternalRef: v.ExternalRef, CreatedAt: timeString(v.CreatedAt)})
	}
	return out, nil
}

func createVersion(ctx context.Context, q *sqlc.Queries, row sqlc.ScopeCommand, c authorization.Content) (authorization.Version, error) {
	sequence, e := q.NextAuthorizationSequence(ctx, row.ProjectID)
	if e != nil {
		return authorization.Version{}, unavailable(e)
	}
	id := access.ID(uuid.NewString())
	encoded, _ := json.Marshal(c)
	from, _ := time.Parse(time.RFC3339Nano, c.ValidFrom)
	until, _ := time.Parse(time.RFC3339Nano, c.ExpiresAt)
	if e = q.InsertAuthorizationVersion(ctx, sqlc.InsertAuthorizationVersionParams{ID: id, ProjectID: row.ProjectID, Sequence: sequence, Content: encoded, ContentHash: authorization.Hash(c), ValidFrom: stamp(from), ExpiresAt: stamp(until), CreatedAt: row.AcceptedAt}); e != nil {
		return authorization.Version{}, unavailable(e)
	}
	if e = q.InsertAuthorizationState(ctx, sqlc.InsertAuthorizationStateParams{ProjectID: row.ProjectID, VersionID: id}); e != nil {
		return authorization.Version{}, unavailable(e)
	}
	for i, entry := range c.Entries {
		if e = q.InsertAuthorizationEntry(ctx, sqlc.InsertAuthorizationEntryParams{ID: access.ID(uuid.NewString()), ProjectID: row.ProjectID, VersionID: id, Ordinal: int32(i), Kind: entry.Kind, Value: entry.Value, IncludeSubdomains: entry.IncludeSubdomains, Operations: entry.Operations, Ports: entry.Ports, ResolvedIpCidrs: entry.ResolvedIPCIDRs}); e != nil {
			return authorization.Version{}, unavailable(e)
		}
	}
	return readVersion(ctx, q, idString(row.ProjectID), idString(id), row.AcceptedAt.Time)
}

func readVersion(ctx context.Context, q *sqlc.Queries, project, id string, now time.Time) (authorization.Version, error) {
	row, e := q.GetAuthorizationVersion(ctx, sqlc.GetAuthorizationVersionParams{ProjectID: access.ID(project), ID: access.ID(id)})
	if e != nil {
		return authorization.Version{}, mapped(e)
	}
	var c authorization.Content
	if e = json.Unmarshal(row.Content, &c); e != nil {
		return authorization.Version{}, unavailable(e)
	}
	ids, e := q.ListAuthorizationEntryIDs(ctx, sqlc.ListAuthorizationEntryIDsParams{ProjectID: row.ProjectID, VersionID: row.ID})
	if e != nil {
		return authorization.Version{}, unavailable(e)
	}
	entryIDs := []string{}
	for _, v := range ids {
		entryIDs = append(entryIDs, idString(v))
	}
	return authorization.Version{ID: id, ProjectID: project, Sequence: strconv.FormatInt(row.Sequence, 10), ContentHash: row.ContentHash, Content: c, EntryIDs: entryIDs, State: row.State, DisplayState: authorization.DisplayState(row.State, c, now), CreatedAt: timeString(row.CreatedAt)}, nil
}
func readDenies(ctx context.Context, q *sqlc.Queries, project string) ([]authorization.Deny, error) {
	rows, e := q.ListCurrentDenies(ctx, nullableID(project))
	if e != nil {
		return nil, unavailable(e)
	}
	out := []authorization.Deny{}
	for _, v := range rows {
		out = append(out, authorization.Deny{ID: idString(v.ID), ProjectID: optionalID(v.ProjectID), Kind: v.Kind, Value: v.Value, IncludeSubdomains: v.IncludeSubdomains, Reason: v.Reason, Active: v.Active, CreatedAt: timeString(v.CreatedAt)})
	}
	return out, nil
}
