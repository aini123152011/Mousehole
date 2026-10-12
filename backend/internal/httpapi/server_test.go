package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mousehole/backend/internal/command"
)

const testCommandID = "11111111-1111-4111-8111-111111111111"
const validBody = `{"kind":"create_task_draft","payload":{"title":"中文任务"}}`

type fakeCommands struct {
	err     error
	calls   int
	created bool
	request command.Request
}

func (f *fakeCommands) Accept(_ context.Context, actor, project, id string, request command.Request) (command.Receipt, bool, error) {
	f.calls++
	f.request = request
	return command.Receipt{CommandID: id, ProjectID: project, Kind: request.Kind, State: "accepted", TaskID: testCommandID, TaskTitle: request.Payload.Title, TaskState: "draft", Version: "1", AcceptedAt: "2026-10-12T00:00:00Z"}, f.created, f.err
}
func (f *fakeCommands) Get(_ context.Context, _, _, _ string) (command.Receipt, error) {
	f.calls++
	return command.Receipt{}, f.err
}

func testHandler(t *testing.T, commands *fakeCommands) http.Handler {
	t.Helper()
	handler, err := NewHandler(Config{Mode: "development", ListenAddr: "127.0.0.1:8080", WebOrigin: "http://127.0.0.1:5173"}, commands, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func request(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Mousehole-Development", "true")
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

func TestAdmissionBoundary(t *testing.T) {
	path := "/api/v1/projects/" + command.DevelopmentProjectID + "/commands/" + testCommandID
	for _, test := range []struct {
		name, body string
		headers    map[string]string
		status     int
	}{
		{"合法中文", validBody, nil, 201},
		{"缺开发标记", validBody, map[string]string{"X-Mousehole-Development": ""}, 403},
		{"伪造来源", validBody, map[string]string{"Origin": "https://evil.example"}, 403},
		{"DNS 重绑定 Host", validBody, map[string]string{"Host": "evil.example:8080"}, 403},
		{"非 JSON", validBody, map[string]string{"Content-Type": "text/plain"}, 415},
		{"未知字段", `{"kind":"create_task_draft","payload":{"title":"任务","extra":true}}`, nil, 400},
		{"重复字段", `{"kind":"create_task_draft","kind":"create_task_draft","payload":{"title":"任务"}}`, nil, 400},
		{"重复标题", `{"kind":"create_task_draft","payload":{"title":"第一","title":"第二"}}`, nil, 400},
		{"错误大小写", `{"Kind":"create_task_draft","payload":{"title":"任务"}}`, nil, 400},
		{"多份正文", validBody + validBody, nil, 400},
		{"缺标题", `{"kind":"create_task_draft","payload":{}}`, nil, 400},
		{"null 正文", `null`, nil, 400},
		{"null 标题", `{"kind":"create_task_draft","payload":{"title":null}}`, nil, 400},
		{"空标题", `{"kind":"create_task_draft","payload":{"title":"  "}}`, nil, 400},
		{"未知命令", `{"kind":"execute","payload":{"title":"任务"}}`, nil, 400},
		{"超大正文", strings.Repeat("x", 16*1024+1), nil, 413},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands := &fakeCommands{created: true}
			w := request(testHandler(t, commands), http.MethodPut, path, test.body, test.headers)
			if w.Code != test.status {
				t.Fatalf("预期HTTP=%d，实际=%d，正文=%s", test.status, w.Code, w.Body)
			}
			if test.status != 201 {
				if commands.calls != 0 {
					t.Fatalf("边界拒绝仍调用业务服务 %d 次", commands.calls)
				}
				var failure APIError
				if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil || failure.Code == "" || failure.Message == "" || !command.ValidID(failure.RequestID) {
					t.Fatalf("错误合同无效：%s", w.Body)
				}
			}
		})
	}
}

func TestReceiptStatusesAndProjectBoundary(t *testing.T) {
	path := "/api/v1/projects/" + command.DevelopmentProjectID + "/commands/" + testCommandID
	for _, test := range []struct {
		name, method, path string
		err                error
		created            bool
		status             int
		code               string
	}{
		{"首次受理", "PUT", path, nil, true, 201, ""},
		{"回放", "PUT", path, nil, false, 200, ""},
		{"参数冲突", "PUT", path, command.ErrConflict, false, 409, "idempotency_conflict"},
		{"未知受理", "PUT", path, command.ErrUnavailable, false, 503, "service_unavailable"},
		{"待确认", "GET", path, command.ErrNotFound, false, 404, "command_not_found"},
		{"项目拒绝", "GET", "/api/v1/projects/" + testCommandID + "/commands/" + testCommandID, nil, false, 403, "project_forbidden"},
		{"UUID 拒绝", "GET", "/api/v1/projects/invalid/commands/" + testCommandID, nil, false, 400, "invalid_request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands := &fakeCommands{err: test.err, created: test.created}
			w := request(testHandler(t, commands), test.method, test.path, validBody, nil)
			if w.Code != test.status {
				t.Fatalf("预期=%d，实际=%d，正文=%s", test.status, w.Code, w.Body)
			}
			if test.code != "" {
				var failure APIError
				_ = json.Unmarshal(w.Body.Bytes(), &failure)
				if failure.Code != test.code {
					t.Fatalf("预期错误码=%s，实际=%s", test.code, failure.Code)
				}
			}
			if strings.Contains(test.name, "拒绝") && commands.calls != 0 {
				t.Fatal("跨项目或非法 ID 不应进入存储")
			}
		})
	}
}

