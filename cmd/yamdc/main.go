package main

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/aiengine"
	_ "github.com/xxxsen/yamdc/internal/aiengine/gemini"
	_ "github.com/xxxsen/yamdc/internal/aiengine/ollama"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/dependency"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/face/pigo"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/translator/ai"
	"github.com/xxxsen/yamdc/internal/translator/google"
	"github.com/xxxsen/yamdc/internal/web"
	"log"
	"net/http"
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
	if err := normalizeDirPaths(c); err != nil {
		return fmt.Errorf("normalize dir paths failed, err:%w", err)
	}
	logkit := logger.Init(c.LogConfig.File, c.LogConfig.Level, int(c.LogConfig.FileCount), int(c.LogConfig.FileSize), int(c.LogConfig.KeepDays), true)
	if err := precheckDir(c); err != nil {
		return fmt.Errorf("precheck dir failed, err:%w", err)
	}
	if err := setupHTTPClient(c); err != nil {
		return fmt.Errorf("setup http client failed, err:%w", err)
	}
	if err := initDependencies(c.DataDir, c.Dependencies); err != nil {
		return fmt.Errorf("ensure dependencies failed, err:%w", err)
	}
	if err := setupAIEngine(c); err != nil {
		return fmt.Errorf("setup ai engine failed, err:%w", err)
	}
	store.SetStorage(store.MustNewSqliteStorage(filepath.Join(c.DataDir, "cache", "cache.db")))
	if err := setupTranslator(c); err != nil {
		logkit.Error("setup translator failed", zap.Error(err))
	}
	if err := setupFace(c, filepath.Join(c.DataDir, "models")); err != nil {
		logkit.Error("init face recognizer failed", zap.Error(err))
	}
	ss, err := buildSearcher(c, c.Plugins, c.PluginConfig)
	if err != nil {
		return fmt.Errorf("build searcher failed, err:%w", err)
	}
	catSs, err := buildCatSearcher(c, c.CategoryPlugins, c.PluginConfig)
	if err != nil {
		return fmt.Errorf("build cat searcher failed, err:%w", err)
	}
	ps, err := buildProcessor(c.Handlers, c.HandlerConfig)
	if err != nil {
		return fmt.Errorf("build processor failed, err:%w", err)
	}
	numCleaner, err := buildNumberCleaner(c)
	if err != nil {
		return fmt.Errorf("build number cleaner failed, err:%w", err)
	}
	cap, err := buildCapture(c, ss, catSs, ps, numCleaner)
	if err != nil {
		return fmt.Errorf("build capture runner failed, err:%w", err)
	}
	appDB, err := repository.NewSQLite(filepath.Join(c.DataDir, "app", "app.db"))
	if err != nil {
		return fmt.Errorf("init app db failed, err:%w", err)
	}
	defer appDB.Close()

	jobRepo := repository.NewJobRepository(appDB.DB())
	logRepo := repository.NewLogRepository(appDB.DB())
	scrapeRepo := repository.NewScrapeDataRepository(appDB.DB())
	scanSvc := scanner.New(c.ScanDir, c.ExtraMediaExts, jobRepo, numCleaner)
	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, cap)
	mediaSvc := medialib.NewService(appDB.DB(), c.LibraryDir, c.SaveDir)
	jobSvc.SetImportGuard(func(ctx context.Context) error {
		if mediaSvc.IsMoveRunning() {
			return fmt.Errorf("move to media library is running")
		}
		return nil
	})
	if err := jobSvc.Recover(context.Background()); err != nil {
		logkit.Error("recover processing jobs failed", zap.Error(err))
	}
	mediaSvc.Start(context.Background())
	api := web.NewAPI(jobRepo, scanSvc, jobSvc, c.SaveDir, mediaSvc)
	addr := os.Getenv("YAMDC_SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	logkit.Info("yamdc server start", zap.String("addr", addr), zap.String("scan_dir", c.ScanDir), zap.String("data_dir", c.DataDir))
	if err := http.ListenAndServe(addr, api.Handler()); err != nil {
		return fmt.Errorf("listen and serve failed, err:%w", err)
	}
	return nil
}

func buildCapture(c *config.Config, ss []searcher.ISearcher, catSs map[string][]searcher.ISearcher, ps []processor.IProcessor, numCleaner numbercleaner.Cleaner) (*capture.Capture, error) {
	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(c.Naming),
		capture.WithScanDir(c.ScanDir),
		capture.WithSaveDir(c.SaveDir),
		capture.WithSeacher(searcher.NewCategorySearcher(ss, catSs)),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithExtraMediaExtList(c.ExtraMediaExts),
		capture.WithNumberCleaner(numCleaner),
		capture.WithTransalteTitleDiscard(c.TranslateConfig.DiscardTranslatedTitle),
		capture.WithTranslatedPlotDiscard(c.TranslateConfig.DiscardTranslatedPlot),
		capture.WithLinkMode(c.SwitchConfig.EnableLinkMode),
	)
	return capture.New(opts...)
}

