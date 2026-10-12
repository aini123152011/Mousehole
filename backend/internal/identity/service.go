package identity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

const loginAttemptTTL = 5 * time.Minute

// Config 是身份协议配置；Issuer、回调和 client ID 必须与 IdP 注册信息精确一致。
type Config struct {
	Issuer             string
	ClientID           string
	ClientSecret       string
	RedirectURL        string
	AuthKey            []byte
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	HTTPClient         *http.Client
}

// Service 完成 OIDC 协议、一次性登录尝试、会话和 CSRF 计算。
type Service struct {
	issuer      string
	clientID    string
	oauth       oauth2.Config
	verifier    *oidc.IDTokenVerifier
	store       store
	keys        keySet
	idleTTL     time.Duration
	absoluteTTL time.Duration
	httpClient  *http.Client
}

// NewService 在启动时完成 OIDC discovery；发现失败会阻止 API 接受登录请求。
func NewService(ctx context.Context, config Config, pool *pgxpool.Pool) (*Service, error) {
	if pool == nil || config.Issuer == "" || config.ClientID == "" || config.RedirectURL == "" {
		return nil, errors.New("OIDC issuer、client ID、redirect URL 和数据库不能为空")
	}
	if config.SessionIdleTTL <= 0 || config.SessionAbsoluteTTL <= 0 || config.SessionIdleTTL > config.SessionAbsoluteTTL {
		return nil, errors.New("会话有效期配置无效")
	}
	keys, err := deriveKeys(config.AuthKey)
	if err != nil {
		return nil, err
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	providerCtx := oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(providerCtx, config.Issuer)
	if err != nil {
		return nil, errors.New("OIDC discovery 失败")
	}
	if provider.Endpoint().AuthURL == "" || provider.Endpoint().TokenURL == "" {
		return nil, errors.New("OIDC discovery 缺少授权或令牌端点")
	}
	return &Service{
		issuer: config.Issuer, clientID: config.ClientID,
		oauth:    oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: config.RedirectURL, Scopes: []string{oidc.ScopeOpenID, "profile", "email"}},
		verifier: provider.Verifier(&oidc.Config{ClientID: config.ClientID}), store: store{pool: pool}, keys: keys,
		idleTTL: config.SessionIdleTTL, absoluteTTL: config.SessionAbsoluteTTL, httpClient: client,
	}, nil
}

// BeginLogin 创建一次性 state、浏览器绑定、nonce 和 PKCE，并返回 IdP 跳转地址。
func (s *Service) BeginLogin(ctx context.Context) (LoginStart, error) {
	state, err := randomToken()
	if err != nil {
		return LoginStart{}, unavailable(err)
	}
	binding, err := randomToken()
	if err != nil {
		return LoginStart{}, unavailable(err)
	}
	nonce, err := randomToken()
	if err != nil {
		return LoginStart{}, unavailable(err)
	}
	verifier, err := randomToken()
	if err != nil {
		return LoginStart{}, unavailable(err)
	}
	encryptedVerifier, err := encrypt(s.keys.loginEncryption, verifier)
	if err != nil {
		return LoginStart{}, unavailable(err)
	}
	expiresAt := time.Now().UTC().Add(loginAttemptTTL)
	if err = s.store.insertLoginAttempt(ctx, state, binding, nonce, encryptedVerifier, expiresAt); err != nil {
		return LoginStart{}, err
	}
	challengeDigest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeDigest[:])
	authorizationURL := s.oauth.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return LoginStart{AuthorizationURL: authorizationURL, BrowserBinding: binding, ExpiresAt: expiresAt}, nil
}

// CompleteLogin 原子领取登录尝试，交换 code，并补齐 go-oidc 未执行的 nonce/azp 校验。
func (s *Service) CompleteLogin(ctx context.Context, state, binding, code string) (LoginResult, error) {
	if state == "" || binding == "" || code == "" {
		return LoginResult{}, ErrInvalidRequest
	}
	attempt, err := s.store.consumeLoginAttempt(ctx, state, binding)
	if err != nil {
		return LoginResult{}, err
	}
	verifier, err := decrypt(s.keys.loginEncryption, attempt.encryptedVerifier)
	if err != nil {
		return LoginResult{}, unavailable(err)
	}
	ctx = oidc.ClientContext(ctx, s.httpClient)
	token, err := s.oauth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return LoginResult{}, ErrInvalidRequest
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return LoginResult{}, ErrInvalidRequest
	}
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return LoginResult{}, ErrInvalidRequest
	}
	var claims struct {
		Subject string `json:"sub"`
		Nonce   string `json:"nonce"`
		AZP     string `json:"azp"`
	}
	if err = idToken.Claims(&claims); err != nil || claims.Subject == "" {
		return LoginResult{}, ErrInvalidRequest
	}
	if !equalDigest(digest(claims.Nonce), attempt.nonceDigest) || !validAuthorizedParty(idToken.Audience, claims.AZP, s.clientID) {
		return LoginResult{}, ErrInvalidRequest
	}
	sessionToken, expiresAt, err := s.store.createSession(ctx, s.issuer, claims.Subject, s.idleTTL, s.absoluteTTL)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{SessionToken: sessionToken, ReturnPath: attempt.returnPath, ExpiresAt: expiresAt}, nil
}

