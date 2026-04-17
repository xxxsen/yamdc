package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/review"
	"github.com/xxxsen/yamdc/internal/web"
)

// stubReviewSvc 返回一个构造时不会 panic 的 review.Service。内部 coordinator
// 等依赖均为 nil, 一旦对它调用任何对外方法 (SaveReviewData / Import /
// CropPosterFromCover) 就会立即 panic (nil pointer deref in Claim / jobRepo 等)。
// 本文件的测试只走健康检查 / listen failure 路径, 不会触发任何 /api/review/*
// 路由或 review 方法, 用它满足 web.NewAPI 的"必需非 nil" 契约即可;
// 若后续新增用例真的要打 review 接口, 必须换成 newTestReviewService 这类
// 带真实依赖的构造器。
func stubReviewSvc() *review.Service {
	return review.NewService(nil, nil, nil, nil, nil)
}

// --- 正常 case: ctx 取消后 ServeHTTP 以 graceful shutdown 方式退出 ---

func TestServeHTTPGracefulShutdownOnCtxCancel(t *testing.T) {
	t.Setenv("YAMDC_SERVER_ADDR", pickFreeAddr(t))
	api := web.NewAPI(nil, nil, nil, stubReviewSvc(), "", nil, nil, nil, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ServeHTTP(ctx, api, nil, "", "")
	}()

	require.NoError(t, waitForHealthz(t, resolveAddr(t)), "server should become reachable")

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ServeHTTP did not return within 5s after ctx cancel")
	}
}

// --- 异常 case: 端口被占用, ServeHTTP 立即返回 ListenAndServe 错误 ---

func TestServeHTTPListenAndServeFailure(t *testing.T) {
	addr := pickFreeAddr(t)
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	t.Setenv("YAMDC_SERVER_ADDR", addr)
	api := web.NewAPI(nil, nil, nil, stubReviewSvc(), "", nil, nil, nil, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ServeHTTP(ctx, api, nil, "", "")
	}()

	select {
	case serveErr := <-done:
		require.Error(t, serveErr, "expected listen failure")
		assert.Contains(t, serveErr.Error(), "listen and serve failed")
	case <-time.After(3 * time.Second):
		t.Fatal("ServeHTTP did not return after listen failure within 3s")
	}
}

// --- 边缘 case: ctx 预先取消, ServeHTTP 依然可以干净退出 (Shutdown 路径) ---

func TestServeHTTPCtxAlreadyCanceled(t *testing.T) {
	t.Setenv("YAMDC_SERVER_ADDR", pickFreeAddr(t))
	api := web.NewAPI(nil, nil, nil, stubReviewSvc(), "", nil, nil, nil, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- ServeHTTP(ctx, api, nil, "", "")
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ServeHTTP did not return within 5s when ctx already canceled")
	}
}

// --- helpers ---

// pickFreeAddr 找一个可用的本机地址。简单通过短暂 Listen + Close 获取端口,
// 测试内串行使用以避免重入冲突。
func pickFreeAddr(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

func resolveAddr(t *testing.T) string {
	t.Helper()
	v := os.Getenv("YAMDC_SERVER_ADDR")
	require.NotEmpty(t, v)
	return v
}

func waitForHealthz(t *testing.T, addr string) error {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fmt.Sprintf("http://%s/api/healthz", addr), nil)
		if err != nil {
			reqCancel()
			return err
		}
		cli := &http.Client{Timeout: 200 * time.Millisecond}
		rsp, err := cli.Do(req)
		reqCancel()
		if err == nil {
			_, _ = io.Copy(io.Discard, rsp.Body)
			_ = rsp.Body.Close()
			if rsp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("unexpected status %d", rsp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("timeout waiting for healthz")
	}
	return lastErr
}
