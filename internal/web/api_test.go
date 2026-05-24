package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/review"
)

// TestNewAPI: 正常路径 — 全 nil 依赖也能构造 API, 不再 panic.
// reviewSvc 由生产组装层 (bootstrap) 通过集成测试保证齐全, NewAPI 自身
// 是 dumb constructor.
func TestNewAPI(t *testing.T) {
	stubReview := review.NewService(nil, nil, nil, nil, nil)
	api := NewAPI(nil, nil, nil, stubReview, "/tmp/save", nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, api)
	assert.Equal(t, "/tmp/save", api.saveDir)
	assert.Same(t, stubReview, api.reviewSvc)
}

// TestNewAPIAllowsNilReviewService: 异常路径 — 旧版本对 reviewSvc nil
// 直接 panic, 现版本允许构造但 review 路由命中时由 requireDependency 守门
// 返回 503. 这保证最小场景 (例如只挂 healthz) 仍可启动.
func TestNewAPIAllowsNilReviewService(t *testing.T) {
	require.NotPanics(t, func() {
		api := NewAPI(nil, nil, nil, nil, "/tmp/save", nil, nil, nil, nil, nil, nil, nil)
		require.NotNil(t, api)
	})
}

// TestEngineHealthzWithAllNilDeps: 边缘路径 — 全 nil 依赖时 /api/healthz
// 仍可启动并返回 0 code. 这是 "最小依赖也能起 server" 的核心契约.
func TestEngineHealthzWithAllNilDeps(t *testing.T) {
	api := NewAPI(nil, nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil)
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

// TestRequireDependencyAvailable: 正常路径 — 依赖存在时 helper 返回 true
// 且 handler 可继续执行.
func TestRequireDependencyAvailable(t *testing.T) {
	c, rec := newGinContext(http.MethodGet, "/", nil)
	stubReview := review.NewService(nil, nil, nil, nil, nil)
	ok := requireDependency(c, stubReview, "review")
	assert.True(t, ok)
	assert.NotEqual(t, http.StatusServiceUnavailable, rec.Code)
}

// TestRequireDependencyNil: 异常路径 — typed nil 触发 503 + body code.
func TestRequireDependencyNil(t *testing.T) {
	c, rec := newGinContext(http.MethodGet, "/", nil)
	var nilReview *review.Service
	ok := requireDependency(c, nilReview, "review")
	assert.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decodeResponse(t, rec)
	assert.Equal(t, errCodeServiceUnavailable, body.Code)
	assert.Contains(t, body.Message, "review")
}

// TestRequireDependencyAnyNil: 边缘路径 — any(nil) 也触发 503, 验证 helper
// 不依赖具体类型也能识别 nil.
func TestRequireDependencyAnyNil(t *testing.T) {
	c, rec := newGinContext(http.MethodGet, "/", nil)
	ok := requireDependency(c, nil, "anything")
	assert.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestRequireDependencyUnknownType: 边缘路径 — 不在已知 switch 内的类型,
// 默认放行 (例如 string / int 这些不可能进 NewAPI 的类型, 由调用层负责).
func TestRequireDependencyUnknownType(t *testing.T) {
	c, _ := newGinContext(http.MethodGet, "/", nil)
	type unknown struct{}
	ok := requireDependency(c, &unknown{}, "x")
	assert.True(t, ok)
}

// TestRequireDependencyReflectTypedNilPointer: 边缘路径 — 未列入 switch
// 的 typed-nil 指针 (例如未来新增的依赖类型还没挂到 switch), 通过 reflect
// 兜底也必须识别为 "不可用". 避免 typed-nil 漏判导致 nil-deref.
func TestRequireDependencyReflectTypedNilPointer(t *testing.T) {
	c, rec := newGinContext(http.MethodGet, "/", nil)
	type futureDep struct{}
	var nilFuture *futureDep
	ok := requireDependency(c, nilFuture, "future")
	assert.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestRequireDependencyReflectTypedNilInterface: 边缘路径 — typed-nil
// interface 包装 (var s SomeInterface = (*Impl)(nil)) 直接 == nil 为 false,
// 必须由 reflect.IsNil 兜住.
func TestRequireDependencyReflectTypedNilInterface(t *testing.T) {
	c, rec := newGinContext(http.MethodGet, "/", nil)
	type Doer interface{ Do() }
	var nilDoer Doer = (*nilDoerImpl)(nil)
	ok := requireDependency(c, nilDoer, "doer")
	assert.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

type nilDoerImpl struct{}

func (n *nilDoerImpl) Do() {}

// TestRequireDependencyReflectValueKinds: 边缘路径 — 走到 reflect 兜底分支的
// 三类形态: nil map / nil slice / 非 nilable 类型 (string). reflect 兜底
// 把前两类识别为不可用, 把 string 这种 default 类型识别为可用.
// 守护点: 未来误把 map / slice 当依赖时不会因为 isDependencyAvailable
// 误判为可用而走到下一行 nil-deref.
func TestRequireDependencyReflectValueKinds(t *testing.T) {
	c1, rec1 := newGinContext(http.MethodGet, "/", nil)
	var nilMap map[string]int
	ok := requireDependency(c1, nilMap, "m")
	assert.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, rec1.Code)

	c2, rec2 := newGinContext(http.MethodGet, "/", nil)
	var nilSlice []int
	ok = requireDependency(c2, nilSlice, "s")
	assert.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, rec2.Code)

	c3, rec3 := newGinContext(http.MethodGet, "/", nil)
	ok = requireDependency(c3, "non-empty-string", "str")
	assert.True(t, ok)
	assert.NotEqual(t, http.StatusServiceUnavailable, rec3.Code)
}

// TestHandleScanWithoutScanner: 异常路径 — 缺 scanner 依赖时 /api/scan
// 应返回 503 而不是 panic.
func TestHandleScanWithoutScanner(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodPost, "/api/scan", nil)
	api.handleScan(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decodeResponse(t, rec)
	assert.Equal(t, errCodeServiceUnavailable, body.Code)
}

// TestHandleListJobsWithoutDeps: 异常路径 — 缺 jobRepo / jobSvc 时返回
// 503, 不 panic.
func TestHandleListJobsWithoutDeps(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodGet, "/api/jobs", nil)
	api.handleListJobs(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHandleReviewSaveWithoutReviewSvc: 异常路径 — review handler 缺 reviewSvc.
func TestHandleReviewSaveWithoutReviewSvc(t *testing.T) {
	api := &API{}
	c, rec := newGinContextWithParams(http.MethodPut, "/api/review/jobs/1", nil, gin.Params{{Key: "id", Value: "1"}})
	api.handleReviewSave(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHandleListLibraryWithoutSaveDir: 异常路径 — library handler 在 media
// 与 saveDir 都为空时返回 503.
func TestHandleListLibraryWithoutSaveDir(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodGet, "/api/library", nil)
	api.handleListLibrary(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHandleMovieIDCleanerExplainWithoutCleaner: 异常路径 — debug handler
// 缺 cleaner 时返回 503.
func TestHandleMovieIDCleanerExplainWithoutCleaner(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodPost, "/api/debug/movieid-cleaner/explain", nil)
	api.handleMovieIDCleanerExplain(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
