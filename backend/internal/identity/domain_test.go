package identity

import (
	"strings"
	"testing"
)

func TestProvisionProfileFieldsMatchSessionSchema(t *testing.T) {
	valid := ProvisionRequest{
		Issuer: "https://idp.example/realms/mousehole", Subject: "subject", UserID: "10000000-0000-4000-8000-000000000001",
		DisplayName: "测试分析员", Email: "analyst@example.invalid", ProjectID: "10000000-0000-4000-8000-000000000002",
		ProjectName: "测试项目", Role: RoleAnalyst,
	}
	if !validProvision(valid) {
		t.Fatal("合法预置资料应通过领域校验")
	}
	for name, change := range map[string]func(*ProvisionRequest){
		"显示名超长":    func(request *ProvisionRequest) { request.DisplayName = strings.Repeat("名", 121) },
		"项目名含 NUL": func(request *ProvisionRequest) { request.ProjectName = "项目\x00名称" },
		"邮箱含显示名称":  func(request *ProvisionRequest) { request.Email = "分析员 <analyst@example.invalid>" },
		"邮箱格式无效":   func(request *ProvisionRequest) { request.Email = "invalid-email" },
	} {
		t.Run(name, func(t *testing.T) {
			request := valid
			change(&request)
			if validProvision(request) {
				t.Fatal("不符合 session schema 的资料必须在数据库操作前拒绝")
			}
		})
	}
}
