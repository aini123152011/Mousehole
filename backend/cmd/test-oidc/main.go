// Command test-oidc 启动本地开发与 E2E 专用的合成 OIDC issuer。
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"mousehole/backend/internal/testoidc"
)

func main() {
	address := value("MOUSEHOLE_TEST_OIDC_LISTEN_ADDR", "127.0.0.1:9090")
	host, _, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		fmt.Fprintln(os.Stderr, "合成 OIDC 只允许明确 loopback 监听地址")
		os.Exit(1)
	}
	issuer, err := testoidc.New(testoidc.Config{
		Issuer: "http://" + address, ClientID: value("MOUSEHOLE_OIDC_CLIENT_ID", "mousehole-local"),
		ClientSecret: value("MOUSEHOLE_OIDC_CLIENT_SECRET", "mousehole-local-secret"),
		Subject:      value("MOUSEHOLE_TEST_OIDC_SUBJECT", "mousehole-local-analyst"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "合成 OIDC 配置无效")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err = testoidc.Serve(ctx, address, issuer); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "合成 OIDC 服务停止")
		os.Exit(1)
	}
}

func value(name, fallback string) string {
	if current := os.Getenv(name); current != "" {
		return current
	}
	return fallback
}
