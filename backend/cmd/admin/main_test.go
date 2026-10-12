package main

import (
	"bytes"
	"strings"
	"testing"

	"mousehole/backend/internal/identity"
)

func TestAdminCLIRejectsInvalidArgumentsBeforeDatabaseAccess(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "缺少命令"},
		{name: "未知命令", args: []string{"unknown"}},
		{name: "缺少数据库", args: []string{"revoke-sessions", "--user-id", "10000000-0000-4000-8000-000000000001"}},
		{name: "非标准 UUID", args: []string{"revoke-sessions", "--database-url", "postgres://unused", "--user-id", "10000000-0000-4000-8000-00000000000A"}},
		{name: "多余位置参数", args: []string{"disable-user", "--database-url", "postgres://unused", "--user-id", "10000000-0000-4000-8000-000000000001", "extra"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("退出码=%d，期望=%d", code, exitUsage)
			}
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("失败输出边界错误：stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestAdminCLIHelpUsesSuccessExitCode(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"revoke-membership", "--help"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitSuccess {
			t.Fatalf("帮助退出码=%d，期望=%d", code, exitSuccess)
		}
		if !strings.Contains(stdout.String(), "revoke-membership") || stderr.Len() != 0 {
			t.Fatalf("帮助输出错误：stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
}

func TestAdminCLIDoesNotEchoInvalidDSN(t *testing.T) {
	const invalidDSN = "postgres://secret-value@%"
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"revoke-sessions", "--database-url", invalidDSN,
		"--user-id", "10000000-0000-4000-8000-000000000001",
	}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("退出码=%d，期望=%d", code, exitUsage)
	}
	if strings.Contains(stdout.String()+stderr.String(), invalidDSN) || strings.Contains(stdout.String()+stderr.String(), "secret-value") {
		t.Fatal("公开错误不得回显数据库连接串")
	}
}

func TestMutationErrorExitCodesAreStable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "参数", err: identity.ErrInvalidRequest, code: exitUsage},
		{name: "不存在", err: identity.ErrAdminTargetNotFound, code: exitNotFound},
		{name: "冲突", err: identity.ErrConflict, code: exitConflict},
		{name: "数据库未确认", err: identity.ErrUnavailable, code: exitUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if result := mutationError(test.err); result.code != test.code {
				t.Fatalf("退出码=%d，期望=%d", result.code, test.code)
			}
		})
	}
}
