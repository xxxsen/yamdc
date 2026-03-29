package main

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/aiengine"
	_ "github.com/xxxsen/yamdc/internal/aiengine/gemini"
	_ "github.com/xxxsen/yamdc/internal/aiengine/ollama"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/dependency"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/face/pigo"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/translator/ai"
	"github.com/xxxsen/yamdc/internal/translator/google"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	_ "github.com/xxxsen/yamdc/internal/searcher/plugin/register"
	"go.uber.org/zap"

	"github.com/samber/lo"
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
	cmd.AddCommand(newServerCmd())
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

func loggerInit(c *config.Config) *zap.Logger {
	return logger.Init(c.LogConfig.File, c.LogConfig.Level, int(c.LogConfig.FileCount), int(c.LogConfig.FileSize), int(c.LogConfig.KeepDays), true)
}

func buildCapture(c *config.Config, storage store.IStorage, ss []searcher.ISearcher, catSs map[string][]searcher.ISearcher, ps []processor.IProcessor, numCleaner numbercleaner.Cleaner) (*capture.Capture, error) {
	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(c.Naming),
		capture.WithScanDir(c.ScanDir),
		capture.WithSaveDir(c.SaveDir),
		capture.WithSeacher(searcher.NewCategorySearcher(ss, catSs)),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithStorage(storage),
		capture.WithExtraMediaExtList(c.ExtraMediaExts),
		capture.WithNumberCleaner(numCleaner),
		capture.WithTransalteTitleDiscard(c.TranslateConfig.DiscardTranslatedTitle),
		capture.WithTranslatedPlotDiscard(c.TranslateConfig.DiscardTranslatedPlot),
		capture.WithLinkMode(c.SwitchConfig.EnableLinkMode),
	)
	return capture.New(opts...)
}

func buildCatSearcher(cli client.IHTTPClient, storage store.IStorage, c *config.Config, cplgs []config.CategoryPlugin, m map[string]config.PluginConfig) (map[string][]searcher.ISearcher, error) {
	rs := make(map[string][]searcher.ISearcher, len(cplgs))
	for _, plg := range cplgs {
		ss, err := buildSearcher(cli, storage, c, plg.Plugins, m)
		if err != nil {
			return nil, err
		}
		rs[strings.ToUpper(plg.Name)] = ss
	}
	return rs, nil
}

func buildSearcher(cli client.IHTTPClient, storage store.IStorage, c *config.Config, plgs []string, m map[string]config.PluginConfig) ([]searcher.ISearcher, error) {
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
			logutil.GetLogger(context.Background()).Info("plugin is disabled, skip create", zap.String("plugin", name))
			continue
		}
		plg, err := factory.CreatePlugin(name, struct{}{})
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
		logutil.GetLogger(context.Background()).Info("create search succ", zap.String("plugin", name), zap.Strings("domains", plg.OnGetHosts(context.Background())))
		rs = append(rs, sr)
	}
	return rs, nil
}

func buildProcessor(deps appdeps.Runtime, hs []string, m map[string]config.HandlerConfig) ([]processor.IProcessor, error) {
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
			logutil.GetLogger(context.Background()).Info("handler is disabled, skip create", zap.String("handler", name))
			continue
		}
		h, err := handler.CreateHandler(name, handlec.Args, deps)
		if err != nil {
			return nil, fmt.Errorf("create handler failed, name:%s, err:%w", name, err)
		}
		p := processor.NewProcessor(name, h)
		logutil.GetLogger(context.Background()).Info("create processor succ", zap.String("handler", name))
		rs = append(rs, p)
	}
	return rs, nil
}

func precheckDir(c *config.Config) error {
	if len(c.DataDir) == 0 {
		return fmt.Errorf("no data dir")
	}
	if len(c.ScanDir) == 0 {
		return fmt.Errorf("no scan dir")
	}
	if len(c.SaveDir) == 0 {
		return fmt.Errorf("no save dir")
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

func initDependencies(cli client.IHTTPClient, datadir string, cdeps []config.Dependency) error {
	deps := make([]*dependency.Dependency, 0, len(cdeps))
	for _, item := range cdeps {
		deps = append(deps, &dependency.Dependency{
			URL:     item.Link,
			Target:  filepath.Join(datadir, item.RelPath),
			Refresh: item.Refresh,
		})
	}
	return dependency.Resolve(cli, deps)
}

func buildFaceRecognizer(c *config.Config, models string) (face.IFaceRec, error) {
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
			logutil.GetLogger(context.Background()).Error("create face rec impl failed", zap.Int("index", index), zap.Error(err))
			continue
		}
		logutil.GetLogger(context.Background()).Info("use face recognizer", zap.String("name", impl.Name()))
		impls = append(impls, impl)
	}
	if len(impls) == 0 {
		return nil, fmt.Errorf("no face rec impl inited")
	}
	return face.NewGroup(impls), nil
}

