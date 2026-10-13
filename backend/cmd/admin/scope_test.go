package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mousehole/backend/internal/scopecommand"
)

const scopeTestID = "10000000-0000-4000-8000-000000000001"

// 管理范围显式选定、CAS 和严格正文在连接数据库前校验，秘密不得回显。
func TestScopeCLIUsageBoundaries(t *testing.T) {
	base := []string{"--database-url", "postgres://secret-value@%", "--command-id", scopeTestID, "--operator-ref", "合成管理"}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"缺范围", []string{"scope", "activate"}},
		{"两个范围", []string{"scope-command", "get", "--project-id", scopeTestID, "--global"}},
		{"负修订", []string{"scope", "activate", "--project-id", scopeTestID, "--version-id", scopeTestID, "--expected-policy-revision", "-1"}},
		{"非规范修订", []string{"scope", "activate", "--project-id", scopeTestID, "--version-id", scopeTestID, "--expected-policy-revision", "01"}},
		{"全局激活", []string{"scope", "activate", "--global", "--version-id", scopeTestID, "--expected-policy-revision", "1"}},
		{"未知动作", []string{"scope", "unknown", "--global", "--expected-policy-revision", "1"}},
		{"查询额外变更", []string{"scope-command", "get", "--global", "--expected-policy-revision", "1"}},
		{"合法项目范围无效DSN", []string{"scope-command", "get", "--project-id", scopeTestID}},
		{"合法全局范围无效DSN", []string{"scope-command", "get", "--global"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, err bytes.Buffer
			args := append(append([]string{}, tc.args...), base...)
			if code := run(args, &out, &err); code != exitUsage || out.Len() != 0 || err.Len() == 0 || strings.Contains(err.String(), "secret-value") {
				t.Fatalf("用法/秘密边界错误：%d %s", code, err.String())
			}
		})
	}
	for _, body := range []string{`{"kind":"ip","value":"10.42.0.1","include_subdomains":false,"reason":"合成禁止","extra":true}`, `{"kind":"ip","kind":"domain","value":"10.42.0.1","include_subdomains":false,"reason":"合成禁止"}`, `{"kind":"ip","value":"10.42.0.1","include_subdomains":false}`, strings.Repeat("x", 256*1024+1)} {
		file := filepath.Join(t.TempDir(), "deny.json")
		if e := os.WriteFile(file, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		args := append([]string{"scope", "deny-add", "--global", "--expected-policy-revision", "1", "--payload-file", file}, base...)
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != exitUsage || out.Len() != 0 || strings.Contains(err.String(), body) {
			t.Fatal("正文严格拒绝且不得回显")
		}
	}
	for _, args := range [][]string{{"scope", "--help"}, {"scope", "activate", "--help"}, {"scope-command", "get", "--help"}} {
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != exitSuccess || !strings.Contains(out.String(), "冻结回执 JSON") || err.Len() != 0 {
			t.Fatal("scope 帮助应说明恢复合同")
		}
	}
}

func TestScopeMutationExitCodes(t *testing.T) {
	for _, tc := range []struct {
		e    error
		code int
	}{{scopecommand.ErrInvalidRequest, exitUsage}, {scopecommand.ErrNotFound, exitNotFound}, {scopecommand.ErrConflict, exitConflict}, {scopecommand.ErrRevisionConflict, exitConflict}, {scopecommand.ErrUnavailable, exitUnavailable}} {
		if scopeMutationError(tc.e).code != tc.code {
			t.Fatal("范围管理错误退出码须稳定")
		}
	}
}
