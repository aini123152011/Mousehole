package identity

import (
	"bytes"
	"testing"
)

func TestDerivedKeysEncryptionAndCSRF(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, 32)
	keys, err := deriveKeys(master)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(keys.loginEncryption, keys.csrf) || bytes.Equal(keys.loginEncryption, master) {
		t.Fatal("域分离子密钥不能相同或复用主密钥")
	}
	ciphertext, err := encrypt(keys.loginEncryption, "pkce-verifier-secret")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("pkce-verifier-secret")) {
		t.Fatal("加密结果不应包含 verifier 明文")
	}
	plaintext, err := decrypt(keys.loginEncryption, ciphertext)
	if err != nil || plaintext != "pkce-verifier-secret" {
		t.Fatalf("加密往返失败：%q %v", plaintext, err)
	}
	first := csrfToken(keys.csrf, "session-a")
	if first == csrfToken(keys.csrf, "session-b") || !equalToken(first, csrfToken(keys.csrf, "session-a")) {
		t.Fatal("CSRF 必须稳定绑定单一会话")
	}
}

func TestValidAuthorizedParty(t *testing.T) {
	for _, test := range []struct {
		name     string
		audience []string
		azp      string
		want     bool
	}{
		{"单 audience", []string{"mousehole"}, "", true},
		{"单 audience 错误", []string{"other"}, "", false},
		{"显式 azp", []string{"mousehole"}, "mousehole", true},
		{"错误 azp", []string{"mousehole"}, "other", false},
		{"多 audience 缺 azp", []string{"mousehole", "api"}, "", false},
		{"多 audience 正确 azp", []string{"mousehole", "api"}, "mousehole", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validAuthorizedParty(test.audience, test.azp, "mousehole"); got != test.want {
				t.Fatalf("预期=%v，实际=%v", test.want, got)
			}
		})
	}
}

func TestCapabilitiesDefaultDeny(t *testing.T) {
	if got := CapabilitiesForRole(RoleViewer); len(got) != 1 || got[0] != CapabilityProjectRead {
		t.Fatalf("viewer 能力错误：%v", got)
	}
	if got := CapabilitiesForRole(RoleAnalyst); len(got) != 5 {
		t.Fatalf("analyst 能力错误：%v", got)
	}
	if got := CapabilitiesForRole("administrator"); len(got) != 0 {
		t.Fatalf("未知角色必须默认无能力：%v", got)
	}
}

func TestProvisionIssuerURLIsExactAndHasNoCredentials(t *testing.T) {
	for value, want := range map[string]bool{
		"https://idp.example/realms/mousehole": true,
		"http://127.0.0.1:9090":                true,
		"https://user@idp.example":             false,
		"https://idp.example?tenant=one":       false,
		"https://idp.example#fragment":         false,
		"/relative-issuer":                     false,
	} {
		if got := validIssuerURL(value); got != want {
			t.Fatalf("issuer=%q 预期=%v，实际=%v", value, want, got)
		}
	}
}
