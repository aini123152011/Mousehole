package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mousehole/backend/internal/command"
	"mousehole/backend/internal/identity"
)

const testCommandID = "11111111-1111-4111-8111-111111111111"
const testActorID = "10000000-0000-4000-8000-000000000001"
const testProjectID = "10000000-0000-4000-8000-000000000002"
const validBody = `{"kind":"create_task_draft","payload":{"title":"中文任务"}}`

type fakeCommands struct {
	err     error
	calls   int
	created bool
}

func (f *fakeCommands) Accept(_ context.Context, actor, project, id string, request command.Request) (command.Receipt, bool, error) {
	f.calls++
	return command.Receipt{CommandID: id, ProjectID: project, Kind: request.Kind, State: "accepted", TaskID: testCommandID, TaskTitle: request.Payload.Title, TaskState: "draft", Version: "1", AcceptedAt: "2026-10-12T00:00:00Z"}, f.created, f.err
}
func (f *fakeCommands) Get(_ context.Context, _, _, _ string) (command.Receipt, error) {
	f.calls++
	return command.Receipt{}, f.err
}

type fakeIdentities struct {
	principal     identity.Principal
	authErr       error
	completeErr   error
	completeCalls int
	csrf          string
}

func (f *fakeIdentities) BeginLogin(context.Context) (identity.LoginStart, error) {
	return identity.LoginStart{AuthorizationURL: "http://127.0.0.1:9090/authorize", BrowserBinding: "browser-binding", ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (f *fakeIdentities) CompleteLogin(context.Context, string, string, string) (identity.LoginResult, error) {
	f.completeCalls++
	return identity.LoginResult{SessionToken: "session-token", ReturnPath: "/", ExpiresAt: time.Now().Add(time.Hour)}, f.completeErr
}
func (f *fakeIdentities) Authenticate(context.Context, string) (identity.Principal, error) {
	return f.principal, f.authErr
}
func (f *fakeIdentities) RevokeSession(context.Context, string) error { return nil }
func (f *fakeIdentities) CSRFToken(string) string                     { return f.csrf }
func (f *fakeIdentities) ValidateCSRF(_, candidate string) error {
	if candidate != f.csrf {
		return identity.ErrCSRFInvalid
	}
	return nil
}

func testConfig() Config {
	return Config{
		Mode: "development", ListenAddr: "127.0.0.1:8080", PublicURL: "http://127.0.0.1:8080",
		WebOrigin: "http://127.0.0.1:5173", OIDCIssuer: "http://127.0.0.1:9090",
		OIDCClientID: "mousehole", OIDCClientSecret: "test-secret", AuthKey: make([]byte, 32),
		SessionIdleTTL: 30 * time.Minute, SessionAbsoluteTTL: 8 * time.Hour,
	}
}

func testHandler(t *testing.T, commands *fakeCommands, identities *fakeIdentities) http.Handler {
	t.Helper()
	handler, err := NewHandler(testConfig(), commands, identities, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func request(handler http.Handler, method, path, body string, headers map[string]string, authenticated bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if authenticated {
		r.AddCookie(&http.Cookie{Name: developmentSessionCookie, Value: "session-token"})
	}
	for key, value := range headers {
		if key == "Host" {
			r.Host = value
		} else {
			r.Header.Set(key, value)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestSessionRequiresCookieAndReturnsNoToken(t *testing.T) {
	identities := &fakeIdentities{csrf: "csrf-value-with-at-least-thirty-two-bytes", principal: identity.Principal{
		ActorID: testActorID, DisplayName: "测试分析员", Projects: []identity.ProjectAccess{{ID: testProjectID, Name: "测试项目", Role: identity.RoleAnalyst, Capabilities: identity.CapabilitiesForRole(identity.RoleAnalyst), Version: "1"}},
		AbsoluteExpiresAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
	}}
	handler := testHandler(t, &fakeCommands{}, identities)
	unauthenticated := request(handler, http.MethodGet, "/api/v1/session", "", nil, false)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应为 401，实际=%d", unauthenticated.Code)
	}
	authenticated := request(handler, http.MethodGet, "/api/v1/session", "", nil, true)
	if authenticated.Code != http.StatusOK || strings.Contains(authenticated.Body.String(), "session-token") || !strings.Contains(authenticated.Body.String(), testActorID) {
		t.Fatalf("会话合同无效：%s", authenticated.Body)
	}
}

func TestCommandRequiresSessionCSRFAndOrigin(t *testing.T) {
	commands := &fakeCommands{created: true}
	identities := &fakeIdentities{csrf: "csrf-value-with-at-least-thirty-two-bytes", principal: identity.Principal{ActorID: testActorID}}
	handler := testHandler(t, commands, identities)
	path := "/api/v1/projects/" + testProjectID + "/commands/" + testCommandID
	for _, test := range []struct {
		name          string
		headers       map[string]string
		authenticated bool
		status        int
	}{
		{"未登录", nil, false, 401},
		{"缺 CSRF", map[string]string{"Origin": "http://127.0.0.1:5173"}, true, 403},
		{"错误来源", map[string]string{"Origin": "https://evil.example", "X-CSRF-Token": identities.csrf}, true, 403},
		{"合法写入", map[string]string{"Origin": "http://127.0.0.1:5173", "X-CSRF-Token": identities.csrf}, true, 201},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := request(handler, http.MethodPut, path, validBody, test.headers, test.authenticated)
			if w.Code != test.status {
				t.Fatalf("预期=%d，实际=%d，正文=%s", test.status, w.Code, w.Body)
			}
		})
	}
	if commands.calls != 1 {
		t.Fatalf("只有合法写入应进入服务，实际=%d", commands.calls)
	}
	duplicate := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:8080"+path, strings.NewReader(validBody))
	duplicate.Header.Set("Content-Type", "application/json")
	duplicate.Header.Set("Origin", "http://127.0.0.1:5173")
	duplicate.Header.Add("X-CSRF-Token", identities.csrf)
	duplicate.Header.Add("X-CSRF-Token", identities.csrf)
	duplicate.AddCookie(&http.Cookie{Name: developmentSessionCookie, Value: "session-token"})
	duplicateResponse := httptest.NewRecorder()
	handler.ServeHTTP(duplicateResponse, duplicate)
	if duplicateResponse.Code != http.StatusForbidden || commands.calls != 1 {
		t.Fatalf("重复 CSRF 头必须在进入业务前拒绝：%d", duplicateResponse.Code)
	}
}

func TestStrictJSONAndDomainErrors(t *testing.T) {
	identities := &fakeIdentities{csrf: "csrf-value-with-at-least-thirty-two-bytes", principal: identity.Principal{ActorID: testActorID}}
	path := "/api/v1/projects/" + testProjectID + "/commands/" + testCommandID
	headers := map[string]string{"Origin": "http://127.0.0.1:5173", "X-CSRF-Token": identities.csrf}
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"未知字段", `{"kind":"create_task_draft","payload":{"title":"任务","extra":true}}`, 400},
		{"重复字段", `{"kind":"create_task_draft","kind":"create_task_draft","payload":{"title":"任务"}}`, 400},
		{"空标题", `{"kind":"create_task_draft","payload":{"title":"  "}}`, 400},
		{"超大正文", strings.Repeat("x", 16*1024+1), 413},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands := &fakeCommands{created: true}
			w := request(testHandler(t, commands, identities), http.MethodPut, path, test.body, headers, true)
			if w.Code != test.status || commands.calls != 0 {
				t.Fatalf("严格边界失败：状态=%d，调用=%d，正文=%s", w.Code, commands.calls, w.Body)
			}
			var failure APIError
			if json.Unmarshal(w.Body.Bytes(), &failure) != nil || failure.Code == "" || !command.ValidID(failure.RequestID) {
				t.Fatalf("公开错误合同无效：%s", w.Body)
			}
		})
	}
}

