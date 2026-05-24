package web

import (
	"context"

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

// HealthCheckFunc 用于 /api/healthz?deep=1 的深度探测。
// 通常实现为 sql.DB.PingContext, 传 nil 表示不提供深度探测。
type HealthCheckFunc func(ctx context.Context) error

type API struct {
	jobRepo     *repository.JobRepository
	scanner     *scanner.Service
	jobSvc      *job.Service
	reviewSvc   *review.Service
	saveDir     string
	media       *medialib.Service
	store       store.IStorage
	cleaner     movieidcleaner.Cleaner
	debugger    *searcher.Debugger
	handlers    *phandler.Debugger
	editor      *plugineditor.Service
	healthCheck HealthCheckFunc
}

// NewAPI 组装 HTTP 层 API 对象, 走 fail-fast: 任意必需依赖为 nil 直接 panic,
// 把"依赖装配错误"在进程启动期暴露, 而不是等到具体 handler 被打到时才发现.
//
// 设计取舍:
//
//   - 必需依赖一律 == nil 即 panic, 调用方 (生产: internal/bootstrap; 测试:
//     stub 注入) 必须保证全部非 nil. handler 内部不再做"运行期 nil 守门",
//     避免漏改一处 guard 就 nil-deref 的隐患.
//   - 这里的参数都是具体指针类型 (*T) 或具体接口类型 (store.IStorage /
//     movieidcleaner.Cleaner), == nil 直接生效, 不需要 reflect 处理
//     "typed-nil 包到 interface 里" 的边角: 调用方若传 typed-nil 是调用方
//     自身 bug, 由 bootstrap 集成测试守住, 不在运行期再绕一层兜底.
//
// 可选项:
//
//   - saveDir: 允许 "", 表示未配置 library 工作目录; library 路由会用
//     requireSaveDir 在请求层返 503, 不在 NewAPI 阻断 (其它路由仍可正常工作).
//   - healthCheck: 允许 nil, 表示不提供 deep 健康检查; /api/healthz 仍可用.
func NewAPI(
	jobRepo *repository.JobRepository,
	scanner *scanner.Service,
	jobSvc *job.Service,
	reviewSvc *review.Service,
	saveDir string,
	media *medialib.Service,
	storage store.IStorage,
	cleaner movieidcleaner.Cleaner,
	debugger *searcher.Debugger,
	handlers *phandler.Debugger,
	editor *plugineditor.Service,
	healthCheck HealthCheckFunc,
) *API {
	if jobRepo == nil {
		panic("yamdc: NewAPI requires non-nil jobRepo")
	}
	if scanner == nil {
		panic("yamdc: NewAPI requires non-nil scanner")
	}
	if jobSvc == nil {
		panic("yamdc: NewAPI requires non-nil jobSvc")
	}
	if reviewSvc == nil {
		panic("yamdc: NewAPI requires non-nil reviewSvc")
	}
	if media == nil {
		panic("yamdc: NewAPI requires non-nil media")
	}
	if storage == nil {
		panic("yamdc: NewAPI requires non-nil storage")
	}
	if cleaner == nil {
		panic("yamdc: NewAPI requires non-nil cleaner")
	}
	if debugger == nil {
		panic("yamdc: NewAPI requires non-nil debugger")
	}
	if handlers == nil {
		panic("yamdc: NewAPI requires non-nil handlers")
	}
	if editor == nil {
		panic("yamdc: NewAPI requires non-nil editor")
	}
	return &API{
		jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, reviewSvc: reviewSvc,
		saveDir: saveDir, media: media, store: storage,
		cleaner: cleaner, debugger: debugger, handlers: handlers, editor: editor,
		healthCheck: healthCheck,
	}
}
