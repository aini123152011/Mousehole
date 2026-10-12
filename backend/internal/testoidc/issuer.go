// Package testoidc 提供测试和本地开发专用的合成 OIDC issuer。
// 它只实现标准 discovery、JWKS、自动授权和 authorization_code + PKCE 交换。
package testoidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Config 是合成 issuer 的固定协议参数；仅允许测试拥有的 loopback 服务使用。
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	Subject      string
	Token        TokenOptions
}

// TokenOptions 只用于合成无效 claim 和签名场景；零值生成标准有效 token。
type TokenOptions struct {
	Issuer                 string
	Audience               []string
	AuthorizedParty        string
	OmitAuthorizedParty    bool
	Nonce                  string
	Lifetime               time.Duration
	SignWithUnpublishedKey bool
}

type authorization struct {
	redirectURI   string
	nonce         string
	codeChallenge string
	createdAt     time.Time
}

// Issuer 是并发安全的内存 OIDC 服务；授权码一次性消费且五分钟过期。
type Issuer struct {
	config         Config
	key            *rsa.PrivateKey
	keyID          string
	unpublishedKey *rsa.PrivateKey
	mux            *http.ServeMux
	mu             sync.Mutex
	codes          map[string]authorization
}

// New 创建新的合成密钥和 issuer handler。
func New(config Config) (*Issuer, error) {
	if _, err := url.ParseRequestURI(config.Issuer); err != nil || config.ClientID == "" || config.ClientSecret == "" || config.Subject == "" {
		return nil, errors.New("合成 OIDC 配置无效")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	unpublishedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	issuer := &Issuer{config: config, key: key, keyID: "mousehole-test-key-1", unpublishedKey: unpublishedKey, mux: http.NewServeMux(), codes: make(map[string]authorization)}
	issuer.mux.HandleFunc("/.well-known/openid-configuration", issuer.discovery)
	issuer.mux.HandleFunc("/jwks", issuer.jwks)
	issuer.mux.HandleFunc("/authorize", issuer.authorize)
	issuer.mux.HandleFunc("/token", issuer.token)
	return issuer, nil
}

// Handler 返回只暴露 OIDC 协议端点的 HTTP handler。
func (i *Issuer) Handler() http.Handler { return i.mux }

// RotateSigningKey 替换发布的签名密钥和 kid，用于验证客户端重新拉取 JWKS。
func (i *Issuer) RotateSigningKey() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	keyID, err := randomValue()
	if err != nil {
		return err
	}
	i.mu.Lock()
	i.key = key
	i.keyID = keyID
	i.mu.Unlock()
	return nil
}

func (i *Issuer) discovery(w http.ResponseWriter, _ *http.Request) {
	i.json(w, map[string]any{
		"issuer": i.config.Issuer, "authorization_endpoint": i.config.Issuer + "/authorize",
		"token_endpoint": i.config.Issuer + "/token", "jwks_uri": i.config.Issuer + "/jwks",
		"response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (i *Issuer) jwks(w http.ResponseWriter, _ *http.Request) {
	i.mu.Lock()
	key := jose.JSONWebKey{Key: &i.key.PublicKey, KeyID: i.keyID, Algorithm: string(jose.RS256), Use: "sig"}
	i.mu.Unlock()
	i.json(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
}

func (i *Issuer) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("client_id") != i.config.ClientID || query.Get("response_type") != "code" || query.Get("state") == "" || query.Get("nonce") == "" ||
		query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("redirect_uri") == "" {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	code, err := randomValue()
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}
	i.mu.Lock()
	i.codes[code] = authorization{redirectURI: query.Get("redirect_uri"), nonce: query.Get("nonce"), codeChallenge: query.Get("code_challenge"), createdAt: time.Now().UTC()}
	i.mu.Unlock()
	redirect, _ := url.Parse(query.Get("redirect_uri"))
	values := redirect.Query()
	values.Set("code", code)
	values.Set("state", query.Get("state"))
	redirect.RawQuery = values.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (i *Issuer) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.ParseForm() != nil || r.Form.Get("grant_type") != "authorization_code" {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	clientID, clientSecret := r.Form.Get("client_id"), r.Form.Get("client_secret")
	if basicID, basicSecret, ok := r.BasicAuth(); ok {
		clientID, clientSecret = basicID, basicSecret
	}
	if clientID != i.config.ClientID || clientSecret != i.config.ClientSecret {
		http.Error(w, "invalid_client", http.StatusUnauthorized)
		return
	}
	i.mu.Lock()
	authorization, ok := i.codes[r.Form.Get("code")]
	delete(i.codes, r.Form.Get("code"))
	i.mu.Unlock()
	challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
	if !ok || time.Since(authorization.createdAt) > 5*time.Minute || authorization.redirectURI != r.Form.Get("redirect_uri") || base64.RawURLEncoding.EncodeToString(challenge[:]) != authorization.codeChallenge {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
		return
	}
	i.mu.Lock()
	signingKey, signingKeyID := i.key, i.keyID
	if i.config.Token.SignWithUnpublishedKey {
		signingKey, signingKeyID = i.unpublishedKey, "mousehole-unpublished-key"
	}
	i.mu.Unlock()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: signingKey, KeyID: signingKeyID}}, nil)
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	tokenIssuer := i.config.Token.Issuer
	if tokenIssuer == "" {
		tokenIssuer = i.config.Issuer
	}
	audience := i.config.Token.Audience
	if audience == nil {
		audience = []string{i.config.ClientID}
	}
	nonce := i.config.Token.Nonce
	if nonce == "" {
		nonce = authorization.nonce
	}
	lifetime := i.config.Token.Lifetime
	if lifetime == 0 {
		lifetime = 5 * time.Minute
	}
	authorizedParty := i.config.Token.AuthorizedParty
	if authorizedParty == "" && !i.config.Token.OmitAuthorizedParty {
		authorizedParty = i.config.ClientID
	}
	claims := struct {
		jwt.Claims
		Nonce string `json:"nonce"`
		AZP   string `json:"azp,omitempty"`
	}{Claims: jwt.Claims{Issuer: tokenIssuer, Subject: i.config.Subject, Audience: jwt.Audience(audience), IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(lifetime))}, Nonce: nonce, AZP: authorizedParty}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}
	i.json(w, map[string]any{"access_token": "opaque-test-access-token", "token_type": "Bearer", "expires_in": 300, "id_token": raw})
}

func (i *Issuer) json(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func randomValue() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

// Serve 运行独立合成 issuer，调用者拥有 listener 和受控停机上下文。
func Serve(ctx context.Context, address string, issuer *Issuer) error {
	server := &http.Server{Addr: address, Handler: issuer.Handler(), ReadHeaderTimeout: 5 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
