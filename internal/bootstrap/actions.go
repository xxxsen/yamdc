package bootstrap

// 本文件只负责"声明式地列出 server 模式的启动动作"。
// 具体每个 Run 实现分散到同包的 actions_*.go:
//   - actions_convert.go      类型转换 (to*/from*)
//   - actions_infra.go        基础设施层 (dir/logger/http client/...)
//   - actions_runtime.go      运行时组件 (ai/translator/face rec)
//   - actions_domain.go       业务域 (searcher/processor/capture/...)
//   - actions_app.go          应用层 (db/service/healthz/http serve)
//   - actions_plugin_bundle.go YAML 插件 bundle 装配与热重载
//   - actions_helpers.go      通用 helper
//
// 一切"什么顺序跑什么"集中在这里, 便于读者快速理解 server 启动拓扑。

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
			Name:     "register_cron_jobs",
			Requires: []string{ResServices},
			Run:      registerCronJobsAction,
		},
		{
			Name:     "start_cron_scheduler",
			Requires: []string{ResServices},
			Run:      startCronSchedulerAction,
		},
		{
			Name:     "serve_http",
			Requires: []string{ResAPI},
			Run:      serveHTTPAction,
		},
	}
}