func TestLoginAndCallbackCookies(t *testing.T) {
	identities := &fakeIdentities{csrf: "csrf-value-with-at-least-thirty-two-bytes"}
	handler := testHandler(t, &fakeCommands{}, identities)
	login := request(handler, http.MethodGet, "/api/v1/auth/login", "", nil, false)
	if login.Code != http.StatusFound || login.Header().Get("Location") != "http://127.0.0.1:9090/authorize" {
		t.Fatalf("登录跳转无效：status=%d location_matches=%t", login.Code, login.Header().Get("Location") == "http://127.0.0.1:9090/authorize")
	}
	loginCookie := login.Result().Cookies()[0]
	if !loginCookie.HttpOnly || loginCookie.Secure || loginCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("开发登录 Cookie 属性错误：http_only=%t secure=%t same_site=%d", loginCookie.HttpOnly, loginCookie.Secure, loginCookie.SameSite)
	}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/v1/auth/callback?state=state&code=code", nil)
	r.AddCookie(loginCookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "http://127.0.0.1:5173/" {
		t.Fatalf("回调跳转无效：status=%d location_matches=%t", w.Code, w.Header().Get("Location") == "http://127.0.0.1:5173/")
	}
}

func TestCallbackRejectsDuplicateLoginBindingCookies(t *testing.T) {
	identities := &fakeIdentities{csrf: "csrf-value-with-at-least-thirty-two-bytes"}
	handler := testHandler(t, &fakeCommands{}, identities)
	callback := httptest.NewRequest(
		http.MethodGet,
		"http://127.0.0.1:8080/api/v1/auth/callback?state=state&code=code",
		nil,
	)
	callback.AddCookie(&http.Cookie{Name: developmentLoginCookie, Value: "first-binding"})
	callback.AddCookie(&http.Cookie{Name: developmentLoginCookie, Value: "second-binding"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, callback)
	if response.Code != http.StatusBadRequest || identities.completeCalls != 0 {
		t.Fatalf("重复登录绑定 Cookie 必须在交换令牌前拒绝：status=%d calls=%d", response.Code, identities.completeCalls)
	}
}

func TestCallbackRejectsOIDCErrorParameterBeforeExchange(t *testing.T) {
	identities := &fakeIdentities{csrf: "csrf-value-with-at-least-thirty-two-bytes"}
	handler := testHandler(t, &fakeCommands{}, identities)
	callback := httptest.NewRequest(
		http.MethodGet,
		"http://127.0.0.1:8080/api/v1/auth/callback?state=state&code=code&error=",
		nil,
	)
	callback.AddCookie(&http.Cookie{Name: developmentLoginCookie, Value: "browser-binding"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, callback)
	if response.Code != http.StatusBadRequest || identities.completeCalls != 0 {
		t.Fatalf("OIDC error 参数必须在交换令牌前拒绝：status=%d calls=%d", response.Code, identities.completeCalls)
	}
}

func TestConfigSeparatesDevelopmentAndProduction(t *testing.T) {
	valid := testConfig()
	for _, change := range []func(*Config){
		func(c *Config) { c.Mode = "" },
		func(c *Config) { c.ListenAddr = "0.0.0.0:8080" },
		func(c *Config) { c.PublicURL = "http://127.0.0.1:8080/" },
		func(c *Config) { c.WebOrigin = "http://127.0.0.1:5173?" },
		func(c *Config) { c.OIDCIssuer = "https://idp.example" },
		func(c *Config) { c.PublicURL = "http://127.0.0.1:80" },
		func(c *Config) { c.AuthKey = make([]byte, 31) },
	} {
		cfg := valid
		change(&cfg)
		if cfg.Validate() == nil {
			t.Fatal("无效配置应默认拒绝")
		}
	}
	production := valid
	production.Mode = "production"
	production.ListenAddr = "0.0.0.0:8080"
	production.PublicURL = "https://mousehole.example"
	production.WebOrigin = "https://mousehole.example"
	production.OIDCIssuer = "https://idp.example/realms/mousehole"
	if err := production.Validate(); err != nil || production.sessionCookieName() != productionSessionCookie {
		t.Fatalf("生产配置无效：%v", err)
	}
	if !production.secureCookies() || production.loginCookieName() != productionLoginCookie {
		t.Fatal("生产 Cookie 必须使用 __Host- 名称并启用 Secure")
	}
}

func TestProductionCookiesUseHostPrefixSecureAndNoDomain(t *testing.T) {
	cfg := testConfig()
	cfg.Mode = "production"
	cfg.ListenAddr = "0.0.0.0:8080"
	cfg.PublicURL = "https://mousehole.example"
	cfg.WebOrigin = "https://mousehole.example"
	cfg.OIDCIssuer = "https://idp.example/realms/mousehole"
	handler, err := NewHandler(cfg, &fakeCommands{}, &fakeIdentities{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRequest(http.MethodGet, "https://mousehole.example/api/v1/auth/login", nil)
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("生产登录应只设置一个绑定 Cookie：count=%d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != productionLoginCookie || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.Domain != "" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("生产登录 Cookie 属性错误：name_matches=%t http_only=%t secure=%t path_matches=%t domain_empty=%t same_site=%d", cookie.Name == productionLoginCookie, cookie.HttpOnly, cookie.Secure, cookie.Path == "/", cookie.Domain == "", cookie.SameSite)
	}
}

func TestCallbackFailureDoesNotExposeProtocolSecrets(t *testing.T) {
	var logs bytes.Buffer
	identities := &fakeIdentities{completeErr: fmt.Errorf("%w: id-token-secret", identity.ErrUnavailable)}
	handler, err := NewHandler(testConfig(), &fakeCommands{}, identities, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	callback := httptest.NewRequest(
		http.MethodGet,
		"http://127.0.0.1:8080/api/v1/auth/callback?state=state-secret&code=code-secret",
		nil,
	)
	callback.AddCookie(&http.Cookie{Name: developmentLoginCookie, Value: "binding-secret"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, callback)
	combined := response.Body.String() + logs.String()
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "service_unavailable") {
		t.Fatalf("回调失败页语义错误：status=%d has_public_code=%t", response.Code, strings.Contains(response.Body.String(), "service_unavailable"))
	}
	for _, secret := range []string{"state-secret", "code-secret", "binding-secret", "id-token-secret"} {
		if strings.Contains(combined, secret) {
			t.Fatal("公开页面或日志泄露协议秘密")
		}
	}
}
