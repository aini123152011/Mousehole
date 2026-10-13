package authorization

import (
	"encoding/json"
	"testing"
	"time"
)

func sampleContent() Content {
	return Content{Title: "合成授权", BasisRef: "工单-1", Contact: "测试联系人", ValidFrom: "2026-01-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z", AlgorithmVersion: "target-v1", PolicyVersion: "policy-v1", CatalogVersion: "precheck-v1", MaxOperationLevel: "non_destructive", Budget: Budget{"100", "2", "5"}, RedirectPolicy: "deny", CredentialPolicy: "none", Entries: []Entry{{Kind: "domain", Value: "EXAMPLE.COM.", IncludeSubdomains: true, Operations: []string{"tcp_connect", "dns_lookup"}, Ports: []int32{443, 80, 443}, ResolvedIPCIDRs: []string{"10.42.0.0/16"}}}}
}

func TestContentCanonicalHashAndCompleteBinding(t *testing.T) {
	// 同语义集合重排相同 hash；每个授权语义字段改变都不能沿用原摘要。
	original, e := NormalizeContent(sampleContent())
	if e != nil {
		t.Fatal(e)
	}
	same := sampleContent()
	same.Entries[0].Operations = []string{"dns_lookup", "tcp_connect"}
	same.Entries[0].Ports = []int32{80, 443}
	same, e = NormalizeContent(same)
	if e != nil || Hash(original) != Hash(same) {
		t.Fatalf("规范集合应得到相同摘要：%v", e)
	}
	for name, change := range map[string]func(*Content){"标题": func(c *Content) { c.Title = "另一授权" }, "依据": func(c *Content) { c.BasisRef = "工单-2" }, "联系人": func(c *Content) { c.Contact = "另一联系人" }, "时窗": func(c *Content) { c.ExpiresAt = "2028-01-01T00:00:00Z" }, "预算": func(c *Content) { c.Budget.MaxRequests = "101" }, "范围": func(c *Content) { c.Entries[0].Value = "other.example.com" }, "子域": func(c *Content) { c.Entries[0].IncludeSubdomains = false }, "端口": func(c *Content) { c.Entries[0].Ports = []int32{443} }, "解析约束": func(c *Content) { c.Entries[0].ResolvedIPCIDRs = []string{"10.43.0.0/16"} }} {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(original)
			var changed Content
			_ = json.Unmarshal(data, &changed)
			change(&changed)
			changed, e = NormalizeContent(changed)
			if e != nil || Hash(changed) == Hash(original) {
				t.Fatalf("语义变更必须重新绑定：%v", e)
			}
		})
	}
}
func TestContentRejectsIncompleteScopeAndUnsupportedVersions(t *testing.T) {
	for name, change := range map[string]func(*Content){"未知版本": func(c *Content) { c.PolicyVersion = "policy-v2" }, "超预算": func(c *Content) { c.Budget.MaxConcurrency = "101" }, "缺解析": func(c *Content) { c.Entries[0].ResolvedIPCIDRs = []string{} }, "公共后缀": func(c *Content) { c.Entries[0].Value = "github.io" }, "HTTP非精确URL": func(c *Content) { c.Entries[0].Operations = []string{"http_metadata"} }, "未知操作": func(c *Content) { c.Entries[0].Operations = []string{"scan"} }, "非法端口": func(c *Content) { c.Entries[0].Ports = []int32{65536} }, "反时窗": func(c *Content) { c.ExpiresAt = c.ValidFrom }, "重定向": func(c *Content) { c.RedirectPolicy = "allow" }} {
		t.Run(name, func(t *testing.T) {
			c := sampleContent()
			change(&c)
			if _, e := NormalizeContent(c); e == nil {
				t.Fatal("不完整或未知授权须拒绝")
			}
		})
	}
}
func TestDisplayStateTimeBoundaries(t *testing.T) {
	c := sampleContent()
	for _, tc := range []struct{ at, state, want string }{{c.ValidFrom, "active", "active"}, {c.ExpiresAt, "active", "expired"}, {"2025-12-31T23:59:59Z", "candidate", "not_yet_valid"}, {c.ExpiresAt, "revoked", "revoked"}, {c.ExpiresAt, "superseded", "superseded"}} {
		at, _ := time.Parse(time.RFC3339, tc.at)
		if got := DisplayState(tc.state, c, at); got != tc.want {
			t.Fatalf("时窗边界预期 %s 实际 %s", tc.want, got)
		}
	}
}