// Authenticate 从不透明 Cookie 恢复用户与当前活动项目，每次请求都会重验本地状态。
func (s *Service) Authenticate(ctx context.Context, sessionToken string) (Principal, error) {
	return s.store.authenticate(ctx, sessionToken, s.idleTTL)
}

// RevokeSession 撤销当前会话；旧 Cookie 重放不能恢复权限。
func (s *Service) RevokeSession(ctx context.Context, sessionToken string) error {
	return s.store.revokeSession(ctx, sessionToken)
}

// CSRFToken 计算只与当前会话原值绑定的 HMAC；数据库和浏览器存储都不保存副本。
func (s *Service) CSRFToken(sessionToken string) string { return csrfToken(s.keys.csrf, sessionToken) }

// ValidateCSRF 使用常量时间比较，拒绝缺失或重复头部后的空值。
func (s *Service) ValidateCSRF(sessionToken, candidate string) error {
	if candidate == "" || !equalToken(s.CSRFToken(sessionToken), candidate) {
		return ErrCSRFInvalid
	}
	return nil
}

// Provision 由受控管理命令幂等创建或核对用户、外部身份、项目与成员关系。
func (s *Service) Provision(ctx context.Context, request ProvisionRequest) error {
	if !validIssuerURL(request.Issuer) {
		return fmt.Errorf("%w: issuer 必须是完整 URL", ErrInvalidRequest)
	}
	return s.store.provision(ctx, request)
}

// ProvisionWithPool 允许 bootstrap CLI 在 OIDC discovery 前建立首位成员。
func ProvisionWithPool(ctx context.Context, pool *pgxpool.Pool, request ProvisionRequest) error {
	if pool == nil {
		return ErrUnavailable
	}
	if !validIssuerURL(request.Issuer) {
		return fmt.Errorf("%w: issuer 必须是完整 URL", ErrInvalidRequest)
	}
	return (&store{pool: pool}).provision(ctx, request)
}

// RevokeAllSessionsWithPool 撤销指定用户的全部未撤销会话，并在同一事务写管理审计。
func RevokeAllSessionsWithPool(ctx context.Context, pool *pgxpool.Pool, userID string) (AdminMutationResult, error) {
	if pool == nil {
		return AdminMutationResult{}, ErrUnavailable
	}
	return (&store{pool: pool}).revokeAllSessions(ctx, userID)
}

// DisableUserWithPool 禁用用户并在同一事务撤销其全部未撤销会话。
func DisableUserWithPool(ctx context.Context, pool *pgxpool.Pool, userID string) (AdminMutationResult, error) {
	if pool == nil {
		return AdminMutationResult{}, ErrUnavailable
	}
	return (&store{pool: pool}).disableUser(ctx, userID)
}

// RevokeMembershipWithPool 保留成员历史行并递增版本，后续项目请求立即拒绝。
func RevokeMembershipWithPool(ctx context.Context, pool *pgxpool.Pool, userID, projectID string) (AdminMutationResult, error) {
	if pool == nil {
		return AdminMutationResult{}, ErrUnavailable
	}
	return (&store{pool: pool}).revokeMembership(ctx, userID, projectID)
}

func validIssuerURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" &&
		parsed.Opaque == "" && parsed.String() == value
}

func validAuthorizedParty(audience []string, azp, clientID string) bool {
	if len(audience) == 0 {
		return false
	}
	found := false
	for _, value := range audience {
		if value == clientID {
			found = true
		}
	}
	if !found || (azp != "" && azp != clientID) {
		return false
	}
	return len(audience) == 1 || azp == clientID
}

func equalDigest(left, right []byte) bool {
	return len(left) == len(right) && equalToken(base64.RawURLEncoding.EncodeToString(left), base64.RawURLEncoding.EncodeToString(right))
}
