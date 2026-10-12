package testoidc

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizationCodePKCEIsOneTime(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	issuer, err := New(Config{Issuer: server.URL, ClientID: "mousehole", ClientSecret: "secret", Subject: "analyst"})
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = issuer.Handler()
	verifier := "0123456789012345678901234567890123456789012"
	digest := sha256.Sum256([]byte(verifier))
	authorize := server.URL + "/authorize?" + url.Values{
		"client_id": {"mousehole"}, "response_type": {"code"}, "redirect_uri": {"http://127.0.0.1:8080/callback"},
		"state": {"state"}, "nonce": {"nonce"}, "code_challenge_method": {"S256"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])},
	}.Encode()
	client := *server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Get(authorize)
	if err != nil {
		t.Fatal("合成授权端点请求失败")
	}
	location, _ := url.Parse(response.Header.Get("Location"))
	code := location.Query().Get("code")
	if response.StatusCode != http.StatusFound || code == "" || location.Query().Get("state") != "state" {
		t.Fatalf("授权响应无效：status=%d has_code=%t state_matches=%t", response.StatusCode, code != "", location.Query().Get("state") == "state")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"http://127.0.0.1:8080/callback"}, "code_verifier": {verifier}}
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth("mousehole", "secret")
	tokenResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var token map[string]any
	if json.NewDecoder(tokenResponse.Body).Decode(&token) != nil || tokenResponse.StatusCode != http.StatusOK || token["id_token"] == "" {
		t.Fatalf("令牌响应无效：status=%d has_id_token=%t", tokenResponse.StatusCode, token["id_token"] != "")
	}
	replay, _ := http.NewRequest(http.MethodPost, server.URL+"/token", strings.NewReader(form.Encode()))
	replay.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	replay.SetBasicAuth("mousehole", "secret")
	replayResponse, err := client.Do(replay)
	if err != nil || replayResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("授权码重放必须拒绝：%v %d", err, replayResponse.StatusCode)
	}
}
