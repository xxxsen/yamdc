package web

import (
	"context"

	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
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
	saveDir     string
	media       *medialib.Service
	store       store.IStorage
	cleaner     movieidcleaner.Cleaner
	debugger    *searcher.Debugger
	handlers    *phandler.Debugger
	editor      *plugineditor.Service
	healthCheck HealthCheckFunc
}

func NewAPI(
	jobRepo *repository.JobRepository,
	scanner *scanner.Service,
	jobSvc *job.Service,
	saveDir string,
	media *medialib.Service,
	storage store.IStorage,
	cleaner movieidcleaner.Cleaner,
	debugger *searcher.Debugger,
	handlers *phandler.Debugger,
	editor *plugineditor.Service,
	healthCheck HealthCheckFunc,
) *API {
	return &API{
		jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, saveDir: saveDir, media: media, store: storage,
		cleaner: cleaner, debugger: debugger, handlers: handlers, editor: editor,
		healthCheck: healthCheck,
	}
}