func buildHTTPClient(c *config.Config) (client.IHTTPClient, error) {
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
	if c.FlareSolverrConfig.Enable {
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
			logutil.GetLogger(context.Background()).Debug("add domain to flaresolverr", zap.String("domain", domain))
		}
		flarerr.MustAddToSolverList(bpc, domainList...)
		clientImpl = bpc
		logutil.GetLogger(context.Background()).Info("enable flaresolverr client")
	}
	return clientImpl, nil
}

func buildAIEngine(cli client.IHTTPClient, c *config.Config) (aiengine.IAIEngine, error) {
	if len(c.AIEngine.Name) == 0 {
		logutil.GetLogger(context.Background()).Info("ai engine is disabled, skip init")
		return nil, nil
	}
	engine, err := aiengine.Create(c.AIEngine.Name, c.AIEngine.Args, aiengine.WithHTTPClient(cli))
	if err != nil {
		return nil, fmt.Errorf("create ai engine failed, name:%s, err:%w", c.AIEngine.Name, err)
	}
	return engine, nil
}

func buildTranslator(c *config.Config, engine aiengine.IAIEngine) (translator.ITranslator, error) {
	if !c.TranslateConfig.Enable {
		return nil, nil
	}
	allEngines := make(map[string]translator.ITranslator, 4)
	enginec := c.TranslateConfig.EngineConfig
	if enginec.Google.Enable {
		opts := []google.Option{}
		if enginec.Google.UseProxy && len(c.NetworkConfig.Proxy) > 0 {
			opts = append(opts, google.WithProxyUrl(c.NetworkConfig.Proxy))
		}
		allEngines[translator.TrNameGoogle] = google.New(opts...)
	}
	if enginec.AI.Enable {
		allEngines[translator.TrNameAI] = ai.New(engine, ai.WithPrompt(enginec.AI.Prompt))
	}
	useEngines := make([]translator.ITranslator, 0, len(allEngines))
	engineNames := []string{
		c.TranslateConfig.Engine,
	}
	engineNames = append(engineNames, c.TranslateConfig.Fallback...)
	engineNames = lo.Uniq(engineNames)
	for _, name := range engineNames {
		e, ok := allEngines[strings.ToLower(name)]
		if !ok {
			logutil.GetLogger(context.Background()).Error("spec engine not found, skip", zap.String("name", name))
			continue
		}
		useEngines = append(useEngines, e)
	}
	if len(useEngines) == 0 {
		return nil, fmt.Errorf("no engine used, need to check engine config")
	}
	return translator.NewGroup(useEngines...), nil
}

