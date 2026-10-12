package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mousehole/backend/internal/store/sqlc"
)

type store struct{ pool *pgxpool.Pool }

type consumedAttempt struct {
	nonceDigest       []byte
	encryptedVerifier []byte
	returnPath        string
}

const adminOperatorSource = "admin_cli"

func (s *store) provision(ctx context.Context, request ProvisionRequest) error {
	if !validProvision(request) || !canonicalID(request.UserID) || !canonicalID(request.ProjectID) {
		return ErrInvalidRequest
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return unavailable(err)
	}
	defer rollback(tx)
	q := sqlc.New(tx)
	userID, projectID := databaseID(request.UserID), databaseID(request.ProjectID)
	email := nullableText(request.Email)
	changed := false
	if rows, insertErr := q.InsertUser(ctx, sqlc.InsertUserParams{ID: userID, DisplayName: strings.TrimSpace(request.DisplayName), Email: email}); insertErr != nil {
		return unavailable(insertErr)
	} else {
		changed = changed || rows == 1
	}
	user, err := q.GetUser(ctx, userID)
	if err != nil {
		return unavailable(err)
	}
	if user.DisplayName != strings.TrimSpace(request.DisplayName) || user.Email != email || user.Status != "active" {
		return ErrConflict
	}
	if rows, insertErr := q.InsertExternalIdentity(ctx, sqlc.InsertExternalIdentityParams{
		Issuer: request.Issuer, Subject: request.Subject, UserID: userID, Email: email,
		DisplayName: nullableText(request.DisplayName),
	}); insertErr != nil {
		return conflictOrUnavailable(insertErr)
	} else {
		changed = changed || rows == 1
	}
	external, err := q.GetExternalIdentity(ctx, sqlc.GetExternalIdentityParams{Issuer: request.Issuer, Subject: request.Subject})
	if err != nil {
		return unavailable(err)
	}
	if external.UserID != userID {
		return ErrConflict
	}
	if rows, insertErr := q.InsertProject(ctx, sqlc.InsertProjectParams{ID: projectID, Name: strings.TrimSpace(request.ProjectName)}); insertErr != nil {
		return unavailable(insertErr)
	} else {
		changed = changed || rows == 1
	}
	project, err := q.GetProject(ctx, projectID)
	if err != nil {
		return unavailable(err)
	}
	if project.Name != strings.TrimSpace(request.ProjectName) || project.Status != "active" {
		return ErrConflict
	}
	if rows, insertErr := q.InsertProjectMembership(ctx, sqlc.InsertProjectMembershipParams{ProjectID: projectID, UserID: userID, Role: request.Role}); insertErr != nil {
		return unavailable(insertErr)
	} else {
		changed = changed || rows == 1
	}
	membership, err := q.GetProjectMembership(ctx, sqlc.GetProjectMembershipParams{ProjectID: projectID, UserID: userID})
	if err != nil {
		return unavailable(err)
	}
	if membership.Role != request.Role || membership.State != "active" {
		return ErrConflict
	}
	if changed {
		summary, _ := json.Marshal(map[string]string{"role": request.Role})
		eventID, eventErr := uuid.NewRandom()
		if eventErr != nil {
			return unavailable(eventErr)
		}
		if err = q.InsertAccessAuditEvent(ctx, sqlc.InsertAccessAuditEventParams{
			ID: databaseUUID(eventID), ActorID: pgtype.UUID{}, OperatorSource: "bootstrap_cli",
			EventType: "identity_provisioned", ObjectType: "project_membership",
			ObjectID: request.ProjectID + ":" + request.UserID, RequestID: pgtype.UUID{}, Summary: summary,
		}); err != nil {
			return unavailable(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

func (s *store) insertLoginAttempt(ctx context.Context, state, binding, nonce string, verifier []byte, expiresAt time.Time) error {
	err := sqlc.New(s.pool).InsertLoginAttempt(ctx, sqlc.InsertLoginAttemptParams{
		StateDigest: digest(state), BrowserDigest: digest(binding), NonceDigest: digest(nonce),
		PkceVerifierEncrypted: verifier, ReturnPath: "/", ExpiresAt: timestamp(expiresAt),
	})
	if err != nil {
		return unavailable(err)
	}
	return nil
}

func (s *store) consumeLoginAttempt(ctx context.Context, state, binding string) (consumedAttempt, error) {
	row, err := sqlc.New(s.pool).ConsumeLoginAttempt(ctx, sqlc.ConsumeLoginAttemptParams{StateDigest: digest(state), BrowserDigest: digest(binding)})
	if errors.Is(err, pgx.ErrNoRows) {
		return consumedAttempt{}, ErrInvalidRequest
	}
	if err != nil {
		return consumedAttempt{}, unavailable(err)
	}
	return consumedAttempt{nonceDigest: row.NonceDigest, encryptedVerifier: row.PkceVerifierEncrypted, returnPath: row.ReturnPath}, nil
}

func (s *store) createSession(ctx context.Context, issuer, subject string, idleTTL, absoluteTTL time.Duration) (string, time.Time, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", time.Time{}, unavailable(err)
	}
	defer rollback(tx)
	q := sqlc.New(tx)
	user, err := q.GetProvisionedIdentity(ctx, sqlc.GetProvisionedIdentityParams{Issuer: issuer, Subject: subject})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, ErrIdentityNotProvisioned
	}
	if err != nil {
		return "", time.Time{}, unavailable(err)
	}
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, unavailable(err)
	}
	now := time.Now().UTC()
	absoluteExpiresAt := now.Add(absoluteTTL)
	idleExpiresAt := now.Add(idleTTL)
	if idleExpiresAt.After(absoluteExpiresAt) {
		idleExpiresAt = absoluteExpiresAt
	}
	if err = q.InsertSession(ctx, sqlc.InsertSessionParams{
		TokenDigest: digest(token), UserID: user.ID, IdleExpiresAt: timestamp(idleExpiresAt),
		AbsoluteExpiresAt: timestamp(absoluteExpiresAt),
	}); err != nil {
		return "", time.Time{}, unavailable(err)
	}
	eventID, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, unavailable(err)
	}
	if err = q.InsertAccessAuditEvent(ctx, sqlc.InsertAccessAuditEventParams{
		ID: databaseUUID(eventID), ActorID: user.ID, OperatorSource: "oidc_callback",
		EventType: "session_created", ObjectType: "session", ObjectID: "sha256:" + fmt.Sprintf("%x", digest(token)[:8]),
		RequestID: pgtype.UUID{}, Summary: []byte(`{}`),
	}); err != nil {
		return "", time.Time{}, unavailable(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", time.Time{}, unavailable(err)
	}
	return token, absoluteExpiresAt, nil
}

func (s *store) authenticate(ctx context.Context, token string, idleTTL time.Duration) (Principal, error) {
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	q := sqlc.New(s.pool)
	row, err := q.GetActiveSession(ctx, digest(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrSessionExpired
	}
	if err != nil {
		return Principal{}, unavailable(err)
	}
	projects, err := q.ListActiveProjects(ctx, row.UserID)
	if err != nil {
		return Principal{}, unavailable(err)
	}
	if err = q.TouchSession(ctx, sqlc.TouchSessionParams{
		IdleTtl: pgtype.Interval{Microseconds: idleTTL.Microseconds(), Valid: true}, TokenDigest: digest(token),
	}); err != nil {
		return Principal{}, unavailable(err)
	}
	principal := Principal{
		ActorID: uuid.UUID(row.UserID.Bytes).String(), DisplayName: row.DisplayName,
		Projects: make([]ProjectAccess, 0, len(projects)), AbsoluteExpiresAt: row.AbsoluteExpiresAt.Time.UTC(),
	}
	if row.Email.Valid {
		email := row.Email.String
		principal.Email = &email
	}
	for _, project := range projects {
		principal.Projects = append(principal.Projects, ProjectAccess{
			ID: uuid.UUID(project.ID.Bytes).String(), Name: project.Name, Role: project.Role,
			Capabilities: CapabilitiesForRole(project.Role), Version: strconv.FormatInt(project.Version, 10),
		})
	}
	return principal, nil
}

func (s *store) revokeSession(ctx context.Context, token string) error {
	if token == "" {
		return ErrUnauthenticated
	}
	if _, err := sqlc.New(s.pool).RevokeSession(ctx, digest(token)); err != nil {
		return unavailable(err)
	}
	return nil
}

func (s *store) revokeAllSessions(ctx context.Context, userID string) (AdminMutationResult, error) {
	if !canonicalID(userID) {
		return AdminMutationResult{}, ErrInvalidRequest
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	defer rollback(tx)
	q := sqlc.New(tx)
	if _, err = q.LockUserForAdmin(ctx, databaseID(userID)); errors.Is(err, pgx.ErrNoRows) {
		return AdminMutationResult{}, ErrAdminTargetNotFound
	}
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	revoked, err := q.RevokeAllUserSessions(ctx, databaseID(userID))
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	result := AdminMutationResult{Changed: revoked > 0, RevokedSessions: revoked}
	if result.Changed {
		if err = insertAdminAudit(ctx, q, "user_sessions_revoked", "user", userID, map[string]any{"revoked_sessions": revoked}); err != nil {
			return AdminMutationResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	return result, nil
}

func (s *store) disableUser(ctx context.Context, userID string) (AdminMutationResult, error) {
	if !canonicalID(userID) {
		return AdminMutationResult{}, ErrInvalidRequest
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	defer rollback(tx)
	q := sqlc.New(tx)
	if _, err = q.LockUserForAdmin(ctx, databaseID(userID)); errors.Is(err, pgx.ErrNoRows) {
		return AdminMutationResult{}, ErrAdminTargetNotFound
	}
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	disabled, err := q.DisableUser(ctx, databaseID(userID))
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	revoked, err := q.RevokeAllUserSessions(ctx, databaseID(userID))
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	result := AdminMutationResult{Changed: disabled > 0 || revoked > 0, RevokedSessions: revoked}
	if result.Changed {
		if err = insertAdminAudit(ctx, q, "user_disabled", "user", userID, map[string]any{"revoked_sessions": revoked}); err != nil {
			return AdminMutationResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	return result, nil
}

func (s *store) revokeMembership(ctx context.Context, userID, projectID string) (AdminMutationResult, error) {
	if !canonicalID(userID) || !canonicalID(projectID) {
		return AdminMutationResult{}, ErrInvalidRequest
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	defer rollback(tx)
	q := sqlc.New(tx)
	params := sqlc.LockProjectMembershipForAdminParams{ProjectID: databaseID(projectID), UserID: databaseID(userID)}
	membership, err := q.LockProjectMembershipForAdmin(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminMutationResult{}, ErrAdminTargetNotFound
	}
	if err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	result := AdminMutationResult{}
	if membership.State == "active" {
		if _, err = q.RevokeProjectMembership(ctx, sqlc.RevokeProjectMembershipParams{
			ProjectID: databaseID(projectID), UserID: databaseID(userID),
		}); err != nil {
			return AdminMutationResult{}, unavailable(err)
		}
		result.Changed = true
		if err = insertAdminAudit(ctx, q, "project_membership_revoked", "project_membership", projectID+":"+userID, map[string]any{"role": membership.Role}); err != nil {
			return AdminMutationResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return AdminMutationResult{}, unavailable(err)
	}
	return result, nil
}

func insertAdminAudit(ctx context.Context, q *sqlc.Queries, eventType, objectType, objectID string, summary map[string]any) error {
	eventID, err := uuid.NewRandom()
	if err != nil {
		return unavailable(err)
	}
	encodedSummary, err := json.Marshal(summary)
	if err != nil {
		return unavailable(err)
	}
	if err = q.InsertAccessAuditEvent(ctx, sqlc.InsertAccessAuditEventParams{
		ID: databaseUUID(eventID), ActorID: pgtype.UUID{}, OperatorSource: adminOperatorSource,
		EventType: eventType, ObjectType: objectType, ObjectID: objectID,
		RequestID: pgtype.UUID{}, Summary: encodedSummary,
	}); err != nil {
		return unavailable(err)
	}
	return nil
}

func nullableText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}

func databaseID(value string) pgtype.UUID {
	id, _ := uuid.Parse(value)
	return databaseUUID(id)
}

func databaseUUID(value uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: value, Valid: true} }
func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func conflictOrUnavailable(err error) error {
	// 唯一约束表示同一用户/issuer 已绑定其他 subject，不能静默覆盖。
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return fmt.Errorf("%w: 唯一约束冲突", ErrConflict)
	}
	return unavailable(err)
}

func unavailable(err error) error { return fmt.Errorf("%w: %w", ErrUnavailable, err) }

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
