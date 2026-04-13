package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/aiengine"
	_ "github.com/xxxsen/yamdc/internal/aiengine/gemini"
	_ "github.com/xxxsen/yamdc/internal/aiengine/ollama"
	"github.com/xxxsen/yamdc/internal/appdeps"
	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/dependency"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/face/pigo"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/searcher"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/translator/ai"
	"github.com/xxxsen/yamdc/internal/translator/google"

	"github.com/spf13/cobra"
	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"go.uber.org/zap"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatalf("execute command failed, err:%v", err)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "yamdc",
		Short:         "YAMDC server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newRunCmd(), newServerCmd(), newRulesetTestCmd(), newPluginTestCmd())
	return cmd
}

func newRunCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one full scraping pass from scan dir to save dir",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Parse(configPath)
			if err != nil {
				return fmt.Errorf("parse config failed, err:%w", err)
			}
			return runCapture(c)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./config.json", "config file")
	return cmd
}

func newServerCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start YAMDC HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Parse(configPath)
			if err != nil {
				return fmt.Errorf("parse config failed, err:%w", err)
			}
			return runServer(c)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./config.json", "config file")
	return cmd
}

func runServer(c *config.Config) error {
	rewriteEnvFlagToConfig(&c.SwitchConfig)
	ysctx := NewYamdcStartContext(c)
	defer func() {
		if err := ysctx.Cleanup(context.Background()); err != nil && ysctx.Logger != nil {
			ysctx.Logger.Error("cleanup yamdc start context failed", zap.Error(err))
		}
	}()
	if err := executeYamdcInitActions(context.Background(), ysctx, newYamdcInitActions()); err != nil {
		return err
	}
	return nil
}

func runCapture(c *config.Config) error {
	rewriteEnvFlagToConfig(&c.SwitchConfig)
	ctx := context.Background()
	logkit := loggerInit(c)
	if logkit != nil {
		defer func() { _ = logkit.Sync() }()
	}
	if err := normalizeDirPaths(c); err != nil {
		return err
	}
	if err := precheckCaptureDir(c); err != nil {
		return err
	}
	logutil.GetLogger(ctx).Info("start capture run", zap.String("scan_dir", c.ScanDir), zap.String("save_dir", c.SaveDir), zap.String("data_dir", c.DataDir))

	cli, err := buildHTTPClient(ctx, c)
	if err != nil {
		return err
	}
	if err := initDependencies(ctx, cli, c.DataDir, c.Dependencies); err != nil {
		return err
	}
	engine, err := buildAIEngine(ctx, cli, c)
	if err != nil {
		return err
	}
	cacheStore, err := store.NewSqliteStorage(filepath.Join(c.DataDir, "cache", "cache.db"))
	if err != nil {
		return err
	}
	if closer, ok := cacheStore.(io.Closer); ok {
		defer func() {
			_ = closer.Close()
		}()
	}
	tr, err := buildTranslator(ctx, c, engine)
	if err != nil {
		logutil.GetLogger(ctx).Error("setup translator failed", zap.Error(err))
	}
	faceRec, err := buildFaceRecognizer(ctx, c, filepath.Join(c.DataDir, "models"))
	if err != nil {
		logutil.GetLogger(ctx).Error("init face recognizer failed", zap.Error(err))
	}
	if _, err := prepareSearcherPlugins(ctx, cli, c); err != nil {
		return err
	}
	searchers, err := buildSearcher(ctx, cli, cacheStore, c, c.Plugins, c.PluginConfig)
	if err != nil {
		return err
	}
	catSearchers, err := buildCatSearcher(ctx, cli, cacheStore, c, c.CategoryPlugins, c.PluginConfig)
	if err != nil {
		return err
	}
	processors, err := buildProcessor(ctx, appdeps.Runtime{
		HTTPClient: cli,
		Storage:    cacheStore,
		Translator: tr,
		AIEngine:   engine,
		FaceRec:    faceRec,
	}, c.Handlers, c.HandlerConfig)
	if err != nil {
		return err
	}
	cleaner, _, err := buildMovieIDCleaner(ctx, cli, c)
	if err != nil {
		return err
	}
	cap, err := buildCapture(c, cacheStore, searcher.NewCategorySearcher(searchers, catSearchers), processors, cleaner)
	if err != nil {
		return err
	}
	if err := cap.Run(ctx); err != nil {
		return err
	}
	logutil.GetLogger(ctx).Info("capture run finished")
	return nil
}