func buildNumberCleaner(cli client.IHTTPClient, c *config.Config) (numbercleaner.Cleaner, func(context.Context), error) {
	cc := c.NumberCleanerConfig
	if cc.Disabled {
		return numbercleaner.NewPassthroughCleaner(), nil, nil
	}
	sourceType := strings.ToLower(strings.TrimSpace(cc.SourceType))
	if sourceType == "" {
		sourceType = numbercleaner.SourceTypeLocal
	}
	var basePath string
	var syncLoop func(context.Context)
	switch sourceType {
	case numbercleaner.SourceTypeLocal:
		localPath := strings.TrimSpace(cc.LocalBundlePath)
		if localPath == "" {
			localPath = cc.RulePath
		}
		resolved, err := resolveRuleSourcePath(c.DataDir, localPath)
		if err != nil {
			return nil, nil, err
		}
		manager := numbercleaner.NewBundleManager(c.DataDir, cli, numbercleaner.SourceTypeLocal, "", resolved)
		basePath, err = manager.CurrentRulePath()
		if err != nil {
			return nil, nil, err
		}
	case numbercleaner.SourceTypeRemote:
		manager := numbercleaner.NewBundleManager(c.DataDir, cli, numbercleaner.SourceTypeRemote, cc.RemoteBundleURL, "")
		var err error
		basePath, _, err = manager.SyncRemote(context.Background())
		if err != nil {
			syncErr := err
			basePath, err = manager.CurrentRulePath()
			if err != nil {
				return nil, nil, fmt.Errorf("sync remote number cleaner bundle failed: %w", syncErr)
			}
			logutil.GetLogger(context.Background()).Warn("sync remote number cleaner bundle failed, use active local bundle", zap.Error(syncErr))
		}
		syncLoop = buildNumberCleanerRemoteSyncLoop(cli, c, manager)
	default:
		return nil, nil, fmt.Errorf("unsupported number cleaner source type: %s", sourceType)
	}
	logutil.GetLogger(context.Background()).Info("load number cleaner base rule", zap.String("path", basePath))
	base, err := numbercleaner.LoadRuleSetFromPath(basePath)
	if err != nil {
		return nil, nil, err
	}
	finalRules := base
	if len(strings.TrimSpace(cc.OverrideRulePath)) != 0 {
		overridePath, err := resolveRuleSourcePath(c.DataDir, cc.OverrideRulePath)
		if err == nil {
			logutil.GetLogger(context.Background()).Info("load number cleaner override rule", zap.String("path", overridePath))
			override, err := numbercleaner.LoadRuleSetFromPath(overridePath)
			if err != nil {
				return nil, nil, err
			}
			finalRules, err = numbercleaner.MergeRuleSets(base, override)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	inner, err := numbercleaner.NewCleaner(finalRules)
	if err != nil {
		return nil, nil, err
	}
	runtimeCleaner := numbercleaner.NewRuntimeCleaner(inner)
	if syncLoop != nil {
		syncLoop = wrapNumberCleanerRemoteSyncLoop(cli, syncLoop, runtimeCleaner, c)
	}
	return runtimeCleaner, syncLoop, nil
}

func buildNumberCleanerRemoteSyncLoop(cli client.IHTTPClient, c *config.Config, manager *numbercleaner.BundleManager) func(context.Context) {
	cc := c.NumberCleanerConfig
	if !cc.AutoSync {
		return nil
	}
	interval := time.Duration(cc.SyncIntervalHour) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return func(ctx context.Context) { runNumberCleanerRemoteSyncLoop(ctx, interval, manager, nil, c) }
}

func wrapNumberCleanerRemoteSyncLoop(cli client.IHTTPClient, baseLoop func(context.Context), runtime *numbercleaner.RuntimeCleaner, c *config.Config) func(context.Context) {
	if baseLoop == nil {
		return nil
	}
	manager := numbercleaner.NewBundleManager(c.DataDir, cli, numbercleaner.SourceTypeRemote, c.NumberCleanerConfig.RemoteBundleURL, "")
	cc := c.NumberCleanerConfig
	interval := time.Duration(cc.SyncIntervalHour) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return func(ctx context.Context) { runNumberCleanerRemoteSyncLoop(ctx, interval, manager, runtime, c) }
}

func runNumberCleanerRemoteSyncLoop(ctx context.Context, interval time.Duration, manager *numbercleaner.BundleManager, runtime *numbercleaner.RuntimeCleaner, c *config.Config) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rulePath, updated, err := manager.SyncRemote(ctx)
			if err != nil {
				logutil.GetLogger(ctx).Error("sync number cleaner remote bundle failed", zap.Error(err))
				continue
			}
			if !updated {
				continue
			}
			if runtime == nil {
				continue
			}
			nextCleaner, err := loadNumberCleanerFromPaths(c.DataDir, rulePath, c.NumberCleanerConfig.OverrideRulePath)
			if err != nil {
				logutil.GetLogger(ctx).Error("reload number cleaner after remote sync failed", zap.Error(err))
				continue
			}
			runtime.Swap(nextCleaner)
			logutil.GetLogger(ctx).Info("reload number cleaner after remote sync", zap.String("path", rulePath))
		}
	}
}

func loadNumberCleanerFromPaths(datadir string, basePath string, overridePath string) (numbercleaner.Cleaner, error) {
	base, err := numbercleaner.LoadRuleSetFromPath(basePath)
	if err != nil {
		return nil, err
	}
	finalRules := base
	if strings.TrimSpace(overridePath) != "" {
		resolvedOverride, err := resolveRuleSourcePath(datadir, overridePath)
		if err == nil {
			override, err := numbercleaner.LoadRuleSetFromPath(resolvedOverride)
			if err != nil {
				return nil, err
			}
			finalRules, err = numbercleaner.MergeRuleSets(base, override)
			if err != nil {
				return nil, err
			}
		}
	}
	return numbercleaner.NewCleaner(finalRules)
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

func rewriteEnvFlagToConfig(c *config.SwitchConfig) {
	//配置项均移到配置文件中, 不再使用环境变量
	if os.Getenv("ENABLE_SEARCH_META_CACHE") == "false" {
		c.EnableSearchMetaCache = false
	}
	if os.Getenv("ENABLE_PIGO_FACE_RECOGNIZER") == "false" {
		c.EnablePigoFaceRecognizer = false
	}
}
