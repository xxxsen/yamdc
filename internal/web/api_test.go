package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/review"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	plugineditor "github.com/xxxsen/yamdc/internal/searcher/plugin/editor"
	"github.com/xxxsen/yamdc/internal/store"
)

// stubDeps 把 NewAPI fail-fast 要求的全部"非 nil 必需依赖"打包. 这些 stub
// 仅满足"构造期非 nil"契约, 内部 *sql.DB / 真实仓库等多数为 nil — 一旦
// 真去打底层方法 (例如 jobRepo.ListJobs / reviewSvc.SaveReviewData) 立刻
// nil-deref. 本 helper 只用于 "测 NewAPI 自身 / 测 healthz 这类不打 service
// 的极简路径". handler-level 单元测试仍然用 &API{cleaner: stub, ...} 这种
// white-box 写法, 直接构造 zero-value API + 注入需要的字段.
type stubDeps struct {
	jobRepo  *repository.JobRepository
	scanner  *scanner.Service
	jobSvc   *job.Service
	reviewer *review.Service
	media    *medialib.Service
	storage  store.IStorage
	cleaner  movieidcleaner.Cleaner
	debugger *searcher.Debugger
	handlers *phandler.Debugger
	editor   *plugineditor.Service
}

func newStubDeps(t *testing.T) stubDeps {
	t.Helper()
	cli := client.MustNewClient()
	editorSvc, err := plugineditor.NewService(cli)
	require.NoError(t, err)
	return stubDeps{
		jobRepo:  repository.NewJobRepository(nil),
		scanner:  scanner.New("", nil, nil, movieidcleaner.NewPassthroughCleaner()),
		jobSvc:   job.NewService(nil, nil, nil, nil, nil),
		reviewer: review.NewService(nil, nil, nil, nil, nil),
		media:    medialib.NewService(nil, "", ""),
		storage:  store.NewMemStorage(),
		cleaner:  movieidcleaner.NewPassthroughCleaner(),
		debugger: searcher.NewDebugger(cli, store.NewMemStorage(), movieidcleaner.NewPassthroughCleaner(), nil, nil),
		handlers: phandler.NewDebugger(phandlerDebugRuntime(), movieidcleaner.NewPassthroughCleaner(), nil, nil),
		editor:   editorSvc,
	}
}

func (s stubDeps) build() *API {
	return NewAPI(
		s.jobRepo, s.scanner, s.jobSvc, s.reviewer,
		"", s.media, s.storage, s.cleaner,
		s.debugger, s.handlers, s.editor, nil,
	)
}

// TestNewAPI: 正常路径 — 全部依赖齐全时构造成功, 字段正确填入.
func TestNewAPI(t *testing.T) {
	deps := newStubDeps(t)
	api := NewAPI(
		deps.jobRepo, deps.scanner, deps.jobSvc, deps.reviewer,
		"/tmp/save", deps.media, deps.storage, deps.cleaner,
		deps.debugger, deps.handlers, deps.editor, nil,
	)
	assert.NotNil(t, api)
	assert.Equal(t, "/tmp/save", api.saveDir)
	assert.Same(t, deps.reviewer, api.reviewSvc)
	assert.Same(t, deps.jobRepo, api.jobRepo)
	assert.Same(t, deps.editor, api.editor)
}

// TestNewAPIPanicsOnNilRequiredDep: 异常路径 — fail-fast: 任意必需依赖为 nil
// 都会在 NewAPI 阶段直接 panic, 不允许把"装配错"延迟到 handler 运行期.
//
// 这条测试逐项把每个必需依赖换成 nil, 断 NewAPI panic, 守 fail-fast 契约
// 在加 / 删依赖时不会被悄悄打破 (新加依赖必须挂上对应 == nil 检查 +
// 新增 stub 字段, 否则这条 test 表里能立刻看到漏掉的项).
func TestNewAPIPanicsOnNilRequiredDep(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*stubDeps)
	}{
		{"nil jobRepo", func(d *stubDeps) { d.jobRepo = nil }},
		{"nil scanner", func(d *stubDeps) { d.scanner = nil }},
		{"nil jobSvc", func(d *stubDeps) { d.jobSvc = nil }},
		{"nil reviewSvc", func(d *stubDeps) { d.reviewer = nil }},
		{"nil media", func(d *stubDeps) { d.media = nil }},
		{"nil storage", func(d *stubDeps) { d.storage = nil }},
		{"nil cleaner", func(d *stubDeps) { d.cleaner = nil }},
		{"nil debugger", func(d *stubDeps) { d.debugger = nil }},
		{"nil handlers", func(d *stubDeps) { d.handlers = nil }},
		{"nil editor", func(d *stubDeps) { d.editor = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newStubDeps(t)
			tc.mutate(&deps)
			assert.Panics(t, func() { _ = deps.build() })
		})
	}
}

// TestNewAPIAllowsNilHealthCheck: 边缘路径 — healthCheck 显式允许 nil
// (deep 健康检查可选), NewAPI 不应因此 panic.
func TestNewAPIAllowsNilHealthCheck(t *testing.T) {
	deps := newStubDeps(t)
	require.NotPanics(t, func() {
		api := deps.build()
		require.NotNil(t, api)
	})
}

// TestNewAPIAllowsEmptySaveDir: 边缘路径 — saveDir="" 表示 library 工作目录
// 未配置, 这是合法状态 (其它路由仍可工作), NewAPI 不应 panic.
func TestNewAPIAllowsEmptySaveDir(t *testing.T) {
	deps := newStubDeps(t)
	require.NotPanics(t, func() {
		api := deps.build()
		require.Equal(t, "", api.saveDir)
	})
}

// TestEngineHealthzWithStubDeps: 边缘路径 — 全 stub 依赖时 /api/healthz
// 仍可启动并返回 0 code. healthz 不依赖任何后端 service, 这条 test 守住
// "最小可启动 API + 健康探针仍工作" 的契约.
func TestEngineHealthzWithStubDeps(t *testing.T) {
	api := newStubDeps(t).build()
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}
