package policy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"mousehole/backend/internal/authorization"
)

type rejectTransport struct{ calls *atomic.Int32 }

func (t rejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, errors.New("夹具禁止目标 HTTP")
}

func TestStaticPolicyUnderRejectNetworkFixture(t *testing.T) {
	// 非并行夹具拒绝目标 DNS/HTTP，并在退出时恢复全局默认对象；PostgreSQL 不在此纯函数测试中。
	var dns, httpCalls atomic.Int32
	oldResolver, oldTransport := net.DefaultResolver, http.DefaultTransport
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		dns.Add(1)
		return nil, errors.New("夹具禁止目标 DNS")
	}}
	http.DefaultTransport = rejectTransport{&httpCalls}
	defer func() { net.DefaultResolver = oldResolver; http.DefaultTransport = oldTransport }()
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	port := 443
	content := authorization.Content{ValidFrom: "2026-01-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z", Entries: []authorization.Entry{{Kind: "domain", Value: "example.invalid", IncludeSubdomains: true, Operations: []string{"tcp_connect"}, Ports: []int32{443}, ResolvedIPCIDRs: []string{"10.42.0.0/16"}}, {Kind: "cidr", Value: "10.42.0.0/16", Operations: []string{"tcp_connect"}, Ports: []int32{443}, ResolvedIPCIDRs: []string{}}}}
	v := authorization.Version{ID: "版本", State: "active", Content: content, EntryIDs: []string{"域名条目", "私网条目"}}
	active := v.ID
	for _, tc := range []struct{ target, outcome string }{{"a.example.invalid", "requires_runtime_checks"}, {"badexample.invalid", "deny"}, {"10.42.0.9", "requires_runtime_checks"}, {"10.43.0.1", "deny"}, {"127.0.0.1", "deny"}, {"::ffff:169.254.169.254", "deny"}} {
		r, e := Normalize(Request{Operation: "tcp_connect", Target: tc.target, Parameters: Parameters{Port: &port}})
		if e != nil {
			t.Fatal(e)
		}
		d := Evaluate(r, v, &active, nil, now)
		if d.Outcome != tc.outcome || len(d.RuntimeChecks) < 8 {
			t.Fatalf("静态反例 %s 预期 %s 实际 %+v", tc.target, tc.outcome, d)
		}
	}
	deny := authorization.Deny{ID: "当前禁止", Kind: "domain", Value: "example.invalid", IncludeSubdomains: true, Active: true}
	d := Evaluate(Request{Operation: "tcp_connect", Target: "a.example.invalid", Parameters: Parameters{Port: &port}}, v, &active, []authorization.Deny{deny}, now)
	if d.Outcome != "deny" || d.ReasonCodes[0] != "scope_denied" {
		t.Fatal("当前禁止须优先于正向规则")
	}
	for _, state := range []string{"candidate", "superseded", "revoked"} {
		v.State = state
		d = Evaluate(Request{Operation: "tcp_connect", Target: "10.42.0.9", Parameters: Parameters{Port: &port}}, v, &active, nil, now)
		if d.Outcome != "deny" {
			t.Fatal("非当前生效版本必须默认拒绝")
		}
	}
	if dns.Load() != 0 || httpCalls.Load() != 0 {
		t.Fatal("预检查不得调用目标网络")
	}
}
func TestOperationCatalogDefaultDeny(t *testing.T) {
	port := 443
	for _, r := range []Request{{Operation: "scan", Target: "example.invalid"}, {Operation: "dns_lookup", Target: "example.invalid", Parameters: Parameters{Port: &port}}, {Operation: "tcp_connect", Target: "example.invalid"}, {Operation: "http_metadata", Target: "https://example.invalid/", Parameters: Parameters{Port: &port}}} {
		if _, e := Normalize(r); e == nil {
			t.Fatal("未知目录或额外参数须受理前拒绝")
		}
	}
}

// active 仍需检查左闭右开时窗；展示状态本身不能替代实际决策验收。
func TestEvaluateAuthorizationTimeBoundaries(t *testing.T) {
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	expires := from.Add(time.Hour)
	port := 443
	v := authorization.Version{ID: "版本", State: "active", EntryIDs: []string{"范围条目"}, Content: authorization.Content{ValidFrom: from.Format(time.RFC3339Nano), ExpiresAt: expires.Format(time.RFC3339Nano), Entries: []authorization.Entry{{Kind: "cidr", Value: "10.42.0.0/16", Operations: []string{"tcp_connect"}, Ports: []int32{443}}}}}
	for _, tc := range []struct {
		name            string
		at              time.Time
		outcome, reason string
		matched         bool
	}{
		{"开始前", from.Add(-time.Nanosecond), "deny", "version_not_yet_valid", false},
		{"恰好开始", from, "requires_runtime_checks", "static_scope_matched", true},
		{"结束前", expires.Add(-time.Nanosecond), "requires_runtime_checks", "static_scope_matched", true},
		{"恰好结束", expires, "deny", "version_expired", false},
		{"结束后", expires.Add(time.Nanosecond), "deny", "version_expired", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Evaluate(Request{Operation: "tcp_connect", Target: "10.42.0.9", Parameters: Parameters{Port: &port}}, v, &v.ID, nil, tc.at)
			if d.Outcome != tc.outcome || len(d.ReasonCodes) != 1 || d.ReasonCodes[0] != tc.reason || (d.MatchedEntryID != nil) != tc.matched || d.CheckedAt != tc.at.Format(time.RFC3339Nano) {
				t.Fatalf("时窗决策错误：%+v", d)
			}
		})
	}
}
