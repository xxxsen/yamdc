package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/appdeps"
	bootapp "github.com/xxxsen/yamdc/internal/bootstrap/app"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/bootstrap/infra"
	bootrt "github.com/xxxsen/yamdc/internal/bootstrap/runtime"
	"github.com/xxxsen/yamdc/internal/bootstrap/server"
	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	plugineditor "github.com/xxxsen/yamdc/internal/searcher/plugin/editor"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"github.com/xxxsen/yamdc/internal/web"
	"go.uber.org/zap"
)

// Resource name constants used in Requires/Provides declarations.
const (
	ResDirPaths         = "dir_paths"
	ResLogger           = "logger"
	ResHTTPClient       = "http_client"
	ResBrowserClient    = "browser_client"
	ResDependencies     = "dependencies"
	ResAIEngine         = "ai_engine"
	ResCacheStore       = "cache_store"
	ResTranslator       = "translator"
	ResFaceRec          = "face_rec"
	ResSearchers        = "searchers"
	ResProcessors       = "processors"
	ResMovieIDCleaner   = "movieid_cleaner"
	ResSearcherDebugger = "searcher_debugger"
	ResHandlerDebugger  = "handler_debugger"
	ResCapture          = "capture"
	ResAppDB            = "app_db"
	ResServices         = "services"
	ResAPI              = "api"
)

// NewServerActions returns the ordered list of bootstrap actions for server mode,
// with explicit Requires/Provides dependency declarations.
func NewServerActions() []InitAction { //nolint:funlen // declarative action list
	return []InitAction{
		{
			Name:     "normalize_dir_paths",
			Provides: []string{ResDirPaths},
			Run:      normalizeDirPathsAction,
		},
		{
			Name:     "init_logger",
			Requires: []string{ResDirPaths},
			Provides: []string{ResLogger},
			Run:      initLoggerAction,
		},
		{
			Name:     "precheck_dirs",
			Requires: []string{ResDirPaths},
			Run:      precheckDirsServerAction,
		},
		{
			Name:     "build_http_client",
			Provides: []string{ResHTTPClient},
			Run:      buildHTTPClientAction,
		},
		{
			Name:     "build_browser_client",
			Requires: []string{ResHTTPClient},
			Provides: []string{ResBrowserClient},
			Run:      buildBrowserClientAction,
		},
		{
			Name:     "init_dependencies",
			Requires: []string{ResHTTPClient, ResDirPaths},
			Provides: []string{ResDependencies},
			Run:      initDependenciesAction,
		},
		{
			Name:     "build_ai_engine",
			Requires: []string{ResHTTPClient},
			Provides: []string{ResAIEngine},
			Run:      buildAIEngineAction,
		},
		{
			Name:     "build_cache_store",
			Requires: []string{ResDirPaths},
			Provides: []string{ResCacheStore},
			Run:      buildCacheStoreAction,
		},
		{
			Name:     "build_translator",
			Requires: []string{ResAIEngine},
			Provides: []string{ResTranslator},
			Run:      buildTranslatorAction,
		},
		{
			Name:     "build_face_recognizer",
			Requires: []string{ResDirPaths},
			Provides: []string{ResFaceRec},
			Run:      buildFaceRecognizerAction,
		},
		{
			Name:     "build_searchers",
			Requires: []string{ResHTTPClient, ResCacheStore},
			Provides: []string{ResSearchers},
			Run:      buildSearchersAction,
		},
		{
			Name:     "build_processors",
			Requires: []string{ResHTTPClient, ResCacheStore, ResTranslator, ResAIEngine, ResFaceRec},
			Provides: []string{ResProcessors},
			Run:      buildProcessorsAction,
		},
		{
			Name:     "build_movieid_cleaner",
			Requires: []string{ResHTTPClient},
			Provides: []string{ResMovieIDCleaner},
			Run:      buildMovieIDCleanerAction,
		},
		{
			Name:     "build_searcher_debugger",
			Requires: []string{ResHTTPClient, ResCacheStore, ResMovieIDCleaner},
			Provides: []string{ResSearcherDebugger},
			Run:      buildSearcherDebuggerAction,
		},
		{
			Name: "build_handler_debugger",
			Requires: []string{
				ResHTTPClient, ResCacheStore, ResTranslator,
				ResAIEngine, ResFaceRec, ResMovieIDCleaner,
			},
			Provides: []string{ResHandlerDebugger},
			Run:      buildHandlerDebuggerAction,
		},
		{
			Name:     "build_capture",
			Requires: []string{ResSearchers, ResProcessors, ResMovieIDCleaner, ResCacheStore},
			Provides: []string{ResCapture},
			Run:      buildCaptureAction,
		},
		{
			Name:     "open_app_db",
			Requires: []string{ResDirPaths},
			Provides: []string{ResAppDB},
			Run:      openAppDBAction,
		},
		{
			Name: "assemble_services",
			Requires: []string{
				ResAppDB, ResCapture, ResCacheStore,
				ResMovieIDCleaner, ResSearcherDebugger, ResHandlerDebugger,
			},
			Provides: []string{ResServices, ResAPI},
			Run:      assembleServicesAction,
		},
		{
			Name:     "recover_jobs",
			Requires: []string{ResServices},
			Run:      recoverJobsAction,
		},
		{
			Name:     "start_media_service",
			Requires: []string{ResServices},
			Run:      startMediaServiceAction,
		},
		{
			Name:     "serve_http",
			Requires: []string{ResAPI},
			Run:      serveHTTPAction,
		},
	}
}

