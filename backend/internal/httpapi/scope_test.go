package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/identity"
	"mousehole/backend/internal/policy"
	"mousehole/backend/internal/scopecommand"
	"mousehole/backend/internal/validation"
)

const validScopeBody = `{"kind":"import_assets","payload":{"entries":[{"kind":"domain","value":"example.invalid"}],"source":"manual","notes":"中文来源","external_ref":"工单"}}`

type fakeScopes struct {
	err                            error
	created                        bool
	called, actor, project, object string
	options                        scopecommand.ListOptions
}

func (f *fakeScopes) bind(route, actor, project, object string) {
	f.called, f.actor, f.project, f.object = route, actor, project, object
}
func (f *fakeScopes) Accept(_ context.Context, actor, project, id string, r scopecommand.Request) (scopecommand.Receipt, bool, error) {
	f.bind("accept", actor, project, id)
	if f.err != nil {
		return scopecommand.Receipt{}, false, f.err
	}
	if _, e := scopecommand.NormalizeRequest(r); e != nil {
		return scopecommand.Receipt{}, false, e
	}
	return scopecommand.Receipt{CommandID: id, ProjectID: &project, Kind: r.Kind, State: "accepted", AcceptedAt: "2026-10-12T00:00:00Z", Result: json.RawMessage(`{"type":"assets_imported"}`)}, f.created, nil
}
func (f *fakeScopes) Get(_ context.Context, a, p, id string) (scopecommand.Receipt, error) {
	f.bind("command", a, p, id)
	return scopecommand.Receipt{}, f.err
}
func (f *fakeScopes) PreviewAssets(_ context.Context, a, p string, r asset.Import) (asset.Preview, error) {
	f.bind("preview", a, p, "")
	_, preview, _ := asset.Normalize(r)
	return preview, f.err
}
func (f *fakeScopes) ListAssets(_ context.Context, a, p string, o scopecommand.ListOptions) (scopecommand.Page[asset.Asset], error) {
	f.bind("assets", a, p, "")
	f.options = o
	return scopecommand.Page[asset.Asset]{Items: []asset.Asset{}, PageCount: "0"}, f.err
}
func (f *fakeScopes) ListVersions(_ context.Context, a, p string, o scopecommand.ListOptions) (scopecommand.Page[authorization.Version], error) {
	f.bind("versions", a, p, "")
	f.options = o
	return scopecommand.Page[authorization.Version]{Items: []authorization.Version{}, PageCount: "0"}, f.err
}
func (f *fakeScopes) GetVersion(_ context.Context, a, p, id string) (authorization.Version, error) {
	f.bind("version", a, p, id)
	return authorization.Version{}, f.err
}
func (f *fakeScopes) GetPolicy(_ context.Context, a, p string) (scopecommand.Policy, error) {
	f.bind("policy", a, p, "")
	return scopecommand.Policy{}, f.err
}
func (f *fakeScopes) ListPrechecks(_ context.Context, a, p string, o scopecommand.ListOptions) (scopecommand.Page[policy.Decision], error) {
	f.bind("prechecks", a, p, "")
	f.options = o
	return scopecommand.Page[policy.Decision]{Items: []policy.Decision{}, PageCount: "0"}, f.err
}
func (f *fakeScopes) GetPrecheck(_ context.Context, a, p, id string) (policy.Decision, error) {
	f.bind("precheck", a, p, id)
	return policy.Decision{}, f.err
}

func scopeHandler(t *testing.T, f *fakeScopes) (http.Handler, map[string]string) {
	t.Helper()
	i := &fakeIdentities{csrf: "固定合成会话绑定值", principal: identity.Principal{ActorID: testActorID}}
	h, e := NewHandler(testConfig(), &fakeCommands{}, i, slog.New(slog.NewTextHandler(io.Discard, nil)), f)
	if e != nil {
		t.Fatal(e)
	}
	return h, map[string]string{"Origin": "http://127.0.0.1:5173", "X-CSRF-Token": i.csrf}
}

