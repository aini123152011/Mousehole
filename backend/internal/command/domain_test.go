package command

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeRequest(t *testing.T) {
	// 标题按 Unicode 字符而不是字节计数，规范化结果参与幂等绑定。
	for _, test := range []struct {
		name, title, kind string
		valid             bool
		normalized        string
	}{
		{"首尾空白", "\u3000 中文任务 \n", CreateTaskDraft, true, "中文任务"},
		{"最大中文长度", strings.Repeat("中", 120), CreateTaskDraft, true, strings.Repeat("中", 120)},
		{"超过长度", strings.Repeat("中", 121), CreateTaskDraft, false, ""},
		{"空标题", " \t", CreateTaskDraft, false, ""},
		{"未知命令", "任务", "execute", false, ""},
		{"非法编码", string([]byte{0xff}), CreateTaskDraft, false, ""},
		{"空字节", "a\x00b", CreateTaskDraft, false, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := NormalizeRequest(Request{Kind: test.kind, Payload: Payload{Title: test.title}})
			if test.valid {
				if err != nil || request.Payload.Title != test.normalized {
					t.Fatalf("规范化结果=%q，错误=%v", request.Payload.Title, err)
				}
			} else if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("预期参数错误，实际=%v", err)
			}
		})
	}
}

func TestValidateAccess(t *testing.T) {
	commandID := "11111111-1111-4111-8111-111111111111"
	for _, test := range []struct {
		name, actor, project, id string
		want                     error
	}{
		{"合法", DevelopmentActorID, DevelopmentProjectID, commandID, nil},
		{"其他身份", commandID, DevelopmentProjectID, commandID, ErrForbidden},
		{"其他项目", DevelopmentActorID, commandID, commandID, ErrForbidden},
		{"非法命令", DevelopmentActorID, DevelopmentProjectID, "invalid", ErrInvalidRequest},
		{"大写 UUID", DevelopmentActorID, DevelopmentProjectID, "AAAAAAAA-1111-4111-8111-111111111111", ErrInvalidRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateAccess(test.actor, test.project, test.id); !errors.Is(err, test.want) {
				t.Fatalf("预期=%v，实际=%v", test.want, err)
			}
		})
	}
}