func loggerInit(c *config.Config) *zap.Logger {
	return logger.Init(c.LogConfig.File, c.LogConfig.Level, int(c.LogConfig.FileCount), int(c.LogConfig.FileSize), int(c.LogConfig.KeepDays), c.LogConfig.Console)
}

func buildCapture(c *config.Config, storage store.IStorage, sr searcher.ISearcher, ps []processor.IProcessor, movieIDCleaner movieidcleaner.Cleaner) (*capture.Capture, error) {
	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(c.Naming),
		capture.WithScanDir(c.ScanDir),
		capture.WithSaveDir(c.SaveDir),
		capture.WithSeacher(sr),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithStorage(storage),
		capture.WithExtraMediaExtList(c.ExtraMediaExts),
		capture.WithMovieIDCleaner(movieIDCleaner),
		capture.WithTransalteTitleDiscard(c.TranslateConfig.DiscardTranslatedTitle),
		capture.WithTranslatedPlotDiscard(c.TranslateConfig.DiscardTranslatedPlot),
		capture.WithLinkMode(c.SwitchConfig.EnableLinkMode),
	)
	return capture.New(opts...)
}

func buildCatSearcher(ctx context.Context, cli client.IHTTPClient, storage store.IStorage, c *config.Config, cplgs []config.CategoryPlugin, m map[string]config.PluginConfig) (map[string][]searcher.ISearcher, error) {
	return buildCatSearcherWithCreators(ctx, cli, storage, c, cplgs, m, factory.Snapshot())
}

func configuredSearcherPluginSources(raw []config.SearcherPluginSource) []config.SearcherPluginSource {
	out := make([]config.SearcherPluginSource, 0, len(raw))
	for _, item := range raw {
		if strings.TrimSpace(item.Location) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func hasMovieIDRulesetSource(c *config.Config) bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.MovieIDRulesetConfig.Location) != ""
}

func logSearcherPluginConfigMissing(ctx context.Context) {
	logutil.GetLogger(ctx).Warn("searcher plugin repo is not configured, skip bundle loading; configure searcher_plugin_config.sources to use your own plugin repo")
}

func logMovieIDRulesetConfigMissing(ctx context.Context) {
	logutil.GetLogger(ctx).Warn("movieid ruleset repo is not configured, fallback to passthrough cleaner; configure movieid_ruleset_config to use your own script repo")
}

func buildCatSearcherWithCreators(ctx context.Context, cli client.IHTTPClient, storage store.IStorage, c *config.Config, cplgs []config.CategoryPlugin, m map[string]config.PluginConfig, creators map[string]factory.CreatorFunc) (map[string][]searcher.ISearcher, error) {
	rs := make(map[string][]searcher.ISearcher, len(cplgs))
	for _, plg := range cplgs {
		ss, err := buildSearcherWithCreators(ctx, cli, storage, c, plg.Plugins, m, creators)
		if err != nil {
			return nil, err
		}
		rs[strings.ToUpper(plg.Name)] = ss
	}
	return rs, nil
}

func buildSearcher(ctx context.Context, cli client.IHTTPClient, storage store.IStorage, c *config.Config, plgs []string, m map[string]config.PluginConfig) ([]searcher.ISearcher, error) {
	return buildSearcherWithCreators(ctx, cli, storage, c, plgs, m, factory.Snapshot())
}

func buildSearcherWithCreators(ctx context.Context, cli client.IHTTPClient, storage store.IStorage, c *config.Config, plgs []string, m map[string]config.PluginConfig, creators map[string]factory.CreatorFunc) ([]searcher.ISearcher, error) {
	rs := make([]searcher.ISearcher, 0, len(plgs))
	defc := config.PluginConfig{
		Disable: false,
	}
	for _, name := range plgs {
		plugc, ok := m[name]
		if !ok {
			plugc = defc
		}
		if plugc.Disable {
			logutil.GetLogger(ctx).Info("plugin is disabled, skip create", zap.String("plugin", name))
			continue
		}
		cr, ok := creators[name]
		if !ok {
			return nil, fmt.Errorf("create plugin failed, name:%s, err:plugin not found", name)
		}
		plg, err := cr(struct{}{})
		if err != nil {
			return nil, fmt.Errorf("create plugin failed, name:%s, err:%w", name, err)
		}
		sr, err := searcher.NewDefaultSearcher(name, plg,
			searcher.WithHTTPClient(cli),
			searcher.WithStorage(storage),
			searcher.WithSearchCache(c.SwitchConfig.EnableSearchMetaCache),
		)
		if err != nil {
			return nil, fmt.Errorf("create searcher failed, plugin:%s, err:%w", name, err)
		}
		logutil.GetLogger(ctx).Info("create search succ", zap.String("plugin", name), zap.Strings("domains", plg.OnGetHosts(ctx)))
		rs = append(rs, sr)
	}
	return rs, nil
}

