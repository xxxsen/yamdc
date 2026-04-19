package bootstrap

import (
	"context"

	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/cronscheduler"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/review"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/web"
)

type InfraDeps struct {
	Config     *config.Config
	Logger     *zap.Logger
	HTTPClient client.IHTTPClient
	CacheStore store.IStorage
}

type RuntimeDeps struct {
	AIEngine   aiengine.IAIEngine
	Translator translator.ITranslator
	FaceRec    face.IFaceRec
}

type DomainDeps struct {
	Searchers         []searcher.ISearcher
	CategorySearchers map[string][]searcher.ISearcher
	Processors        []processor.IProcessor
	MovieIDCleaner    movieidcleaner.Cleaner
	// MovieIDCleanerMgr 留着是因为 cronscheduler 需要注册它的 remote sync job;
	// MovieIDCleaner 字段是 Cleaner 接口 (passthrough 或 runtime swap), 无法
	// 直接拿到 Manager.CronJob, 所以把 Manager 也挂进 Domain, nil 合法表示
	// "配置里没有 ruleset, 对应 cron 注册会被跳过"。
	MovieIDCleanerMgr *movieidcleaner.Manager
	SearcherDebugger  *searcher.Debugger
	RuntimeSearcher   *searcher.RuntimeCategorySearcher
	HandlerDebugger   *handler.Debugger
	PluginBundleMgr   *pluginbundle.Manager
	Capture           *capture.Capture
}

type AppDeps struct {
	AppDB         *repository.SQLite
	JobRepo       *repository.JobRepository
	LogRepo       *repository.LogRepository
	ScrapeRepo    *repository.ScrapeDataRepository
	ScanSvc       *scanner.Service
	JobSvc        *job.Service
	ReviewSvc     *review.Service
	MediaSvc      *medialib.Service
	API           *web.API
	CronScheduler *cronscheduler.Scheduler
}

type StartContext struct {
	Infra   InfraDeps
	Runtime RuntimeDeps
	Domain  DomainDeps
	App     AppDeps
	Cleanup *CleanupManager
}

func NewStartContext(c *config.Config) *StartContext {
	return &StartContext{
		Infra: InfraDeps{
			Config: c,
		},
		Cleanup: NewCleanupManager(),
	}
}

func (sc *StartContext) RunCleanup(ctx context.Context) error {
	return sc.Cleanup.Run(ctx)
}
