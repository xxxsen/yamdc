package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/xxxsen/yamdc/internal/web"
	"go.uber.org/zap"
)

// defaultShutdownTimeout 是收到退出信号后等待 HTTP server 收尾的最大时间。
// 超时后 Shutdown 会强制关闭未完成的连接并返回错误。
const defaultShutdownTimeout = 30 * time.Second

// defaultReadHeaderTimeout 防止慢客户端长时间占住连接。
const defaultReadHeaderTimeout = 30 * time.Second

// ServeHTTP 启动 YAMDC 内置 HTTP server 并阻塞至服务结束。
//
// ctx 被取消(进程收到 SIGINT/SIGTERM)后会触发 graceful shutdown:
// 停止接受新连接 -> 等待已有请求完成或 defaultShutdownTimeout 到期 ->
// 函数返回。调用方随后应执行数据库/后台任务 wait 等收尾动作。
func ServeHTTP(
	ctx context.Context,
	api *web.API,
	logger *zap.Logger,
	scanDir, dataDir string,
) error {
	addr := os.Getenv("YAMDC_SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if logger != nil {
		logger.Info("yamdc server start",
			zap.String("addr", addr),
			zap.String("scan_dir", scanDir),
			zap.String("data_dir", dataDir),
		)
	}
	engine, err := api.Engine(addr)
	if err != nil {
		return fmt.Errorf("init web engine failed, err:%w", err)
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()
	select {
	case <-ctx.Done():
		if logger != nil {
			logger.Info("yamdc server stopping", zap.Error(ctx.Err()))
		}
		// shutdownCtx 必须独立于已被 cancel 的 ctx, 否则 Shutdown 会立刻
		// 返回 context canceled 而不去等待 in-flight 请求完成, 失去 graceful 的意义。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		//nolint:contextcheck // 独立 ctx 是有意设计, 用于实现 grace window
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown failed: %w", err)
		}
		if err := <-serveErr; err != nil {
			return fmt.Errorf("http server serve error: %w", err)
		}
		if logger != nil {
			logger.Info("yamdc server stopped")
		}
		return nil
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("listen and serve failed, err:%w", err)
		}
		return nil
	}
}