func buildProcessor(ctx context.Context, deps appdeps.Runtime, hs []string, m map[string]config.HandlerConfig) ([]processor.IProcessor, error) {
	rs := make([]processor.IProcessor, 0, len(hs))
	defc := config.HandlerConfig{
		Disable: false,
	}
	for _, name := range hs {
		handlec, ok := m[name]
		if !ok {
			handlec = defc
		}
		if handlec.Disable {
			logutil.GetLogger(ctx).Info("handler is disabled, skip create", zap.String("handler", name))
			continue
		}
		h, err := handler.CreateHandler(name, handlec.Args, deps)
		if err != nil {
			return nil, fmt.Errorf("create handler failed, name:%s, err:%w", name, err)
		}
		p := processor.NewProcessor(name, h)
		logutil.GetLogger(ctx).Info("create processor succ", zap.String("handler", name))
		rs = append(rs, p)
	}
	return rs, nil
}

func buildSearcherDebugger(cli client.IHTTPClient, storage store.IStorage, cleaner movieidcleaner.Cleaner, c *config.Config) *searcher.Debugger {
	categoryPlugins := make(map[string][]string, len(c.CategoryPlugins))
	for _, item := range c.CategoryPlugins {
		categoryPlugins[item.Name] = append([]string(nil), item.Plugins...)
	}
	return searcher.NewDebugger(cli, storage, cleaner, c.Plugins, categoryPlugins)
}