func buildCatSearcher(c *config.Config, cplgs []config.CategoryPlugin, m map[string]config.PluginConfig) (map[string][]searcher.ISearcher, error) {
	rs := make(map[string][]searcher.ISearcher, len(cplgs))
	for _, plg := range cplgs {
		ss, err := buildSearcher(c, plg.Plugins, m)
		if err != nil {
			return nil, err
		}
		rs[strings.ToUpper(plg.Name)] = ss
	}
	return rs, nil
}

func buildSearcher(c *config.Config, plgs []string, m map[string]config.PluginConfig) ([]searcher.ISearcher, error) {
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
			searcher.WithHTTPClient(client.DefaultClient()),
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

func buildProcessor(hs []string, m map[string]config.HandlerConfig) ([]processor.IProcessor, error) {
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
		h, err := handler.CreateHandler(name, handlec.Args)
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

func initDependencies(datadir string, cdeps []config.Dependency) error {
	deps := make([]*dependency.Dependency, 0, len(cdeps))
	for _, item := range cdeps {
		deps = append(deps, &dependency.Dependency{
			URL:     item.Link,
			Target:  filepath.Join(datadir, item.RelPath),
			Refresh: item.Refresh,
		})
	}
	return dependency.Resolve(client.DefaultClient(), deps)
}

func setupFace(c *config.Config, models string) error {
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
		return fmt.Errorf("no face rec impl inited")
	}
	face.SetFaceRec(face.NewGroup(impls))
	return nil
}

func setupHTTPClient(c *config.Config) error {
	opts := make([]client.Option, 0, 4)
	if c.NetworkConfig.Timeout > 0 {
		opts = append(opts, client.WithTimeout(time.Duration(c.NetworkConfig.Timeout)*time.Second))
	}
	if pxy := c.NetworkConfig.Proxy; len(pxy) > 0 {
		opts = append(opts, client.WithProxy(pxy))
	}
	clientImpl, err := client.NewClient(opts...)
	if err != nil {
		return err
	}
	if c.FlareSolverrConfig.Enable {
		bpc, err := flarerr.New(clientImpl, c.FlareSolverrConfig.Host)
		if err != nil {
			return fmt.Errorf("create flaresolverr client failed, err:%w", err)
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
	client.SetDefault(clientImpl)
	return nil
}

func setupAIEngine(c *config.Config) error {
	if len(c.AIEngine.Name) == 0 {
		logutil.GetLogger(context.Background()).Info("ai engine is disabled, skip init")
		return nil
	}
	engine, err := aiengine.Create(c.AIEngine.Name, c.AIEngine.Args)
	if err != nil {
		return fmt.Errorf("create ai engine failed, name:%s, err:%w", c.AIEngine.Name, err)
	}
	aiengine.SetAIEngine(engine)
	return nil
}

func setupTranslator(c *config.Config) error {
	if !c.TranslateConfig.Enable {
		return nil
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
		allEngines[translator.TrNameAI] = ai.New(ai.WithPrompt(enginec.AI.Prompt))
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
		return fmt.Errorf("no engine used, need to check engine config")
	}
	tr := translator.NewGroup(useEngines...)
	translator.SetTranslator(tr)
	return nil
}

func readScriptStreamWithPath(datadir string, relpath string) ([]byte, string, error) {
	paths := []string{relpath, path.Join(datadir, relpath)} //尝试从所有的路径种查找脚本, relpath可以被用户传入, 所以优先查找
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		return data, p, nil
	}
	return nil, "", fmt.Errorf("no scripts found in paths, relpath:%s", relpath)
}

func buildNumberCleaner(c *config.Config) (numbercleaner.Cleaner, error) {
	cc := c.NumberCleanerConfig
	if cc.Disabled || len(strings.TrimSpace(cc.RulePath)) == 0 {
		return numbercleaner.NewPassthroughCleaner(), nil
	}
	baseRule, basePath, err := readScriptStreamWithPath(c.DataDir, cc.RulePath)
	if err != nil {
		return nil, err
	}
	logutil.GetLogger(context.Background()).Info("load number cleaner base rule", zap.String("path", basePath))
	base, err := numbercleaner.NewLoader().Load(baseRule)
	if err != nil {
		return nil, err
	}
	finalRules := base
	if len(strings.TrimSpace(cc.OverrideRulePath)) != 0 {
		overrideRaw, overridePath, err := readScriptStreamWithPath(c.DataDir, cc.OverrideRulePath)
		if err == nil {
			logutil.GetLogger(context.Background()).Info("load number cleaner override rule", zap.String("path", overridePath))
			override, err := numbercleaner.NewLoader().Load(overrideRaw)
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

func rewriteEnvFlagToConfig(c *config.SwitchConfig) {
	//配置项均移到配置文件中, 不再使用环境变量
	if os.Getenv("ENABLE_SEARCH_META_CACHE") == "false" {
		c.EnableSearchMetaCache = false
	}
	if os.Getenv("ENABLE_PIGO_FACE_RECOGNIZER") == "false" {
		c.EnablePigoFaceRecognizer = false
	}
}
