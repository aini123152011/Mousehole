package scopecommand

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/validation"
	"strings"
	"testing"
)

func TestStrictScopeJSONAndPayload(t *testing.T) {
	for _, body := range []string{`{"kind":"import_assets","kind":"precheck_target","payload":{}}`, `{"Kind":"import_assets","payload":{}}`, `{"kind":"import_assets","payload":{},"extra":true}`, `{"kind":"import_assets"}`, `{"kind":"import_assets","payload":null}`, `{"kind":"import_assets","payload":{}} {}`} {
		var r Request
		if DecodeJSON([]byte(body), &r) == nil {
			t.Fatalf("歧义结构须拒绝：%s", body)
		}
	}
	for _, payload := range []string{`{"entries":[],"source":"manual","notes":"","external_ref":""}`, `{"entries":[{"kind":"domain","value":"example.invalid","Value":"other.invalid"}],"source":"manual","notes":"","external_ref":""}`, `{"entries":[{"kind":"domain","value":"example.invalid"}],"source":"manual","notes":"","external_ref":"","actor_id":"伪造"}`} {
		if _, e := NormalizeRequest(Request{"import_assets", json.RawMessage(payload)}); e == nil {
			t.Fatal("固定导入结构须拒绝无效字段或空批次")
		}
	}
}

func TestNormalizationFieldDetailsAndPrivacy(t *testing.T) {
	c := authorization.Content{Title: "合成授权", BasisRef: "工单", Contact: "合成联系人", ValidFrom: "2026-01-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z", AlgorithmVersion: "target-v1", PolicyVersion: "policy-v1", CatalogVersion: "precheck-v1", MaxOperationLevel: "non_destructive", Budget: authorization.Budget{MaxRequests: "100", MaxConcurrency: "2", MaxRequestsPerSecond: "5"}, RedirectPolicy: "deny", CredentialPolicy: "none", Entries: []authorization.Entry{{Kind: "cidr", Value: "10.42.0.9/16", Operations: []string{"tcp_connect"}, Ports: []int32{443}, ResolvedIPCIDRs: []string{}}}}
	for _, tc := range []struct {
		field      string
		change     func(*authorization.Content)
		suggestion string
	}{
		{"payload.entries.0.value", func(*authorization.Content) {}, "10.42.0.0/16"},
		{"payload.budget.max_requests", func(c *authorization.Content) { c.Budget.MaxRequests = "0" }, "正整数"},
		{"payload.entries.0.value", func(c *authorization.Content) {
			c.Entries[0].Kind = "url_exact"
			c.Entries[0].Value = "https://example.invalid/path%2Fbad?private-secret=hidden"
		}, "编码的分隔符"},
	} {
		t.Run(tc.field+tc.suggestion, func(t *testing.T) {
			copy := c
			copy.Entries = append([]authorization.Entry(nil), c.Entries...)
			tc.change(&copy)
			body, _ := json.Marshal(copy)
			_, e := NormalizeRequest(Request{Kind: "create_authorization_version", Payload: body})
			details := validation.Details(e)
			if !errors.Is(e, ErrInvalidRequest) || len(details) != 1 || details[0].Field != tc.field || !strings.Contains(details[0].Message, tc.suggestion) || strings.Contains(details[0].Message, "private-secret") {
				t.Fatalf("规范化定位/受控说明错误：%+v", details)
			}
		})
	}
	if validation.Details(errors.New("SQL/DSN/private-secret")) != nil {
		t.Fatal("外部错误不得变成公开字段说明")
	}
}
func TestCursorIntegrityAndSearchPrivacy(t *testing.T) {
	s := NewService(nil, []byte("合成签名密钥"))
	o := ListOptions{PageSize: 1, Search: "https://example.invalid/?secret=value"}
	cursor := s.cursor("项目", "assets", o, "2026-10-12T00:00:00Z", "10000000-0000-4000-8000-000000000001")
	raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(cursor, ".")[0])
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "example.invalid") {
		t.Fatal("cursor 不得携带搜索目标原文")
	}
	o.Cursor = cursor
	if _, _, e := s.parseCursor("项目", "assets", o); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.parseCursor("另一项目", "assets", o); e == nil {
		t.Fatal("跨项目 cursor 必须拒绝")
	}
	o.Search = "另一搜索"
	if _, _, e := s.parseCursor("项目", "assets", o); e == nil {
		t.Fatal("不同筛选 cursor 必须拒绝")
	}
	o.Search = "https://example.invalid/?secret=value"
	parts := strings.Split(cursor, ".")
	parts[0] = base64.RawURLEncoding.EncodeToString(append(raw, ' '))
	o.Cursor = strings.Join(parts, ".")
	if _, _, e := s.parseCursor("项目", "assets", o); e == nil {
		t.Fatal("篡改 cursor 必须拒绝")
	}
}
