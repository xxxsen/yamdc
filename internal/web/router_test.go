package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineBuildDoesNotPanic(t *testing.T) {
	api := &API{}
	require.NotPanics(t, func() {
		engine, err := api.Engine(":0")
		require.NoError(t, err)
		require.NotNil(t, engine)
	})
}

func TestEngineHealthzRoute(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

// TestEngineCORSPreflightAllowedOrigin: 命中白名单的 OPTIONS 预检必须返回
// 204 + 完整的 Allow-* 头, 并且 Access-Control-Allow-Origin 回显请求的 Origin.
func TestEngineCORSPreflightAllowedOrigin(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Contains(t, rec.Header().Get("Vary"), "Origin")
}

// TestEngineCORSPreflightDeniedOrigin: 不在白名单的 OPTIONS 预检必须返回
// 403, 不返回任何 Allow-* 头, 让浏览器直接放弃后续的真正请求.
func TestEngineCORSPreflightDeniedOrigin(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	body := decodeResponse(t, rec)
	assert.Equal(t, errCodeOriginForbidden, body.Code)
	assert.Contains(t, body.Message, "https://evil.example")
}

// TestEngineCORSPreflightNoOrigin: same-origin 的 OPTIONS 没有 Origin 头,
// 仍然要返回 204, 不应被 403 误杀 (例如某些客户端在同源下也发预检).
func TestEngineCORSPreflightNoOrigin(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSGetAllowedOrigin: 命中白名单的 GET 请求返回 Allow-Origin 头.
func TestEngineCORSGetAllowedOrigin(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://127.0.0.1:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSGetUnknownOriginAllowed: 不在白名单的 Origin 发 GET 不被
// 拦截 (GET 是只读, 真正的拦截依赖浏览器无法读响应), 但响应头里不会
// 出现 Access-Control-Allow-Origin, 浏览器会自动阻止 JS 读取响应.
func TestEngineCORSGetUnknownOriginAllowed(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSPostDeniedOrigin: 状态变更方法 (POST) + 不在白名单 Origin
// 必须返回 403, 防止任意第三方网页跨站触发本机副作用.
func TestEngineCORSPostDeniedOrigin(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/scan", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	body := decodeResponse(t, rec)
	assert.Equal(t, errCodeOriginForbidden, body.Code)
}

// TestEngineCORSPostNoOriginAllowed: 没有 Origin 头的 POST (CLI / curl /
// same-origin SSR fetch) 必须正常进入 handler, 不能被 Origin 校验误杀.
// /api/scan 在没有依赖时会 500/panic 之前先经过 middleware, 这里我们用
// 不会真正触发依赖的 healthz 来验证 middleware 行为, 但 healthz 是 GET;
// 改用一个 GET 验证 + POST 校验的组合: POST 到一个会 fail 的路由仍然要
// 进入 handler 并返回 200 + body 业务错误 (而不是 middleware 的 403).
func TestEngineCORSPostNoOriginAllowed(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	// /api/scan 没有 scanner 时会 panic, 我们捕获 panic 来验证 handler
	// 至少被 middleware 放行, 而不是被 403 拦在更早.
	defer func() { _ = recover() }()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/scan", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusForbidden, rec.Code)
}

// TestLoadAllowedOriginsDefault: YAMDC_ALLOWED_ORIGINS 未设置时应该返回
// localhost:3000 / 127.0.0.1:3000.
func TestLoadAllowedOriginsDefault(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "")
	got := loadAllowedOrigins()
	assert.ElementsMatch(t, []string{"http://localhost:3000", "http://127.0.0.1:3000"}, got)
}

// TestLoadAllowedOriginsCustom: 自定义 origin 列表覆盖默认.
func TestLoadAllowedOriginsCustom(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001 , http://lan.local:3000 ,")
	got := loadAllowedOrigins()
	assert.ElementsMatch(t, []string{"http://desk.local:3001", "http://lan.local:3000"}, got)
}

// TestLoadAllowedOriginsEmptyAfterTrim: 全空白 / 全空逗号回退到默认.
func TestLoadAllowedOriginsEmptyAfterTrim(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "  ,  ")
	got := loadAllowedOrigins()
	assert.ElementsMatch(t, []string{"http://localhost:3000", "http://127.0.0.1:3000"}, got)
}

// TestEngineCORSCustomOrigin: 通过环境变量配置自定义 origin, 该 origin 的
// 请求应该被 middleware 接受.
func TestEngineCORSCustomOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	req.Header.Set("Origin", "http://desk.local:3001")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://desk.local:3001", rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestAllowedOriginSetContains: 直接验证集合行为, 包括空字符串过滤和重复
// 元素去重.
func TestAllowedOriginSetContains(t *testing.T) {
	set := buildAllowedOriginSet([]string{"http://a", "  ", "http://a", "http://b"})
	assert.True(t, set.contains("http://a"))
	assert.True(t, set.contains("http://b"))
	assert.False(t, set.contains("http://c"))
	assert.False(t, set.contains(""))
}

// TestAbortWithOriginForbiddenBody: middleware 用于 403 的 body 必须遵循
// 项目协议 { code, message, data }, 并写入 errCodeOriginForbidden.
func TestAbortWithOriginForbiddenBody(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/scan", strings.NewReader("{}"))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	body := decodeResponse(t, rec)
	assert.Equal(t, errCodeOriginForbidden, body.Code)
	assert.NotEmpty(t, body.Message)
}
