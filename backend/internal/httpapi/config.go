package httpapi

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"mousehole/backend/internal/identity"
)

const (
	developmentSessionCookie = "mousehole_session"
	developmentLoginCookie   = "mousehole_login_binding"
	productionSessionCookie  = "__Host-mousehole_session"
	productionLoginCookie    = "__Host-mousehole_login_binding"
)

// Config 描述 HTTP、OIDC 和会话部署边界；秘密只保存在进程内存中。
type Config struct {
	Mode               string
	ListenAddr         string
	PublicURL          string
	WebOrigin          string
	OIDCIssuer         string
	OIDCClientID       string
	OIDCClientSecret   string
	AuthKey            []byte
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
}

// LoadConfig 从明确环境变量加载配置；不会自动读取 .env 文件。
func LoadConfig() (Config, error) {
	idleTTL, err := durationEnv("MOUSEHOLE_SESSION_IDLE_TTL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	absoluteTTL, err := durationEnv("MOUSEHOLE_SESSION_ABSOLUTE_TTL", 8*time.Hour)
	if err != nil {
		return Config{}, err
	}
	authKey, err := base64.StdEncoding.DecodeString(os.Getenv("MOUSEHOLE_AUTH_KEY"))
	if err != nil {
		return Config{}, errors.New("MOUSEHOLE_AUTH_KEY 必须是标准 base64")
	}
	cfg := Config{
		Mode: os.Getenv("MOUSEHOLE_MODE"), ListenAddr: os.Getenv("MOUSEHOLE_LISTEN_ADDR"),
		PublicURL: os.Getenv("MOUSEHOLE_PUBLIC_URL"), WebOrigin: os.Getenv("MOUSEHOLE_WEB_ORIGIN"),
		OIDCIssuer: os.Getenv("MOUSEHOLE_OIDC_ISSUER"), OIDCClientID: os.Getenv("MOUSEHOLE_OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("MOUSEHOLE_OIDC_CLIENT_SECRET"), AuthKey: authKey,
		SessionIdleTTL: idleTTL, SessionAbsoluteTTL: absoluteTTL,
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:8080"
	}
	if cfg.PublicURL == "" && cfg.Mode == "development" {
		cfg.PublicURL = "http://" + cfg.ListenAddr
	}
	if cfg.WebOrigin == "" && cfg.Mode == "development" {
		cfg.WebOrigin = "http://127.0.0.1:5173"
	}
	return cfg, cfg.Validate()
}

// Validate 对生产和开发采用各自的明确来源、TLS 与 Cookie 前置条件。
func (cfg Config) Validate() error {
	if cfg.Mode != "development" && cfg.Mode != "production" {
		return errors.New("MOUSEHOLE_MODE 必须显式设置为 development 或 production")
	}
	listenHost, port, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil || !validPort(port) {
		return errors.New("监听地址必须包含有效主机和端口")
	}
	if cfg.Mode == "development" && !loopbackIP(listenHost) {
		return errors.New("开发模式监听地址必须是 loopback IP")
	}
	publicURL, err := strictOrigin(cfg.PublicURL)
	if err != nil {
		return errors.New("MOUSEHOLE_PUBLIC_URL 必须是不含路径、查询、片段或凭据的完整来源")
	}
	webURL, err := strictOrigin(cfg.WebOrigin)
	if err != nil {
		return errors.New("MOUSEHOLE_WEB_ORIGIN 必须是不含路径、查询、片段或凭据的完整来源")
	}
	issuer, err := strictIssuer(cfg.OIDCIssuer)
	if err != nil {
		return errors.New("MOUSEHOLE_OIDC_ISSUER 必须是不含查询、片段或凭据的完整 issuer URL")
	}
	if cfg.Mode == "development" {
		if publicURL.Scheme != "http" || webURL.Scheme != "http" || issuer.Scheme != "http" ||
			!loopbackIP(publicURL.Hostname()) || !loopbackIP(webURL.Hostname()) || !loopbackIP(issuer.Hostname()) {
			return errors.New("开发模式公共地址、页面来源和 OIDC issuer 必须是精确 HTTP loopback 来源")
		}
	} else if publicURL.Scheme != "https" || webURL.Scheme != "https" || issuer.Scheme != "https" {
		return errors.New("生产模式公共地址、页面来源和 OIDC issuer 必须使用 HTTPS")
	}
	if cfg.OIDCClientID == "" || cfg.OIDCClientSecret == "" {
		return errors.New("必须设置 OIDC client ID 和 client secret")
	}
	if len(cfg.AuthKey) != 32 {
		return errors.New("MOUSEHOLE_AUTH_KEY 解码后必须正好为 32 字节")
	}
	if cfg.SessionIdleTTL < 5*time.Minute || cfg.SessionIdleTTL > 24*time.Hour ||
		cfg.SessionAbsoluteTTL < cfg.SessionIdleTTL || cfg.SessionAbsoluteTTL > 7*24*time.Hour {
		return errors.New("会话空闲有效期必须为 5 分钟至 24 小时，绝对有效期不得更短且最多 7 天")
	}
	return nil
}

// IdentityConfig 返回身份服务配置，回调地址固定在当前公共 API 来源下。
func (cfg Config) IdentityConfig() identity.Config {
	return identity.Config{
		Issuer: cfg.OIDCIssuer, ClientID: cfg.OIDCClientID, ClientSecret: cfg.OIDCClientSecret,
		RedirectURL: cfg.PublicURL + "/api/v1/auth/callback", AuthKey: cfg.AuthKey,
		SessionIdleTTL: cfg.SessionIdleTTL, SessionAbsoluteTTL: cfg.SessionAbsoluteTTL,
	}
}

func (cfg Config) sessionCookieName() string {
	if cfg.Mode == "production" {
		return productionSessionCookie
	}
	return developmentSessionCookie
}

func (cfg Config) loginCookieName() string {
	if cfg.Mode == "production" {
		return productionLoginCookie
	}
	return developmentLoginCookie
}

func (cfg Config) secureCookies() bool { return cfg.Mode == "production" }

func (cfg Config) publicHost() string {
	parsed, _ := url.Parse(cfg.PublicURL)
	return parsed.Host
}

func strictOrigin(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" ||
		strings.Contains(value, "#") || strings.HasSuffix(value, "?") || (parsed.Port() != "" && !validPort(parsed.Port())) ||
		(parsed.Scheme == "http" && parsed.Port() == "80") || (parsed.Scheme == "https" && parsed.Port() == "443") {
		return nil, errors.New("来源无效")
	}
	return parsed, nil
}

func strictIssuer(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" ||
		strings.Contains(value, "#") || strings.HasSuffix(value, "?") ||
		(parsed.Port() != "" && !validPort(parsed.Port())) {
		return nil, errors.New("issuer 无效")
	}
	return parsed, nil
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, errors.New(name + " 必须是 Go duration，例如 30m")
	}
	return parsed, nil
}

func loopbackIP(host string) bool { ip := net.ParseIP(host); return ip != nil && ip.IsLoopback() }
func validPort(port string) bool {
	number, err := strconv.Atoi(port)
	return err == nil && number > 0 && number <= 65535
}