// --- config → sub-package type conversions ---

func toHTTPClientConfig(c *config.Config) infra.HTTPClientConfig {
	return infra.HTTPClientConfig{
		TimeoutSec: c.NetworkConfig.Timeout,
		Proxy:      c.NetworkConfig.Proxy,
	}
}

func toDependencySpecs(deps []config.Dependency) []infra.DependencySpec {
	out := make([]infra.DependencySpec, 0, len(deps))
	for _, d := range deps {
		out = append(out, infra.DependencySpec{
			URL: d.Link, RelPath: d.RelPath, Refresh: d.Refresh,
		})
	}
	return out
}

func toTranslatorConfig(c *config.Config) bootrt.TranslatorConfig {
	ec := c.TranslateConfig.EngineConfig
	return bootrt.TranslatorConfig{
		Engine:   c.TranslateConfig.Engine,
		Fallback: c.TranslateConfig.Fallback,
		Proxy:    c.NetworkConfig.Proxy,
		Google: bootrt.GoogleTranslatorConfig{
			Enable:   ec.Google.Enable,
			UseProxy: ec.Google.UseProxy,
		},
		AI: bootrt.AITranslatorConfig{
			Enable: ec.AI.Enable,
			Prompt: ec.AI.Prompt,
		},
	}
}

func toPluginOptions(m map[string]config.PluginConfig) map[string]domain.PluginOption {
	out := make(map[string]domain.PluginOption, len(m))
	for k, v := range m {
		out[k] = domain.PluginOption{Disable: v.Disable}
	}
	return out
}

func toHandlerOptions(m map[string]config.HandlerConfig) map[string]domain.HandlerOption {
	out := make(map[string]domain.HandlerOption, len(m))
	for k, v := range m {
		out[k] = domain.HandlerOption{Disable: v.Disable, Args: v.Args}
	}
	return out
}

func toCategoryPlugins(items []config.CategoryPlugin) []domain.CategoryPlugin {
	out := make([]domain.CategoryPlugin, 0, len(items))
	for _, item := range items {
		out = append(out, domain.CategoryPlugin{
			Name: item.Name, Plugins: item.Plugins,
		})
	}
	return out
}

func toPluginSources(items []config.SearcherPluginSource) []domain.PluginSource {
	out := make([]domain.PluginSource, 0, len(items))
	for _, item := range items {
		out = append(out, domain.PluginSource{
			SourceType: item.SourceType, Location: item.Location,
		})
	}
	return out
}

func toCaptureConfig(c *config.Config) domain.CaptureConfig {
	return domain.CaptureConfig{
		Naming:                 c.Naming,
		ScanDir:                c.ScanDir,
		SaveDir:                c.SaveDir,
		ExtraMediaExts:         c.ExtraMediaExts,
		DiscardTranslatedTitle: c.TranslateConfig.DiscardTranslatedTitle,
		DiscardTranslatedPlot:  c.TranslateConfig.DiscardTranslatedPlot,
		EnableLinkMode:         c.SwitchConfig.EnableLinkMode,
	}
}

