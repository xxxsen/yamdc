package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 正常 case: 默认 /api/healthz 永远 200 ok ---

func TestHandleHealthz(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodGet, "/api/healthz", nil)
	api.handleHealthz(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "ok", resp.Message)
}

// TestHandleHealthzDeepWithoutChecker: deep=1 但未注入 checker,
// 应返回 200 ok 并在 data.deep 里标记 skipped, 保持不破坏探活脚本。
func TestHandleHealthzDeepWithoutChecker(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodGet, "/api/healthz?deep=1", nil)
	api.handleHealthz(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be map, got %T", resp.Data)
	assert.Equal(t, "ok", data["status"])
	assert.Equal(t, "skipped", data["deep"])
}

// TestHandleHealthzDeepSuccess: deep=1 且 checker 返回 nil, 应 200 ok + deep=ok。
func TestHandleHealthzDeepSuccess(t *testing.T) {
	called := false
	api := &API{healthCheck: func(context.Context) error {
		called = true
		return nil
	}}
	c, rec := newGinContext(http.MethodGet, "/api/healthz?deep=1", nil)
	api.handleHealthz(c)
	assert.True(t, called, "healthCheck should be invoked for deep=1")
	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", data["deep"])
}

// --- 异常 case: checker 报错 -> 503 ---

func TestHandleHealthzDeepFailure(t *testing.T) {
	api := &API{healthCheck: func(context.Context) error {
		return errors.New("db down")
	}}
	c, rec := newGinContext(http.MethodGet, "/api/healthz?deep=1", nil)
	api.handleHealthz(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var resp responseBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, errCodeUnknown, resp.Code)
	assert.Contains(t, resp.Message, "db down")
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "unhealthy", data["status"])
}

// --- 边缘 case: deep 值不是 "1" 的视为非深度探测, 走快路径 ---

func TestHandleHealthzDeepQueryVariants(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantCalled bool
	}{
		{"deep=0", "/api/healthz?deep=0", false},
		{"deep=true (not 1)", "/api/healthz?deep=true", false},
		{"no deep param", "/api/healthz", false},
		{"deep=1 explicit", "/api/healthz?deep=1", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			api := &API{healthCheck: func(context.Context) error {
				called = true
				return nil
			}}
			c, rec := newGinContext(http.MethodGet, tc.target, nil)
			api.handleHealthz(c)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tc.wantCalled, called)
		})
	}
}

// --- 边缘 case: checker 慢 + 请求 ctx 已取消, handler 应及时返回而不是卡住 ---

func TestHandleHealthzDeepRespectsContext(t *testing.T) {
	api := &API{healthCheck: func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}}
	rec := httptest.NewRecorder()
	c, _ := newGinContextWithCanceledCtx(t, http.MethodGet, "/api/healthz?deep=1", rec)
	done := make(chan struct{})
	go func() {
		defer close(done)
		api.handleHealthz(c)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after request ctx canceled")
	}
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
