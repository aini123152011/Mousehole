//go:build integration

package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"mousehole/backend/db/migrations"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/testoidc"
)

func identityDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("MOUSEHOLE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("身份集成测试必须设置 MOUSEHOLE_TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil || admin.Ping(ctx) != nil {
		t.Fatal("无法连接真实 PostgreSQL 测试数据库")
	}
	schema := "mousehole_identity_" + uuid.NewString()
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
	})
	db := stdlib.OpenDB(*config.ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestProvisionAndAdminRevocation(t *testing.T) {
	pool := identityDatabase(t)
	ctx := context.Background()
	request := ProvisionRequest{
		Issuer: "https://idp.example:443", Subject: "analyst-1", UserID: "10000000-0000-4000-8000-000000000001",
		DisplayName: "测试分析员", Email: "analyst@example.invalid", ProjectID: "10000000-0000-4000-8000-000000000002",
		ProjectName: "身份集成项目", Role: RoleAnalyst,
	}
	storage := &store{pool: pool}
	if err := storage.provision(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := storage.provision(ctx, request); err != nil {
		t.Fatalf("相同参数重放必须幂等：%v", err)
	}
	conflict := request
	conflict.DisplayName = "覆盖名称"
	if err := storage.provision(ctx, conflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("异参数必须冲突：%v", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM access_audit_events WHERE event_type='identity_provisioned'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("幂等预置审计错误：count=%d err=%v", auditCount, err)
	}
	tokenA, _, err := storage.createSession(ctx, request.Issuer, request.Subject, 30*time.Minute, 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tokenB, _, err := storage.createSession(ctx, request.Issuer, request.Subject, 30*time.Minute, 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var digestCount int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE token_digest = $1", []byte(tokenA)).Scan(&digestCount); err != nil {
		t.Fatal(err)
	}
	if digestCount != 0 {
		t.Fatal("数据库只能保存会话 SHA-256 摘要")
	}
	principal, err := storage.authenticate(ctx, tokenA, 30*time.Minute)
	if err != nil || principal.ActorID != request.UserID || len(principal.Projects) != 1 || len(principal.Projects[0].Capabilities) != 2 {
		t.Fatalf("会话投影错误：%+v err=%v", principal, err)
	}
	commandID := uuid.NewString()
	if _, _, err = command.NewService(pool).Accept(ctx, request.UserID, request.ProjectID, commandID, command.Request{
		Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "撤销前保留草稿"},
	}); err != nil {
		t.Fatal(err)
	}

	revokedSessions, err := RevokeAllSessionsWithPool(ctx, pool, request.UserID)
	if err != nil || !revokedSessions.Changed || revokedSessions.RevokedSessions != 2 {
		t.Fatalf("全局会话撤销结果错误：%+v err=%v", revokedSessions, err)
	}
	replayedSessions, err := RevokeAllSessionsWithPool(ctx, pool, request.UserID)
	if err != nil || replayedSessions.Changed || replayedSessions.RevokedSessions != 0 {
		t.Fatalf("全局会话撤销重放必须幂等：%+v err=%v", replayedSessions, err)
	}
	if _, err = storage.authenticate(ctx, tokenA, 30*time.Minute); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("第一枚旧 Cookie 重放必须失败：%v", err)
	}
	if _, err = storage.authenticate(ctx, tokenB, 30*time.Minute); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("第二枚旧 Cookie 重放必须失败：%v", err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM access_audit_events WHERE event_type='user_sessions_revoked'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("会话撤销重放不得重复审计：count=%d err=%v", auditCount, err)
	}

	membershipToken, _, err := storage.createSession(ctx, request.Issuer, request.Subject, 30*time.Minute, 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	revokedMembership, err := RevokeMembershipWithPool(ctx, pool, request.UserID, request.ProjectID)
	if err != nil || !revokedMembership.Changed {
		t.Fatalf("成员撤销结果错误：%+v err=%v", revokedMembership, err)
	}
	replayedMembership, err := RevokeMembershipWithPool(ctx, pool, request.UserID, request.ProjectID)
	if err != nil || replayedMembership.Changed {
		t.Fatalf("成员撤销重放必须幂等：%+v err=%v", replayedMembership, err)
	}
	principal, err = storage.authenticate(ctx, membershipToken, 30*time.Minute)
	if err != nil || len(principal.Projects) != 0 {
		t.Fatalf("成员撤销后项目应立即消失：%+v err=%v", principal, err)
	}
	if _, _, err = command.NewService(pool).Accept(ctx, request.UserID, request.ProjectID, uuid.NewString(), command.Request{
		Kind: command.CreateTaskDraft, Payload: command.Payload{Title: "撤销后拒绝草稿"},
	}); !errors.Is(err, command.ErrForbidden) {
		t.Fatalf("成员撤销后的命令必须拒绝：%v", err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM access_audit_events WHERE event_type='project_membership_revoked'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("成员撤销重放不得重复审计：count=%d err=%v", auditCount, err)
	}

	disableToken, _, err := storage.createSession(ctx, request.Issuer, request.Subject, 30*time.Minute, 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := DisableUserWithPool(ctx, pool, request.UserID)
	if err != nil || !disabled.Changed || disabled.RevokedSessions != 2 {
		t.Fatalf("用户禁用结果错误：%+v err=%v", disabled, err)
	}
	replayedDisable, err := DisableUserWithPool(ctx, pool, request.UserID)
	if err != nil || replayedDisable.Changed || replayedDisable.RevokedSessions != 0 {
		t.Fatalf("用户禁用重放必须幂等：%+v err=%v", replayedDisable, err)
	}
	if _, err = storage.authenticate(ctx, disableToken, 30*time.Minute); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("禁用后的旧 Cookie 重放必须失败：%v", err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM access_audit_events WHERE event_type='user_disabled'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("用户禁用重放不得重复审计：count=%d err=%v", auditCount, err)
	}

	var userStatus, membershipState string
	var membershipVersion, commandCount, commandAuditCount int
	if err = pool.QueryRow(ctx, "SELECT status FROM users WHERE id=$1", request.UserID).Scan(&userStatus); err != nil || userStatus != "disabled" {
		t.Fatalf("用户历史行必须保留且标记禁用：status=%s err=%v", userStatus, err)
	}
	if err = pool.QueryRow(ctx, "SELECT state, version FROM project_memberships WHERE project_id=$1 AND user_id=$2", request.ProjectID, request.UserID).Scan(&membershipState, &membershipVersion); err != nil || membershipState != "revoked" || membershipVersion != 2 {
		t.Fatalf("成员历史行必须保留且只递增一次：state=%s version=%d err=%v", membershipState, membershipVersion, err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE command_id=$1", commandID).Scan(&commandCount); err != nil || commandCount != 1 {
		t.Fatalf("历史命令不得删除：count=%d err=%v", commandCount, err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE command_id=$1", commandID).Scan(&commandAuditCount); err != nil || commandAuditCount != 1 {
		t.Fatalf("历史命令审计不得删除：count=%d err=%v", commandAuditCount, err)
	}

	missingUserID := uuid.NewString()
	if _, err = RevokeAllSessionsWithPool(ctx, pool, missingUserID); !errors.Is(err, ErrAdminTargetNotFound) {
		t.Fatalf("不存在用户必须返回稳定错误：%v", err)
	}
	if _, err = DisableUserWithPool(ctx, pool, missingUserID); !errors.Is(err, ErrAdminTargetNotFound) {
		t.Fatalf("禁用不存在用户必须返回稳定错误：%v", err)
	}
	if _, err = RevokeMembershipWithPool(ctx, pool, request.UserID, uuid.NewString()); !errors.Is(err, ErrAdminTargetNotFound) {
		t.Fatalf("不存在成员关系必须返回稳定错误：%v", err)
	}
	if _, err = RevokeAllSessionsWithPool(ctx, pool, "10000000-0000-4000-8000-000000000001 "); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("非标准 UUID 必须在存储前拒绝：%v", err)
	}
}

func TestAdminMutationRollsBackWhenAuditFails(t *testing.T) {
	pool := identityDatabase(t)
	ctx := context.Background()
	request := ProvisionRequest{
		Issuer: "https://rollback-idp.example", Subject: "rollback-user", UserID: "30000000-0000-4000-8000-000000000001",
		DisplayName: "事务回滚用户", ProjectID: "30000000-0000-4000-8000-000000000002", ProjectName: "事务回滚项目", Role: RoleAnalyst,
	}
	storage := &store{pool: pool}
	if err := storage.provision(ctx, request); err != nil {
		t.Fatal(err)
	}
	token, _, err := storage.createSession(ctx, request.Issuer, request.Subject, 30*time.Minute, 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
CREATE FUNCTION reject_session_revoke_audit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.event_type = 'user_sessions_revoked' THEN
        RAISE EXCEPTION '合成管理审计故障' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER reject_session_revoke_audit BEFORE INSERT ON access_audit_events
FOR EACH ROW EXECUTE FUNCTION reject_session_revoke_audit();`); err != nil {
		t.Fatal(err)
	}
	if _, err = RevokeAllSessionsWithPool(ctx, pool, request.UserID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("审计失败必须使管理操作返回未确认：%v", err)
	}
	if _, err = storage.authenticate(ctx, token, 30*time.Minute); err != nil {
		t.Fatalf("审计失败后会话撤销必须整体回滚：%v", err)
	}
	var revokedCount int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE user_id=$1 AND revoked_at IS NOT NULL", request.UserID).Scan(&revokedCount); err != nil || revokedCount != 0 {
		t.Fatalf("审计失败不得留下部分撤销：count=%d err=%v", revokedCount, err)
	}
}

func TestLoginAttemptIsBoundAndOneTime(t *testing.T) {
	pool := identityDatabase(t)
	storage := &store{pool: pool}
	if err := storage.insertLoginAttempt(context.Background(), "state", "binding", "nonce", []byte("encrypted-verifier"), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.consumeLoginAttempt(context.Background(), "state", "wrong-binding"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("错误浏览器绑定必须拒绝：%v", err)
	}
	if _, err := storage.consumeLoginAttempt(context.Background(), "state", "binding"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.consumeLoginAttempt(context.Background(), "state", "binding"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("重复回调必须拒绝：%v", err)
	}
}

func newSyntheticService(t *testing.T, pool *pgxpool.Pool, options testoidc.TokenOptions, provision bool) (*Service, *testoidc.Issuer) {
	t.Helper()
	server := httptest.NewServer(nil)
	issuer, err := testoidc.New(testoidc.Config{
		Issuer: server.URL, ClientID: "mousehole-integration", ClientSecret: "integration-secret",
		Subject: "integration-subject", Token: options,
	})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = issuer.Handler()
	t.Cleanup(server.Close)
	if provision {
		err = (&store{pool: pool}).provision(context.Background(), ProvisionRequest{
			Issuer: server.URL, Subject: "integration-subject", UserID: "20000000-0000-4000-8000-000000000001",
			DisplayName: "OIDC 集成测试用户", ProjectID: "20000000-0000-4000-8000-000000000002",
			ProjectName: "OIDC 集成测试项目", Role: RoleAnalyst,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	service, err := NewService(context.Background(), Config{
		Issuer: server.URL, ClientID: "mousehole-integration", ClientSecret: "integration-secret",
		RedirectURL: "http://127.0.0.1:8080/callback", AuthKey: make([]byte, 32),
		SessionIdleTTL: 30 * time.Minute, SessionAbsoluteTTL: 8 * time.Hour,
		HTTPClient: server.Client(),
	}, pool)
	if err != nil {
		t.Fatal(err)
	}
	return service, issuer
}

func authorizationResult(t *testing.T, service *Service) (LoginStart, string, string) {
	t.Helper()
	start, err := service.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	client := *service.httpClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Get(start.AuthorizationURL)
	if err != nil {
		t.Fatal("合成授权端点请求失败")
	}
	defer response.Body.Close()
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || response.StatusCode != http.StatusFound {
		t.Fatalf("合成授权响应无效：status=%d has_location=%t", response.StatusCode, response.Header.Get("Location") != "")
	}
	return start, location.Query().Get("state"), location.Query().Get("code")
}

func TestOIDCProtocolValidationAndKeyRotation(t *testing.T) {
	pool := identityDatabase(t)
	service, issuer := newSyntheticService(t, pool, testoidc.TokenOptions{}, true)
	start, state, code := authorizationResult(t, service)
	result, err := service.CompleteLogin(context.Background(), state, start.BrowserBinding, code)
	if err != nil {
		t.Fatalf("标准 OIDC 登录失败：%v", err)
	}
	if principal, authErr := service.Authenticate(context.Background(), result.SessionToken); authErr != nil || principal.ActorID != "20000000-0000-4000-8000-000000000001" {
		t.Fatalf("登录会话无法恢复：%+v err=%v", principal, authErr)
	}
	if err = issuer.RotateSigningKey(); err != nil {
		t.Fatal(err)
	}
	start, state, code = authorizationResult(t, service)
	if _, err = service.CompleteLogin(context.Background(), state, start.BrowserBinding, code); err != nil {
		t.Fatalf("JWKS 换钥后应重新拉取并完成登录：%v", err)
	}
}

func TestOIDCInvalidStateBindingCodeAndClaimsDoNotCreateSession(t *testing.T) {
	pool := identityDatabase(t)
	service, _ := newSyntheticService(t, pool, testoidc.TokenOptions{}, true)
	start, validState, validCode := authorizationResult(t, service)
	for name, values := range map[string][3]string{
		"错误 state": {"wrong-state", start.BrowserBinding, validCode},
		"错误浏览器绑定":  {validState, "wrong-binding", validCode},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.CompleteLogin(context.Background(), values[0], values[1], values[2]); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("必须拒绝：%v", err)
			}
		})
	}
	start, state, _ := authorizationResult(t, service)
	if _, err := service.CompleteLogin(context.Background(), state, start.BrowserBinding, "invalid-code"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("无效授权码/PKCE 交换必须拒绝：%v", err)
	}

	faults := []struct {
		name    string
		options testoidc.TokenOptions
	}{
		{"错误 nonce", testoidc.TokenOptions{Nonce: "wrong-nonce"}},
		{"错误 audience", testoidc.TokenOptions{Audience: []string{"other-client"}}},
		{"多 audience 缺少 azp", testoidc.TokenOptions{Audience: []string{"mousehole-integration", "other-api"}, OmitAuthorizedParty: true}},
		{"错误 azp", testoidc.TokenOptions{AuthorizedParty: "other-client"}},
		{"错误 issuer", testoidc.TokenOptions{Issuer: "https://other-issuer.example"}},
		{"未发布签名密钥", testoidc.TokenOptions{SignWithUnpublishedKey: true}},
		{"已过期 token", testoidc.TokenOptions{Lifetime: -time.Minute}},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			faulty, _ := newSyntheticService(t, pool, fault.options, true)
			login, loginState, loginCode := authorizationResult(t, faulty)
			if _, err := faulty.CompleteLogin(context.Background(), loginState, login.BrowserBinding, loginCode); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("无效 OIDC token 必须拒绝：%v", err)
			}
		})
	}
	unknown, _ := newSyntheticService(t, pool, testoidc.TokenOptions{}, false)
	login, loginState, loginCode := authorizationResult(t, unknown)
	if _, err := unknown.CompleteLogin(context.Background(), loginState, login.BrowserBinding, loginCode); !errors.Is(err, ErrIdentityNotProvisioned) {
		t.Fatalf("未预置 subject 必须拒绝：%v", err)
	}
	var sessionCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&sessionCount); err != nil || sessionCount != 0 {
		t.Fatalf("无效登录不得创建会话：count=%d err=%v", sessionCount, err)
	}
}