func categoryPluginStringMap(c *config.Config) map[string][]string {
	m := make(map[string][]string, len(c.CategoryPlugins))
	for _, item := range c.CategoryPlugins {
		m[item.Name] = append([]string(nil), item.Plugins...)
	}
	return m
}

// --- infra action wrappers ---

func normalizeDirPathsAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	if err := infra.NormalizeDirPaths(
		&c.DataDir, &c.ScanDir, &c.SaveDir, &c.LibraryDir,
	); err != nil {
		return fmt.Errorf("normalize dir paths: %w", err)
	}
	return nil
}

func initLoggerAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	sc.Infra.Logger = infra.InitLogger(
		c.LogConfig.File, c.LogConfig.Level,
		int(c.LogConfig.FileCount), //nolint:gosec // bounded config value
		int(c.LogConfig.FileSize),  //nolint:gosec // bounded config value
		int(c.LogConfig.KeepDays),
		c.LogConfig.Console,
	)
	return nil
}

func precheckDirsServerAction(_ context.Context, sc *StartContext) error {
	if err := config.ValidateForServer(sc.Infra.Config); err != nil {
		return fmt.Errorf("server dir validation failed: %w", err)
	}
	return nil
}

func buildHTTPClientAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	cli, err := infra.BuildHTTPClient(ctx, toHTTPClientConfig(c))
	if err != nil {
		return fmt.Errorf("build http client: %w", err)
	}
	sc.Infra.HTTPClient = cli
	return nil
}

func buildBrowserClientAction(_ context.Context, sc *StartContext) error {
	if sc.Infra.Config.FlareSolverrConfig.Enable {
		sc.Infra.HTTPClient = flarerr.NewHTTPClient(
			sc.Infra.HTTPClient,
			sc.Infra.Config.FlareSolverrConfig.Host,
		)
	}
	nav := browser.NewNavigator(&browser.Config{
		RemoteURL: sc.Infra.Config.BrowserConfig.RemoteURL,
		DataDir:   sc.Infra.Config.DataDir,
		Proxy:     sc.Infra.Config.NetworkConfig.Proxy,
	})
	sc.Cleanup.Add("browser_navigator", func(context.Context) error {
		return nav.Close()
	})
	sc.Infra.HTTPClient = browser.NewHTTPClient(sc.Infra.HTTPClient, nav)
	return nil
}

func initDependenciesAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	if err := infra.InitDependencies(
		ctx, sc.Infra.HTTPClient, c.DataDir, toDependencySpecs(c.Dependencies),
	); err != nil {
		return fmt.Errorf("init dependencies: %w", err)
	}
	return nil
}

func buildCacheStoreAction(ctx context.Context, sc *StartContext) error {
	cacheStore, err := infra.BuildCacheStore(ctx, sc.Infra.Config.DataDir)
	if err != nil {
		return fmt.Errorf("build cache store: %w", err)
	}
	sc.Infra.CacheStore = cacheStore
	if closer, ok := cacheStore.(io.Closer); ok {
		sc.Cleanup.Add("cache_store", func(context.Context) error {
			return closer.Close()
		})
	}
	return nil
}

// --- runtime action wrappers ---

func buildAIEngineAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	engine, err := bootrt.BuildAIEngine(ctx, sc.Infra.HTTPClient, c.AIEngine.Name, c.AIEngine.Args)
	if err != nil && !errors.Is(err, bootrt.ErrAIEngineNotConfigured) {
		return fmt.Errorf("build ai engine: %w", err)
	}
	sc.Runtime.AIEngine = engine
	return nil
}

func buildTranslatorAction(ctx context.Context, sc *StartContext) error {
	if !sc.Infra.Config.TranslateConfig.Enable {
		return nil
	}
	tr, err := bootrt.BuildTranslator(ctx, toTranslatorConfig(sc.Infra.Config), sc.Runtime.AIEngine)
	if err != nil && !errors.Is(err, bootrt.ErrTranslatorNotConfigured) {
		logOptionalSetupFailure(ctx, sc, "setup translator failed", err)
		return nil
	}
	sc.Runtime.Translator = tr
	return nil
}

func buildFaceRecognizerAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	faceRec, err := bootrt.BuildFaceRecognizer(
		ctx, c.SwitchConfig.EnablePigoFaceRecognizer,
		filepath.Join(c.DataDir, "models"),
	)
	if err != nil {
		logOptionalSetupFailure(ctx, sc, "init face recognizer failed", err)
		return nil
	}
	sc.Runtime.FaceRec = faceRec
	return nil
}

// --- domain action wrappers ---

func buildSearchersAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	runtimeSearcher := searcher.NewCategorySearcher(nil, nil)
	sc.Domain.RuntimeSearcher = runtimeSearcher
	sources := toPluginSources(c.SearcherPluginConfig.Sources)
	if len(domain.ConfiguredPluginSources(sources)) != 0 {
		manager, err := prepareSearcherPluginsForServer(ctx, sc, runtimeSearcher)
		if err != nil {
			return err
		}
		sc.Domain.PluginBundleMgr = manager
		return nil
	}
	domain.LogSearcherPluginConfigMissing(ctx)
	plugOpts := toPluginOptions(c.PluginConfig)
	ss, err := domain.BuildSearcher(
		ctx, sc.Infra.HTTPClient, sc.Infra.CacheStore,
		c.SwitchConfig.EnableSearchMetaCache, c.Plugins, plugOpts,
	)
	if err != nil {
		return fmt.Errorf("build searchers: %w", err)
	}
	catSs, err := domain.BuildCatSearcher(
		ctx, sc.Infra.HTTPClient, sc.Infra.CacheStore,
		c.SwitchConfig.EnableSearchMetaCache, toCategoryPlugins(c.CategoryPlugins), plugOpts,
	)
	if err != nil {
		return fmt.Errorf("build category searchers: %w", err)
	}
	sc.Domain.Searchers = ss
	sc.Domain.CategorySearchers = catSs
	runtimeSearcher.Swap(ss, catSs)
	return nil
}

func buildProcessorsAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	ps, err := domain.BuildProcessor(ctx, appdeps.Runtime{
		HTTPClient: sc.Infra.HTTPClient,
		Storage:    sc.Infra.CacheStore,
		Translator: sc.Runtime.Translator,
		AIEngine:   sc.Runtime.AIEngine,
		FaceRec:    sc.Runtime.FaceRec,
	}, c.Handlers, toHandlerOptions(c.HandlerConfig))
	if err != nil {
		return fmt.Errorf("build processors: %w", err)
	}
	sc.Domain.Processors = ps
	return nil
}

func buildMovieIDCleanerAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	cleaner, _, err := domain.BuildMovieIDCleaner(
		ctx, sc.Infra.HTTPClient,
		c.DataDir, c.MovieIDRulesetConfig.SourceType, c.MovieIDRulesetConfig.Location,
	)
	if err != nil {
		return fmt.Errorf("build movieid cleaner: %w", err)
	}
	sc.Domain.MovieIDCleaner = cleaner
	return nil
}

func buildSearcherDebuggerAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	sc.Domain.SearcherDebugger = domain.BuildSearcherDebugger(
		sc.Infra.HTTPClient, sc.Infra.CacheStore,
		sc.Domain.MovieIDCleaner, c.Plugins, categoryPluginStringMap(c),
	)
	return nil
}

func buildHandlerDebuggerAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	handlerOptions := make(
		map[string]handler.DebugHandlerOption,
		len(c.HandlerConfig),
	)
	for name, cfg := range c.HandlerConfig {
		handlerOptions[name] = handler.DebugHandlerOption{
			Disable: cfg.Disable,
			Args:    cfg.Args,
		}
	}
	sc.Domain.HandlerDebugger = handler.NewDebugger(appdeps.Runtime{
		HTTPClient: sc.Infra.HTTPClient,
		Storage:    sc.Infra.CacheStore,
		Translator: sc.Runtime.Translator,
		AIEngine:   sc.Runtime.AIEngine,
		FaceRec:    sc.Runtime.FaceRec,
	}, sc.Domain.MovieIDCleaner, c.Handlers, handlerOptions)
	return nil
}

