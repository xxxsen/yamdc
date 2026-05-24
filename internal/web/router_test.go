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

// TestEngineCORSWildcardPreflightAnyOrigin: 默认 wildcard 模式下任意 Origin
// 的 OPTIONS 预检必须返回 204 + Access-Control-Allow-Origin: * 与完整 Allow-* 头.
func TestEngineCORSWildcardPreflightAnyOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	req.Header.Set("Origin", "https://yamdc.local")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
}

// TestEngineCORSWildcardGetAnyOrigin: 默认 wildcard 模式下任意 Origin 的
// GET 请求返回 200 + Access-Control-Allow-Origin: *. Allow-Methods /
// Allow-Headers 仅在 OPTIONS 预检阶段写入, 实际 GET / POST 不再重复写
// 这两个头 (浏览器按 fetch spec 也只会在预检阶段读取它们).
func TestEngineCORSWildcardGetAnyOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "https://yamdc.local")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Headers"))
}

// TestEngineCORSWildcardPostAnyOriginNotBlocked: 默认 wildcard 模式下
// POST + 任意 Origin 不被 CORS middleware 以 403 拦截, 由 handler 决定结果.
func TestEngineCORSWildcardPostAnyOriginNotBlocked(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	defer func() { _ = recover() }()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/scan", nil)
	req.Header.Set("Origin", "https://desk.local")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSWildcardNoOrigin: wildcard 模式下没有 Origin 头的请求 (本机
// curl / same-origin) 仍然能通过, 响应里仍写入 "*" 是无害的.
func TestEngineCORSWildcardNoOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSWhitelistPreflightAllowedOrigin: 显式白名单模式下命中白名单
// 的 OPTIONS 预检必须返回 204 + Allow-* 头, 并回显请求 Origin.
func TestEngineCORSWhitelistPreflightAllowedOrigin(t *testing.T) {
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
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Contains(t, rec.Header().Get("Vary"), "Origin")
}

// TestEngineCORSWhitelistPreflightDeniedOrigin: 显式白名单模式下不在白名单
// 的 OPTIONS 预检必须返回 403, 不返回 Allow-* 头.
func TestEngineCORSWhitelistPreflightDeniedOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
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

// TestEngineCORSWhitelistPreflightNoOrigin: 显式白名单模式下 same-origin 的
// OPTIONS 没有 Origin 头, 仍然要返回 204, 不应被 403 误杀.
func TestEngineCORSWhitelistPreflightNoOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSWhitelistGetAllowedOrigin: 显式白名单模式下命中白名单的 GET
// 请求返回 Allow-Origin 头.
func TestEngineCORSWhitelistGetAllowedOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "http://desk.local:3001")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://desk.local:3001", rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestEngineCORSWhitelistGetUnknownOriginAllowed: 显式白名单模式下不在白名单
// 的 Origin 发 GET 不被 middleware 拦截 (GET 是只读), 但响应头里不会出现
// Access-Control-Allow-Origin, 浏览器会自动阻止 JS 读取响应.
func TestEngineCORSWhitelistGetUnknownOriginAllowed(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
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

// TestEngineCORSWhitelistPostDeniedOrigin: 显式白名单模式下状态变更方法
// (POST) + 不在白名单 Origin 必须返回 403.
func TestEngineCORSWhitelistPostDeniedOrigin(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
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

// TestEngineCORSWhitelistPostNoOriginAllowed: 显式白名单模式下没有 Origin
// 头的 POST (CLI / curl / same-origin SSR fetch) 必须正常进入 handler.
func TestEngineCORSWhitelistPostNoOriginAllowed(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	defer func() { _ = recover() }()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/scan", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusForbidden, rec.Code)
}

// TestLoadAllowedOriginsDefault: YAMDC_ALLOWED_ORIGINS 未设置时返回空切片,
// 表示 wildcard 模式 (Access-Control-Allow-Origin: *).
func TestLoadAllowedOriginsDefault(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "")
	got := loadAllowedOrigins()
	assert.Empty(t, got)
}

// TestLoadAllowedOriginsCustom: 自定义 origin 列表覆盖默认.
func TestLoadAllowedOriginsCustom(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001 , http://lan.local:3000 ,")
	got := loadAllowedOrigins()
	assert.ElementsMatch(t, []string{"http://desk.local:3001", "http://lan.local:3000"}, got)
}

// TestLoadAllowedOriginsEmptyAfterTrim: 全空白 / 全空逗号回退到 wildcard 模式.
func TestLoadAllowedOriginsEmptyAfterTrim(t *testing.T) {
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "  ,  ")
	got := loadAllowedOrigins()
	assert.Empty(t, got)
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
	t.Setenv("YAMDC_ALLOWED_ORIGINS", "http://desk.local:3001")
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