func prepareSearcherPlugins(ctx context.Context, cli client.IHTTPClient, c *config.Config) (*pluginbundle.Manager, error) {
	sourcesCfg := configuredSearcherPluginSources(c.SearcherPluginConfig.Sources)
	if len(sourcesCfg) == 0 {
		logSearcherPluginConfigMissing(ctx)
		return nil, nil
	}
	sources := make([]pluginbundle.Source, 0, len(sourcesCfg))
	for _, source := range sourcesCfg {
		item := pluginbundle.Source{
			SourceType: source.SourceType,
			Location:   source.Location,
		}
		if strings.ToLower(strings.TrimSpace(item.SourceType)) == "" || strings.EqualFold(item.SourceType, basebundle.SourceTypeLocal) {
			resolved, err := resolveBundleSourcePath(c.DataDir, item.Location)
			if err != nil {
				return nil, err
			}
			item.Location = resolved
			item.SourceType = basebundle.SourceTypeLocal
		}
		sources = append(sources, item)
	}
	manager, err := pluginbundle.NewManager("searcher_plugin", c.DataDir, cli, sources, func(ctx context.Context, resolved *pluginbundle.ResolvedBundle, _ []string) error {
		pluginyaml.SyncBundle(resolved.Plugins)
		applyResolvedSearcherPluginBundle(ctx, c, resolved)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := manager.Start(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}

func applyResolvedSearcherPluginBundle(ctx context.Context, c *config.Config, resolved *pluginbundle.ResolvedBundle) {
	defaultPlugins, categoryPlugins := resolvedPluginConfig(resolved)
	for _, warning := range resolved.Warnings {
		logutil.GetLogger(ctx).Warn("plugin bundle conflict", zap.String("detail", warning))
	}
	c.Plugins = defaultPlugins
	c.CategoryPlugins = categoryPlugins
	logutil.GetLogger(ctx).Info("load searcher plugin bundles", zap.Strings("default_plugins", c.Plugins), zap.Int("category_count", len(c.CategoryPlugins)))
}

func categoryPluginMap(items []config.CategoryPlugin) map[string][]string {
	out := make(map[string][]string, len(items))
	for _, item := range items {
		out[item.Name] = append([]string(nil), item.Plugins...)
	}
	return out
}

func resolvedPluginConfig(resolved *pluginbundle.ResolvedBundle) ([]string, []config.CategoryPlugin) {
	if resolved == nil {
		return nil, nil
	}
	defaultPlugins := append([]string(nil), resolved.DefaultPlugins...)
	categoryPlugins := make([]config.CategoryPlugin, 0, len(resolved.CategoryChains))
	categoryNames := make([]string, 0, len(resolved.CategoryChains))
	for category := range resolved.CategoryChains {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)
	for _, category := range categoryNames {
		categoryPlugins = append(categoryPlugins, config.CategoryPlugin{
			Name:    category,
			Plugins: append([]string(nil), resolved.CategoryChains[category]...),
		})
	}
	return defaultPlugins, categoryPlugins
}

func precheckCaptureDir(c *config.Config) error {
	if len(c.DataDir) == 0 {
		return fmt.Errorf("no data dir")
	}
	if len(c.ScanDir) == 0 {
		return fmt.Errorf("no scan dir")
	}
	if len(c.SaveDir) == 0 {
		return fmt.Errorf("no save dir")
	}
	return nil
}

func precheckServerDir(c *config.Config) error {
	if err := precheckCaptureDir(c); err != nil {
		return err
	}
	if len(c.LibraryDir) == 0 {
		return fmt.Errorf("no library dir")
	}
	return nil
}

func normalizeDirPaths(c *config.Config) error {
	var err error
	if c.DataDir != "" {
		c.DataDir, err = filepath.Abs(c.DataDir)
		if err != nil {
			return err
		}
	}
	if c.ScanDir != "" {
		c.ScanDir, err = filepath.Abs(c.ScanDir)
		if err != nil {
			return err
		}
	}
	if c.SaveDir != "" {
		c.SaveDir, err = filepath.Abs(c.SaveDir)
		if err != nil {
			return err
		}
	}
	if c.LibraryDir != "" {
		c.LibraryDir, err = filepath.Abs(c.LibraryDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func initDependencies(ctx context.Context, cli client.IHTTPClient, datadir string, cdeps []config.Dependency) error {
	deps := make([]*dependency.Dependency, 0, len(cdeps))
	for _, item := range cdeps {
		deps = append(deps, &dependency.Dependency{
			URL:     item.Link,
			Target:  filepath.Join(datadir, item.RelPath),
			Refresh: item.Refresh,
		})
	}
	return dependency.Resolve(ctx, cli, deps)
}

func buildFaceRecognizer(ctx context.Context, c *config.Config, models string) (face.IFaceRec, error) {
	impls := make([]face.IFaceRec, 0, 2)
	var faceRecCreator = make([]func() (face.IFaceRec, error), 0, 2)
	if c.SwitchConfig.EnablePigoFaceRecognizer {
		faceRecCreator = append(faceRecCreator, func() (face.IFaceRec, error) {
			return pigo.NewPigo(models)
		})
	}
	for index, creator := range faceRecCreator {
		impl, err := creator()
		if err != nil {
			logutil.GetLogger(ctx).Error("create face rec impl failed", zap.Int("index", index), zap.Error(err))
			continue
		}
		logutil.GetLogger(ctx).Info("use face recognizer", zap.String("name", impl.Name()))
		impls = append(impls, impl)
	}
	if len(impls) == 0 {
		return nil, fmt.Errorf("no face rec impl inited")
	}
	return face.NewGroup(impls), nil
}

func buildHTTPClient(ctx context.Context, c *config.Config) (client.IHTTPClient, error) {
	opts := make([]client.Option, 0, 4)
	if c.NetworkConfig.Timeout > 0 {
		opts = append(opts, client.WithTimeout(time.Duration(c.NetworkConfig.Timeout)*time.Second))
	}
	if pxy := c.NetworkConfig.Proxy; len(pxy) > 0 {
		opts = append(opts, client.WithProxy(pxy))
	}
	clientImpl, err := client.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	if !c.FlareSolverrConfig.Enable {
		return clientImpl, nil
	}
	bpc, err := flarerr.New(clientImpl, c.FlareSolverrConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("create flaresolverr client failed, err:%w", err)
	}
	domainList := make([]string, 0, len(c.FlareSolverrConfig.Domains))
	for domain, ok := range c.FlareSolverrConfig.Domains {
		if !ok {
			continue
		}
		domainList = append(domainList, domain)
		logutil.GetLogger(ctx).Debug("add domain to flaresolverr", zap.String("domain", domain))
	}
	flarerr.MustAddToSolverList(bpc, domainList...)
	logutil.GetLogger(ctx).Info("enable flaresolverr client")
	return bpc, nil
}

func buildAIEngine(ctx context.Context, cli client.IHTTPClient, c *config.Config) (aiengine.IAIEngine, error) {
	if len(c.AIEngine.Name) == 0 {
		logutil.GetLogger(ctx).Info("ai engine is disabled, skip init")
		return nil, nil
	}
	engine, err := aiengine.Create(c.AIEngine.Name, c.AIEngine.Args, aiengine.WithHTTPClient(cli))
	if err != nil {
		return nil, fmt.Errorf("create ai engine failed, name:%s, err:%w", c.AIEngine.Name, err)
	}
	return engine, nil
}

func buildTranslator(ctx context.Context, c *config.Config, engine aiengine.IAIEngine) (translator.ITranslator, error) {
	if !c.TranslateConfig.Enable {
		return nil, nil
	}
	allEngines := make(map[string]translator.ITranslator, 2)
	enginec := c.TranslateConfig.EngineConfig
	if enginec.Google.Enable {
		opts := []google.Option{}
		if enginec.Google.UseProxy && c.NetworkConfig.Proxy != "" {
			opts = append(opts, google.WithProxyUrl(c.NetworkConfig.Proxy))
		}
		allEngines[translator.TrNameGoogle] = google.New(opts...)
	}
	if enginec.AI.Enable {
		allEngines[translator.TrNameAI] = ai.New(engine, ai.WithPrompt(enginec.AI.Prompt))
	}
	engineNames := []string{c.TranslateConfig.Engine}
	engineNames = append(engineNames, c.TranslateConfig.Fallback...)
	useEngines := make([]translator.ITranslator, 0, len(engineNames))
	seen := make(map[string]struct{}, len(engineNames))
	for _, name := range engineNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		e, ok := allEngines[name]
		if !ok {
			logutil.GetLogger(ctx).Error("spec engine not found, skip", zap.String("name", name))
			continue
		}
		useEngines = append(useEngines, e)
	}
	if len(useEngines) == 0 {
		return nil, fmt.Errorf("no engine used, need to check engine config")
	}
	return translator.NewGroup(useEngines...), nil
}

func buildMovieIDCleaner(ctx context.Context, cli client.IHTTPClient, c *config.Config) (movieidcleaner.Cleaner, *movieidcleaner.Manager, error) {
	if !hasMovieIDRulesetSource(c) {
		logMovieIDRulesetConfigMissing(ctx)
		return movieidcleaner.NewPassthroughCleaner(), nil, nil
	}
	cc := c.MovieIDRulesetConfig
	sourceType := strings.ToLower(strings.TrimSpace(cc.SourceType))
	if sourceType == "" {
		sourceType = basebundle.SourceTypeLocal
	}
	location := strings.TrimSpace(cc.Location)
	if sourceType == basebundle.SourceTypeLocal {
		resolved, err := resolveRuleSourcePath(c.DataDir, location)
		if err != nil {
			return nil, nil, err
		}
		location = resolved
	}
	runtimeCleaner := movieidcleaner.NewRuntimeCleaner(nil)
	manager, err := movieidcleaner.NewManager(c.DataDir, cli, sourceType, location, func(ctx context.Context, rs *movieidcleaner.RuleSet, files []string) error {
		logutil.GetLogger(ctx).Debug("load movieid rules", zap.Strings("files", files))
		inner, err := movieidcleaner.NewCleaner(rs)
		if err != nil {
			return err
		}
		runtimeCleaner.Swap(inner)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if err := manager.Start(ctx); err != nil {
		return nil, nil, err
	}
	return runtimeCleaner, manager, nil
}

func resolveRuleSourcePath(datadir string, raw string) (string, error) {
	paths := []string{raw, path.Join(datadir, raw)}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() || strings.HasSuffix(strings.ToLower(p), ".yaml") || strings.HasSuffix(strings.ToLower(p), ".yml") {
			return filepath.Abs(p)
		}
	}
	return "", fmt.Errorf("no rule source found in paths, raw:%s", raw)
}

func resolveBundleSourcePath(datadir string, raw string) (string, error) {
	paths := []string{raw, path.Join(datadir, raw)}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return filepath.Abs(p)
		}
	}
	return "", fmt.Errorf("no bundle source found in paths, raw:%s", raw)
}

func rewriteEnvFlagToConfig(c *config.SwitchConfig) {
	//配置项均移到配置文件中, 不再使用环境变量
	if os.Getenv("ENABLE_SEARCH_META_CACHE") == "false" {
		c.EnableSearchMetaCache = false
	}
	if os.Getenv("ENABLE_PIGO_FACE_RECOGNIZER") == "false" {
		c.EnablePigoFaceRecognizer = false
	}
}
