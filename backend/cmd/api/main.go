// Command api 启动企业身份与草稿命令服务；不执行数据库迁移。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mousehole/backend/internal/command"
	"mousehole/backend/internal/httpapi"
	"mousehole/backend/internal/identity"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API 启动或运行失败", "event", "api_failed", "reason", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := httpapi.LoadConfig()
	if err != nil {
		return err
	}
	databaseURL := os.Getenv("MOUSEHOLE_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("必须设置 MOUSEHOLE_DATABASE_URL")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("数据库连接配置无效，请检查本地配置")
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(pingCtx)
	cancel()
	if err != nil {
		return errors.New("无法连接数据库，请检查本地服务和配置")
	}
	identityService, err := identity.NewService(ctx, cfg.IdentityConfig(), pool)
	if err != nil {
		return err
	}
	handler, err := httpapi.NewHandler(cfg, command.NewService(pool), identityService, logger)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("监听本地端口失败: %w", err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	result := make(chan error, 1)
	// 入口拥有此 goroutine，退出前通过 result 等待 HTTP 服务停止。
	go func() { result <- server.Serve(listener) }()
	logger.Info("API 已启动", "event", "api_started", "listen_addr", cfg.ListenAddr, "mode", cfg.Mode)
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP 服务意外停止")
	case <-ctx.Done():
		// 请求上下文仍可在停机宽限期内完成；不复用已经取消的进程上下文。
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err = server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-result
			return errors.New("HTTP 服务未在宽限期内完成停机")
		}
		<-result
		logger.Info("API 已停止", "event", "api_stopped")
		return nil
	}
}