// 实际路由注册、嵌套 chi 参数和 Cookie 身份必须传入正确服务方法。
func TestScopeRoutesBindSessionAndProject(t *testing.T) {
	for _, tc := range []struct{ path, called, object string }{
		{"assets", "assets", ""}, {"authorization-versions", "versions", ""}, {"prechecks", "prechecks", ""}, {"policy", "policy", ""},
		{"scope-commands/" + testCommandID, "command", testCommandID}, {"authorization-versions/" + testCommandID, "version", testCommandID}, {"prechecks/" + testCommandID, "precheck", testCommandID},
	} {
		t.Run(tc.called, func(t *testing.T) {
			f := &fakeScopes{}
			h, _ := scopeHandler(t, f)
			w := request(h, http.MethodGet, "/api/v1/projects/"+testProjectID+"/"+tc.path, "", nil, true)
			if w.Code != 200 || f.called != tc.called || f.actor != testActorID || f.project != testProjectID || f.object != tc.object {
				t.Fatalf("路由身份/对象绑定错误：%d %s", w.Code, w.Body)
			}
		})
	}
}

func TestScopeProtectedWriteBoundaries(t *testing.T) {
	path := "/api/v1/projects/" + testProjectID + "/scope-commands/" + testCommandID
	for _, tc := range []struct {
		name, body, contentType string
		authenticated, csrf     bool
		status                  int
	}{
		{"未登录", validScopeBody, "application/json", false, true, 401},
		{"无 CSRF", validScopeBody, "application/json", true, false, 403},
		{"错误媒体", validScopeBody, "text/plain", true, true, 415},
		{"正文上限", strings.Repeat("x", 256*1024+1), "application/json", true, true, 413},
		{"未知键", `{"kind":"import_assets","payload":{},"extra":true}`, "application/json", true, true, 400},
		{"重复键", `{"kind":"import_assets","kind":"precheck_target","payload":{}}`, "application/json", true, true, 400},
		{"大小写键", `{"Kind":"import_assets","payload":{}}`, "application/json", true, true, 400},
		{"空正文", `null`, "application/json", true, true, 400},
		{"尾随正文", validScopeBody + `{}`, "application/json", true, true, 400},
		{"嵌套未知键", strings.Replace(validScopeBody, `"notes":"中文来源"`, `"notes":"中文来源","actor_id":"伪造"`, 1), "application/json", true, true, 400},
		{"嵌套缺字段", strings.Replace(validScopeBody, `,"external_ref":"工单"`, "", 1), "application/json", true, true, 400},
		{"管理 kind", `{"kind":"activate_authorization_version","payload":{}}`, "application/json", true, true, 400},
		{"合法受理", validScopeBody, "application/json; charset=utf-8", true, true, 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeScopes{created: true}
			h, headers := scopeHandler(t, f)
			headers["Content-Type"] = tc.contentType
			if !tc.csrf {
				delete(headers, "X-CSRF-Token")
			}
			w := request(h, http.MethodPut, path, tc.body, headers, tc.authenticated)
			if w.Code != tc.status {
				t.Fatalf("预期=%d 实际=%d %s", tc.status, w.Code, w.Body)
			}
			if tc.status < 400 {
				return
			}
			var response APIError
			if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil || response.RequestID == "" || response.Message == "" || w.Header().Get("X-Request-ID") == "" {
				t.Fatal("失败需中文消息、稳定 envelope 与请求 ID")
			}
		})
	}
	for _, path := range []string{"/api/v1/projects/invalid/assets", "/api/v1/projects/" + testProjectID + "/authorization-versions/INVALID", "/api/v1/projects/" + testProjectID + "/scope-commands/invalid", "/api/v1/projects/" + testProjectID + "/prechecks/invalid"} {
		f := &fakeScopes{}
		h, _ := scopeHandler(t, f)
		w := request(h, http.MethodGet, path, "", nil, true)
		if w.Code != 400 || f.called != "" {
			t.Fatalf("无效路径 ID 不得进入服务：%d %s", w.Code, w.Body)
		}
	}
}