func buildCaptureAction(_ context.Context, sc *StartContext) error {
	var useSearcher searcher.ISearcher
	if sc.Domain.RuntimeSearcher != nil {
		useSearcher = sc.Domain.RuntimeSearcher
	} else {
		useSearcher = searcher.NewCategorySearcher(
			sc.Domain.Searchers, sc.Domain.CategorySearchers,
		)
	}
	capt, err := domain.BuildCapture(
		toCaptureConfig(sc.Infra.Config), sc.Infra.CacheStore, useSearcher,
		sc.Domain.Processors, sc.Domain.MovieIDCleaner,
	)
	if err != nil {
		return fmt.Errorf("build capture: %w", err)
	}
	sc.Domain.Capture = capt
	return nil
}

// --- app action wrappers ---

func openAppDBAction(ctx context.Context, sc *StartContext) error {
	appDB, err := bootapp.OpenAppDB(ctx, sc.Infra.Config.DataDir)
	if err != nil {
		return fmt.Errorf("open app db: %w", err)
	}
	sc.App.AppDB = appDB
	sc.Cleanup.Add("app_db", func(context.Context) error {
		return appDB.Close()
	})
	return nil
}

func assembleServicesAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	sc.App.JobRepo = repository.NewJobRepository(sc.App.AppDB.DB())
	sc.App.LogRepo = repository.NewLogRepository(sc.App.AppDB.DB())
	sc.App.ScrapeRepo = repository.NewScrapeDataRepository(sc.App.AppDB.DB())
	sc.App.ScanSvc = scanner.New(
		c.ScanDir, c.ExtraMediaExts,
		sc.App.JobRepo, sc.Domain.MovieIDCleaner,
	)
	sc.App.JobSvc = job.NewService(
		sc.App.JobRepo, sc.App.LogRepo, sc.App.ScrapeRepo,
		sc.Domain.Capture, sc.Infra.CacheStore,
	)
	sc.App.MediaSvc = medialib.NewService(
		sc.App.AppDB.DB(), c.LibraryDir, c.SaveDir,
	)
	sc.App.JobSvc.SetImportGuard(func(_ context.Context) error {
		if sc.App.MediaSvc.IsMoveRunning() {
			return ErrMoveToMediaLibRunning
		}
		return nil
	})
	editorSvc, err := plugineditor.NewService(sc.Infra.HTTPClient)
	if err != nil {
		return fmt.Errorf("init plugin editor service failed, err:%w", err)
	}
	sc.App.API = web.NewAPI(
		sc.App.JobRepo,
		sc.App.ScanSvc,
		sc.App.JobSvc,
		c.SaveDir,
		sc.App.MediaSvc,
		sc.Infra.CacheStore,
		sc.Domain.MovieIDCleaner,
		sc.Domain.SearcherDebugger,
		sc.Domain.HandlerDebugger,
		editorSvc,
	)
	return nil
}

func recoverJobsAction(ctx context.Context, sc *StartContext) error {
	if err := sc.App.JobSvc.Recover(ctx); err != nil && sc.Infra.Logger != nil {
		sc.Infra.Logger.Error("recover processing jobs failed", zap.Error(err))
	}
	return nil
}

func startMediaServiceAction(ctx context.Context, sc *StartContext) error {
	sc.App.MediaSvc.Start(ctx)
	return nil
}

// --- server action wrapper ---

func serveHTTPAction(_ context.Context, sc *StartContext) error {
	if err := server.ServeHTTP(
		sc.App.API, sc.Infra.Logger,
		sc.Infra.Config.ScanDir, sc.Infra.Config.DataDir,
	); err != nil {
		return fmt.Errorf("serve http: %w", err)
	}
	return nil
}

// --- server-mode wiring that needs StartContext ---

