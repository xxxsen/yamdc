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

// NewAPI 组装 HTTP 层 API 对象。reviewSvc 在生产路径是必需的: /api/review/*
// 路由全部依赖它, 没有它 handler 会 nil-deref。为避免"本地测试时传 nil 启动
// 成功 → 请求到 review 路由才 panic" 这种滞后故障, 这里在构造时显式拒绝 nil。
// 其它依赖 (jobRepo / scanner / media / store / cleaner / debugger / handlers /
// editor / healthCheck) 按 handler 出现时再 nil-check, 因为它们分属互不重叠
// 的路由子集, 最小场景 (例如只挂 /api/healthz) 允许部分 nil 以便轻量集成测试。
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
	if reviewSvc == nil {
		panic("web.NewAPI: reviewSvc is required")
	}
	return &API{
		jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, reviewSvc: reviewSvc,
		saveDir: saveDir, media: media, store: storage,
		cleaner: cleaner, debugger: debugger, handlers: handlers, editor: editor,
		healthCheck: healthCheck,
	}
}
