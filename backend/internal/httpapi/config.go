package httpapi

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config 是本地开发入口配置；任何生产或非 loopback 配置都在启动前拒绝。
type Config struct {
	Mode       string
	ListenAddr string // 只接受明确的 loopback IP 与非零端口，不接受通配地址或域名。
	WebOrigin  string // 只允许一个完整的 HTTP loopback 来源，不支持通配符。
}

// LoadConfig 从明确的环境变量加载配置；模式没有默认值，必须主动选择 development。
func LoadConfig() (Config, error) {
	cfg := Config{Mode: os.Getenv("MOUSEHOLE_MODE"), ListenAddr: os.Getenv("MOUSEHOLE_LISTEN_ADDR"), WebOrigin: os.Getenv("MOUSEHOLE_WEB_ORIGIN")}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:8080"
	}
	if cfg.WebOrigin == "" {
		cfg.WebOrigin = "http://127.0.0.1:5173"
	}
	return cfg, cfg.Validate()
}

// Validate 拒绝生产模式、非本机地址及含路径、查询、片段或凭据的来源配置。
func (cfg Config) Validate() error {
	if cfg.Mode != "development" {
		return errors.New("必须显式设置 MOUSEHOLE_MODE=development")
	}
	host, port, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil || !loopbackIP(host) || !validPort(port) {
		return errors.New("监听地址必须是 loopback IP 和有效端口")
	}
	origin, err := url.Parse(cfg.WebOrigin)
	// url.Parse 会丢弃空片段分隔符；原字符串也须检查，避免接受浏览器不会发送的来源。
	if err != nil || origin.Scheme != "http" || !loopbackIP(origin.Hostname()) || !validPort(origin.Port()) || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || strings.Contains(cfg.WebOrigin, "#") || origin.ForceQuery || origin.Opaque != "" {
		return errors.New("页面来源必须是明确的 HTTP loopback IP 和端口，且不含路径、查询、片段或凭据")
	}
	return nil
}

func loopbackIP(host string) bool { ip := net.ParseIP(host); return ip != nil && ip.IsLoopback() }
func validPort(port string) bool {
	number, err := strconv.Atoi(port)
	return err == nil && number > 0 && number <= 65535
}
