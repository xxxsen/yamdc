package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/web"
	"go.uber.org/zap"
)

type YamdcStartContext struct {
	Config *config.Config
	Logger *zap.Logger

	HTTPClient client.IHTTPClient
	CacheStore store.IStorage
	AIEngine   aiengine.IAIEngine
	Translator translator.ITranslator
	FaceRec    face.IFaceRec

	Searchers         []searcher.ISearcher
	CategorySearchers map[string][]searcher.ISearcher
	Processors        []processor.IProcessor
	NumberCleaner     numbercleaner.Cleaner
	NumberCleanerSync func(context.Context)

	Capture *capture.Capture

	AppDB      *repository.SQLite
	JobRepo    *repository.JobRepository
	LogRepo    *repository.LogRepository
	ScrapeRepo *repository.ScrapeDataRepository

	ScanSvc  *scanner.Service
	JobSvc   *job.Service
	MediaSvc *medialib.Service
	API      *web.API

	cleanups []func(context.Context) error
}

func NewYamdcStartContext(c *config.Config) *YamdcStartContext {
	return &YamdcStartContext{
		Config: c,
	}
}

func (ysctx *YamdcStartContext) AddCleanup(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	ysctx.cleanups = append(ysctx.cleanups, fn)
}

func (ysctx *YamdcStartContext) Cleanup(ctx context.Context) error {
	var firstErr error
	for i := len(ysctx.cleanups) - 1; i >= 0; i-- {
		if err := ysctx.cleanups[i](ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type YamdcInitFunc func(ctx context.Context, ysctx *YamdcStartContext) error

type YamdcInitAction struct {
	Name string
	Fn   YamdcInitFunc
}

func newYamdcInitActions() []YamdcInitAction {
	return []YamdcInitAction{
		{Name: "normalize_dir_paths", Fn: normalizeDirPathsAction},
		{Name: "init_logger", Fn: initLoggerAction},
		{Name: "precheck_dirs", Fn: precheckDirsAction},
		{Name: "build_http_client", Fn: buildHTTPClientAction},
		{Name: "init_dependencies", Fn: initDependenciesAction},
		{Name: "build_ai_engine", Fn: buildAIEngineAction},
		{Name: "build_cache_store", Fn: buildCacheStoreAction},
		{Name: "build_translator", Fn: buildTranslatorAction},
		{Name: "build_face_recognizer", Fn: buildFaceRecognizerAction},
		{Name: "build_searchers", Fn: buildSearchersAction},
		{Name: "build_processors", Fn: buildProcessorsAction},
		{Name: "build_number_cleaner", Fn: buildNumberCleanerAction},
		{Name: "build_capture", Fn: buildCaptureAction},
		{Name: "open_app_db", Fn: openAppDBAction},
		{Name: "assemble_services", Fn: assembleServicesAction},
		{Name: "recover_jobs", Fn: recoverJobsAction},
		{Name: "start_media_service", Fn: startMediaServiceAction},
		{Name: "start_number_cleaner_sync", Fn: startNumberCleanerSyncAction},
		{Name: "serve_http", Fn: serveHTTPAction},
	}
}

func executeYamdcInitActions(ctx context.Context, ysctx *YamdcStartContext, actions []YamdcInitAction) error {
	for _, action := range actions {
		start := time.Now()
		if err := action.Fn(ctx, ysctx); err != nil {
			return fmt.Errorf("%s failed: %w", action.Name, err)
		}
		if ysctx.Logger != nil {
			ysctx.Logger.Debug("yamdc init action done", zap.String("action", action.Name), zap.Duration("cost", time.Since(start)))
		}
	}
	return nil
}

func normalizeDirPathsAction(_ context.Context, ysctx *YamdcStartContext) error {
	return normalizeDirPaths(ysctx.Config)
}

func initLoggerAction(_ context.Context, ysctx *YamdcStartContext) error {
	c := ysctx.Config
	ysctx.Logger = loggerInit(c)
	return nil
}

func precheckDirsAction(_ context.Context, ysctx *YamdcStartContext) error {
	return precheckDir(ysctx.Config)
}

func buildHTTPClientAction(_ context.Context, ysctx *YamdcStartContext) error {
	cli, err := buildHTTPClient(ysctx.Config)
	if err != nil {
		return err
	}
	ysctx.HTTPClient = cli
	return nil
}

func initDependenciesAction(_ context.Context, ysctx *YamdcStartContext) error {
	return initDependencies(ysctx.HTTPClient, ysctx.Config.DataDir, ysctx.Config.Dependencies)
}

func buildAIEngineAction(_ context.Context, ysctx *YamdcStartContext) error {
	engine, err := buildAIEngine(ysctx.HTTPClient, ysctx.Config)
	if err != nil {
		return err
	}
	ysctx.AIEngine = engine
	return nil
}

func buildCacheStoreAction(_ context.Context, ysctx *YamdcStartContext) error {
	cacheStore, err := store.NewSqliteStorage(filepath.Join(ysctx.Config.DataDir, "cache", "cache.db"))
	if err != nil {
		return err
	}
	ysctx.CacheStore = cacheStore
	return nil
}

func buildTranslatorAction(ctx context.Context, ysctx *YamdcStartContext) error {
	tr, err := buildTranslator(ysctx.Config, ysctx.AIEngine)
	if err != nil {
		if ysctx.Logger != nil {
			ysctx.Logger.Error("setup translator failed", zap.Error(err))
		} else {
			logutil.GetLogger(ctx).Error("setup translator failed", zap.Error(err))
		}
		return nil
	}
	ysctx.Translator = tr
	return nil
}

func buildFaceRecognizerAction(ctx context.Context, ysctx *YamdcStartContext) error {
	faceRec, err := buildFaceRecognizer(ysctx.Config, filepath.Join(ysctx.Config.DataDir, "models"))
	if err != nil {
		if ysctx.Logger != nil {
			ysctx.Logger.Error("init face recognizer failed", zap.Error(err))
		} else {
			logutil.GetLogger(ctx).Error("init face recognizer failed", zap.Error(err))
		}
		return nil
	}
	ysctx.FaceRec = faceRec
	return nil
}

func buildSearchersAction(_ context.Context, ysctx *YamdcStartContext) error {
	ss, err := buildSearcher(ysctx.HTTPClient, ysctx.CacheStore, ysctx.Config, ysctx.Config.Plugins, ysctx.Config.PluginConfig)
	if err != nil {
		return err
	}
	catSs, err := buildCatSearcher(ysctx.HTTPClient, ysctx.CacheStore, ysctx.Config, ysctx.Config.CategoryPlugins, ysctx.Config.PluginConfig)
	if err != nil {
		return err
	}
	ysctx.Searchers = ss
	ysctx.CategorySearchers = catSs
	return nil
}

func buildProcessorsAction(_ context.Context, ysctx *YamdcStartContext) error {
	ps, err := buildProcessor(appdeps.Runtime{
		HTTPClient: ysctx.HTTPClient,
		Storage:    ysctx.CacheStore,
		Translator: ysctx.Translator,
		AIEngine:   ysctx.AIEngine,
		FaceRec:    ysctx.FaceRec,
	}, ysctx.Config.Handlers, ysctx.Config.HandlerConfig)
	if err != nil {
		return err
	}
	ysctx.Processors = ps
	return nil
}

func buildNumberCleanerAction(_ context.Context, ysctx *YamdcStartContext) error {
	cleaner, syncLoop, err := buildNumberCleaner(ysctx.HTTPClient, ysctx.Config)
	if err != nil {
		return err
	}
	ysctx.NumberCleaner = cleaner
	ysctx.NumberCleanerSync = syncLoop
	return nil
}

func buildCaptureAction(_ context.Context, ysctx *YamdcStartContext) error {
	cap, err := buildCapture(ysctx.Config, ysctx.CacheStore, ysctx.Searchers, ysctx.CategorySearchers, ysctx.Processors, ysctx.NumberCleaner)
	if err != nil {
		return err
	}
	ysctx.Capture = cap
	return nil
}

func openAppDBAction(_ context.Context, ysctx *YamdcStartContext) error {
	appDB, err := repository.NewSQLite(filepath.Join(ysctx.Config.DataDir, "app", "app.db"))
	if err != nil {
		return fmt.Errorf("init app db failed, err:%w", err)
	}
	ysctx.AppDB = appDB
	ysctx.AddCleanup(func(context.Context) error {
		return appDB.Close()
	})
	return nil
}

func assembleServicesAction(_ context.Context, ysctx *YamdcStartContext) error {
	ysctx.JobRepo = repository.NewJobRepository(ysctx.AppDB.DB())
	ysctx.LogRepo = repository.NewLogRepository(ysctx.AppDB.DB())
	ysctx.ScrapeRepo = repository.NewScrapeDataRepository(ysctx.AppDB.DB())
	ysctx.ScanSvc = scanner.New(ysctx.Config.ScanDir, ysctx.Config.ExtraMediaExts, ysctx.JobRepo, ysctx.NumberCleaner)
	ysctx.JobSvc = job.NewService(ysctx.JobRepo, ysctx.LogRepo, ysctx.ScrapeRepo, ysctx.Capture, ysctx.CacheStore)
	ysctx.MediaSvc = medialib.NewService(ysctx.AppDB.DB(), ysctx.Config.LibraryDir, ysctx.Config.SaveDir)
	ysctx.JobSvc.SetImportGuard(func(ctx context.Context) error {
		if ysctx.MediaSvc.IsMoveRunning() {
			return fmt.Errorf("move to media library is running")
		}
		return nil
	})
	ysctx.API = web.NewAPI(ysctx.JobRepo, ysctx.ScanSvc, ysctx.JobSvc, ysctx.Config.SaveDir, ysctx.MediaSvc, ysctx.CacheStore, ysctx.NumberCleaner)
	return nil
}

func recoverJobsAction(ctx context.Context, ysctx *YamdcStartContext) error {
	if err := ysctx.JobSvc.Recover(ctx); err != nil && ysctx.Logger != nil {
		ysctx.Logger.Error("recover processing jobs failed", zap.Error(err))
	}
	return nil
}

func startMediaServiceAction(ctx context.Context, ysctx *YamdcStartContext) error {
	ysctx.MediaSvc.Start(ctx)
	return nil
}

func startNumberCleanerSyncAction(ctx context.Context, ysctx *YamdcStartContext) error {
	if ysctx.NumberCleanerSync != nil {
		go ysctx.NumberCleanerSync(ctx)
	}
	return nil
}

func serveHTTPAction(_ context.Context, ysctx *YamdcStartContext) error {
	addr := os.Getenv("YAMDC_SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if ysctx.Logger != nil {
		ysctx.Logger.Info("yamdc server start", zap.String("addr", addr), zap.String("scan_dir", ysctx.Config.ScanDir), zap.String("data_dir", ysctx.Config.DataDir))
	}
	if err := http.ListenAndServe(addr, ysctx.API.Handler()); err != nil {
		return fmt.Errorf("listen and serve failed, err:%w", err)
	}
	return nil
}