func TestSessionAndPreflight(t *testing.T) {
	handler := testHandler(t, &fakeCommands{})
	w := request(handler, "GET", "/api/v1/session", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), command.DevelopmentActorID) || !strings.Contains(w.Body.String(), "本地演示项目") {
		t.Fatalf("身份合同无效：%s", w.Body)
	}
	w = request(handler, "OPTIONS", "/api/v1/session", "", map[string]string{"Origin": "http://127.0.0.1:5173", "Access-Control-Request-Method": "GET", "Access-Control-Request-Headers": "X-Mousehole-Development"})
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:5173" {
		t.Fatalf("合法预检失败：%d", w.Code)
	}
	w = request(handler, "OPTIONS", "/api/v1/session", "", map[string]string{"Origin": "http://127.0.0.1:5173", "Access-Control-Request-Method": "POST"})
	if w.Code != 403 {
		t.Fatalf("未知方法预检应拒绝，实际=%d", w.Code)
	}
}

func TestConfigRequiresDevelopmentAndLoopback(t *testing.T) {
	valid := Config{Mode: "development", ListenAddr: "127.0.0.1:8080", WebOrigin: "http://127.0.0.1:5173"}
	for _, test := range []struct {
		name   string
		change func(*Config)
	}{
		{"缺模式", func(c *Config) { c.Mode = "" }},
		{"生产模式", func(c *Config) { c.Mode = "production" }},
		{"通配监听", func(c *Config) { c.ListenAddr = "0.0.0.0:8080" }},
		{"远端监听", func(c *Config) { c.ListenAddr = "192.168.1.1:8080" }},
		{"域名监听", func(c *Config) { c.ListenAddr = "localhost:8080" }},
		{"远端来源", func(c *Config) { c.WebOrigin = "http://192.168.1.1:5173" }},
		{"来源路径", func(c *Config) { c.WebOrigin = "http://127.0.0.1:5173/" }},
		{"来源凭据", func(c *Config) { c.WebOrigin = "http://user@127.0.0.1:5173" }},
		{"来源查询", func(c *Config) { c.WebOrigin = "http://127.0.0.1:5173?key=value" }},
		{"来源空查询", func(c *Config) { c.WebOrigin = "http://127.0.0.1:5173?" }},
		{"来源片段", func(c *Config) { c.WebOrigin = "http://127.0.0.1:5173#section" }},
		{"来源空片段", func(c *Config) { c.WebOrigin = "http://127.0.0.1:5173#" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.change(&cfg)
			if cfg.Validate() == nil {
				t.Fatal("配置应被默认拒绝")
			}
		})
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
}