func reloadSearcherPluginBundle(
	cbCtx context.Context,
	sc *StartContext,
	runtimeSearcher *searcher.RuntimeCategorySearcher,
	resolved *pluginbundle.ResolvedBundle,
) error {
	c := sc.Infra.Config
	nextDefaultPlugins, nextCategoryPlugins := domain.ResolvedPluginConfig(resolved)
	registerCtx := pluginyaml.BuildRegisterContext(resolved.Plugins)
	creatorSnapshot := registerCtx.Snapshot()
	plugOpts := toPluginOptions(c.PluginConfig)
	ss, err := domain.BuildSearcherWithCreators(
		cbCtx, sc.Infra.HTTPClient, sc.Infra.CacheStore,
		c.SwitchConfig.EnableSearchMetaCache, nextDefaultPlugins, plugOpts, creatorSnapshot,
	)
	if err != nil {
		return fmt.Errorf("rebuild searchers: %w", err)
	}
	catSs, err := domain.BuildCatSearcherWithCreators(
		cbCtx, sc.Infra.HTTPClient, sc.Infra.CacheStore,
		c.SwitchConfig.EnableSearchMetaCache, nextCategoryPlugins, plugOpts, creatorSnapshot,
	)
	if err != nil {
		return fmt.Errorf("rebuild category searchers: %w", err)
	}
	factory.Swap(registerCtx)
	domain.LogPluginBundleWarnings(cbCtx, resolved.Warnings)
	c.Plugins = nextDefaultPlugins
	c.CategoryPlugins = fromDomainCategoryPlugins(nextCategoryPlugins)
	logutil.GetLogger(cbCtx).Info("load searcher plugin bundles",
		zap.Strings("default_plugins", c.Plugins),
		zap.Int("category_count", len(c.CategoryPlugins)),
	)
	sc.Domain.Searchers = ss
	sc.Domain.CategorySearchers = catSs
	runtimeSearcher.Swap(ss, catSs)
	if sc.Domain.SearcherDebugger != nil {
		sc.Domain.SearcherDebugger.SwapState(
			nextDefaultPlugins,
			domain.CategoryPluginMap(nextCategoryPlugins),
			creatorSnapshot,
		)
	}
	logutil.GetLogger(cbCtx).Info("reload searcher plugin runtime",
		zap.Int("default_plugins", len(c.Plugins)),
		zap.Int("category_chains", len(c.CategoryPlugins)),
	)
	return nil
}

func prepareSearcherPluginsForServer(
	ctx context.Context,
	sc *StartContext,
	runtimeSearcher *searcher.RuntimeCategorySearcher,
) (*pluginbundle.Manager, error) {
	c := sc.Infra.Config
	sources := toPluginSources(c.SearcherPluginConfig.Sources)
	if len(domain.ConfiguredPluginSources(sources)) == 0 {
		return nil, domain.ErrNoPluginSources
	}
	bundleSources := make([]pluginbundle.Source, 0, len(sources))
	for _, source := range sources {
		item := pluginbundle.Source{
			SourceType: source.SourceType,
			Location:   source.Location,
		}
		st := strings.TrimSpace(item.SourceType)
		if st == "" || strings.EqualFold(st, "local") {
			resolved, err := domain.ResolveBundleSourcePath(c.DataDir, item.Location)
			if err != nil {
				return nil, fmt.Errorf("resolve bundle source path: %w", err)
			}
			item.Location = resolved
			item.SourceType = "local"
		}
		bundleSources = append(bundleSources, item)
	}
	manager, err := pluginbundle.NewManager(
		"searcher_plugin", c.DataDir, sc.Infra.HTTPClient, bundleSources,
		func(cbCtx context.Context, resolved *pluginbundle.ResolvedBundle, _ []string) error {
			pluginyaml.SyncBundle(resolved.Plugins)
			return reloadSearcherPluginBundle(cbCtx, sc, runtimeSearcher, resolved)
		})
	if err != nil {
		return nil, fmt.Errorf("create plugin bundle manager failed: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("start plugin bundle manager failed: %w", err)
	}
	return manager, nil
}

func fromDomainCategoryPlugins(items []domain.CategoryPlugin) []config.CategoryPlugin {
	out := make([]config.CategoryPlugin, 0, len(items))
	for _, item := range items {
		out = append(out, config.CategoryPlugin{
			Name: item.Name, Plugins: item.Plugins,
		})
	}
	return out
}

func logOptionalSetupFailure(ctx context.Context, sc *StartContext, message string, err error) {
	if sc.Infra.Logger != nil {
		sc.Infra.Logger.Error(message, zap.Error(err))
		return
	}
	logutil.GetLogger(ctx).Error(message, zap.Error(err))
}