func TestScopeErrorsReplayAndPreview(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{scopecommand.ErrForbidden, 403, "forbidden"}, {scopecommand.ErrNotFound, 404, "not_found"}, {scopecommand.ErrConflict, 409, "command_conflict"}, {scopecommand.ErrRevisionConflict, 409, "policy_revision_conflict"}, {scopecommand.ErrUnavailable, 503, "unavailable"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			f := &fakeScopes{err: fmt.Errorf("%w: secret-sql-dsn", tc.err)}
			h, headers := scopeHandler(t, f)
			w := request(h, http.MethodPut, "/api/v1/projects/"+testProjectID+"/scope-commands/"+testCommandID, validScopeBody, headers, true)
			var response APIError
			_ = json.Unmarshal(w.Body.Bytes(), &response)
			if w.Code != tc.status || response.Code != tc.code || strings.Contains(w.Body.String(), "secret-sql-dsn") {
				t.Fatal("领域错误映射或脱敏失败")
			}
		})
	}
	f := &fakeScopes{created: false}
	h, headers := scopeHandler(t, f)
	w := request(h, http.MethodPut, "/api/v1/projects/"+testProjectID+"/scope-commands/"+testCommandID, validScopeBody, headers, true)
	if w.Code != 200 {
		t.Fatal("重放必须返回 200")
	}
	body := `{"entries":[{"kind":"domain","value":"good.invalid"},{"kind":"ip","value":"invalid"}],"source":"manual","notes":"","external_ref":""}`
	w = request(h, http.MethodPost, "/api/v1/projects/"+testProjectID+"/assets/preview", body, headers, true)
	var p asset.Preview
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if w.Code != 200 || p.Valid || len(p.Rows) != 2 || p.Rows[1].Row != 2 || p.Rows[1].Error == "" {
		t.Fatal("预览行错误是正常业务响应，需保留正确行号")
	}
}

func TestScopeListQueryLimits(t *testing.T) {
	for _, query := range []string{"page_size=0", "page_size=101", "page_size=1&page_size=2", "unknown=1", "page_size=x", "cursor=%zz", "search=a;kind=ip"} {
		f := &fakeScopes{}
		h, _ := scopeHandler(t, f)
		w := request(h, http.MethodGet, "/api/v1/projects/"+testProjectID+"/assets?"+query, "", nil, true)
		if w.Code != 400 || f.called != "" {
			t.Fatalf("列表必须拒绝歧义/超限参数：%s %d", query, w.Code)
		}
	}
	for _, resource := range []string{"authorization-versions", "prechecks"} {
		f := &fakeScopes{}
		h, _ := scopeHandler(t, f)
		w := request(h, http.MethodGet, "/api/v1/projects/"+testProjectID+"/"+resource+"?kind=ip", "", nil, true)
		if w.Code != 400 {
			t.Fatal("类型筛选仅资产列表支持")
		}
	}
	f := &fakeScopes{}
	h, _ := scopeHandler(t, f)
	w := request(h, http.MethodGet, "/api/v1/projects/"+testProjectID+"/assets?page_size=100&kind=domain&search=example.invalid&cursor=opaque", "", nil, true)
	if w.Code != 200 || f.options.PageSize != 100 || f.options.Kind != "domain" || f.options.Search != "example.invalid" || f.options.Cursor != "opaque" {
		t.Fatal("合法筛选/分页应完整传递服务")
	}
}

func TestScopeValidationDetailsAreControlled(t *testing.T) {
	f := &fakeScopes{err: fmt.Errorf("%w: %w", scopecommand.ErrInvalidRequest, validation.New("payload.entries.0.value", "CIDR 含主机位，请确认网络范围后使用 10.42.0.0/16"))}
	h, headers := scopeHandler(t, f)
	w := request(h, http.MethodPut, "/api/v1/projects/"+testProjectID+"/scope-commands/"+testCommandID, validScopeBody, headers, true)
	var response APIError
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	if w.Code != 400 || len(response.ValidationErrors) != 1 || response.ValidationErrors[0].Field != "payload.entries.0.value" || !strings.Contains(response.ValidationErrors[0].Message, "10.42.0.0/16") {
		t.Fatal("受控字段定位与CIDR建议必须到达HTTP")
	}
	f.err = fmt.Errorf("%w: SQL/DSN/private-secret", scopecommand.ErrInvalidRequest)
	w = request(h, http.MethodPut, "/api/v1/projects/"+testProjectID+"/scope-commands/"+testCommandID, validScopeBody, headers, true)
	if strings.Contains(w.Body.String(), "private-secret") || strings.Contains(w.Body.String(), "validation_errors") {
		t.Fatal("未分类底层原因不能公开或伪造字段说明")
	}
}
