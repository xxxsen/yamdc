package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"github.com/xxxsen/yamdc/internal/aiengine"
	_ "github.com/xxxsen/yamdc/internal/aiengine/gemini"
	_ "github.com/xxxsen/yamdc/internal/aiengine/ollama"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/capture/ruleapi"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/dependency"
	"github.com/xxxsen/yamdc/internal/dynscript"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/face/pigo"
	"github.com/xxxsen/yamdc/internal/ffmpeg"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/translator/ai"
	"github.com/xxxsen/yamdc/internal/translator/google"

	"github.com/spf13/pflag"
	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	_ "github.com/xxxsen/yamdc/internal/searcher/plugin/register"

	"github.com/samber/lo"
)

func main() {
	configPath, err := parseConfigPath()
	if err != nil {
		log.Fatalf("parse flags failed, err:%v", err)
	}
	c, err := config.Parse(configPath)
	if err != nil {
		log.Fatalf("parse config failed, err:%v", err)
	}
	rewriteEnvFlagToConfig(&c.SwitchConfig)
	logkit := logger.Init(c.LogConfig.File, c.LogConfig.Level, int(c.LogConfig.FileCount), int(c.LogConfig.FileSize), int(c.LogConfig.KeepDays), c.LogConfig.Console)
	if err := precheckDir(c); err != nil {
		logkit.Fatal("precheck dir failed", zap.Error(err))
	}
	if err := setupHTTPClient(c); err != nil {
		logkit.Fatal("setup http client failed", zap.Error(err))
	}
	logkit.Info("check dependencies...")
	if err := initDependencies(c.DataDir, c.Dependencies); err != nil {
		logkit.Fatal("ensure dependencies failed", zap.Error(err))
	}
	logkit.Info("check dependencies finish...")
	logkit.Info("use switch config", zap.Any("switch", c.SwitchConfig))
	if err := setupAIEngine(c); err != nil {
		logkit.Fatal("setup ai engine failed", zap.Error(err))
	}

	store.SetStorage(store.MustNewSqliteStorage(filepath.Join(c.DataDir, "cache", "cache.db")))
	if err := setupTranslator(c); err != nil {
		logkit.Error("setup translator failed", zap.Error(err)) //非关键路径
	}
	logkit.Info("use translator engine", zap.String("engine", c.TranslateConfig.Engine))
	if err := setupFace(c, filepath.Join(c.DataDir, "models")); err != nil {
		logkit.Error("init face recognizer failed", zap.Error(err))
	}
	logkit.Info("support plugins", zap.Strings("plugins", factory.Plugins()))
	logkit.Info("support handlers", zap.Strings("handlers", handler.Handlers()))
	logkit.Info("use plugins", zap.Strings("plugins", c.Plugins))
	for _, ct := range c.CategoryPlugins {
		logkit.Info("-- cat plugins", zap.String("cat", ct.Name), zap.Strings("plugins", ct.Plugins))
	}
	logkit.Info("use handlers", zap.Strings("handlers", c.Handlers))
	logkit.Info("use naming rule", zap.String("rule", c.Naming))
	logkit.Info("scrape from dir", zap.String("dir", c.ScanDir))
	logkit.Info("save to dir", zap.String("dir", c.SaveDir))
	logkit.Info("use data dir", zap.String("dir", c.DataDir))
	logkit.Info("check feature list")
	logkit.Info("-- ffmpeg", zap.Bool("enable", ffmpeg.IsFFMpegEnabled()))
	logkit.Info("-- ffprobe", zap.Bool("enable", ffmpeg.IsFFProbeEnabled()))
	logkit.Info("-- translator", zap.Bool("enable", translator.IsTranslatorEnabled()))
	logkit.Info("-- face recognize", zap.Bool("enable", face.IsFaceRecognizeEnabled()))
	logkit.Info("-- ai engine", zap.Bool("enable", aiengine.IsAIEngineEnabled()))

	ss, err := buildSearcher(c, c.Plugins, c.PluginConfig)
	if err != nil {
		logkit.Fatal("build searcher failed", zap.Error(err))
	}
	catSs, err := buildCatSearcher(c, c.CategoryPlugins, c.PluginConfig)
	if err != nil {
		logkit.Fatal("build cat searcher failed", zap.Error(err))
	}
	tryTestSearcher(c, ss, catSs)
	ps, err := buildProcessor(c.Handlers, c.HandlerConfig)
	if err != nil {
		logkit.Fatal("build processor failed", zap.Error(err))
	}
	cap, err := buildCapture(c, ss, catSs, ps)
	if err != nil {
		logkit.Fatal("build capture runner failed", zap.Error(err))
	}
	logkit.Info("capture kit init succ, start scraping")
	if err := cap.Run(context.Background()); err != nil {
		logkit.Error("run capture kit failed", zap.Error(err))
		return
	}
	logkit.Info("run capture kit finish, all file scrape succ")
}

func parseConfigPath() (string, error) {
	fs := pflag.NewFlagSet("yamdc", pflag.ContinueOnError)
	configPath := fs.String("config", "./config.json", "config file")
	if len(os.Args) > 1 && os.Args[1] == "run" {
		if err := fs.Parse(os.Args[2:]); err != nil {
			return "", err
		}
		return *configPath, nil
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return "", err
	}
	return *configPath, nil
}

func buildCapture(c *config.Config, ss []searcher.ISearcher, catSs map[string][]searcher.ISearcher, ps []processor.IProcessor) (*capture.Capture, error) {
	numberUncensorRule, err := buildNumberUncensorRule(c)
	if err != nil {
		return nil, err
	}
	numberCategoryRule, err := buildNumberCategoryRule(c)
	if err != nil {
		return nil, err
	}
	numberRewriteRule, err := buildNumberRewriteRule(c)
	if err != nil {
		return nil, err
	}

	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(c.Naming),
		capture.WithScanDir(c.ScanDir),
		capture.WithSaveDir(c.SaveDir),
		capture.WithSeacher(searcher.NewCategorySearcher(ss, catSs)),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithExtraMediaExtList(c.ExtraMediaExts),
		capture.WithUncensorTester(numberUncensorRule),
		capture.WithNumberCategorier(numberCategoryRule),
		capture.WithNumberRewriter(numberRewriteRule),
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

func tryTestSearcher(c *config.Config, ss []searcher.ISearcher, catSs map[string][]searcher.ISearcher) {
	if !c.SwitchConfig.EnableSearcherCheck {
		return
	}
	testMap := make(map[string]searcher.ISearcher)
	for _, s := range ss {
		testMap[s.Name()] = s
	}
	for _, catS := range catSs {
		for _, item := range catS {
			if _, ok := testMap[item.Name()]; ok {
				continue
			}
			testMap[item.Name()] = item
		}
	}
	wg, ctx := errgroup.WithContext(context.Background())
	m := make(map[string]error, len(ss))
	var lck sync.Mutex
	logutil.GetLogger(ctx).Info("try test searhers...")
	for _, s := range testMap {
		s := s
		wg.Go(func() error {
			err := s.Check(ctx)
			lck.Lock()
			defer lck.Unlock()
			m[s.Name()] = err
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		logutil.GetLogger(ctx).Error("test searcher internal err", zap.Error(err))
		return
	}
	for name, err := range m {
		if err != nil {
			logutil.GetLogger(ctx).Error("-- test searher failed", zap.String("searcher", name), zap.Error(err))
			continue
		}
		logutil.GetLogger(ctx).Info("-- test searcher succ", zap.String("searcher", name))
	}

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

func readScriptStream(datadir string, relpath string) ([]byte, error) {
	paths := []string{relpath, path.Join(datadir, relpath)} //尝试从所有的路径种查找脚本, relpath可以被用户传入, 所以优先查找
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		return data, nil
	}
	return nil, fmt.Errorf("no scripts found in paths, relpath:%s", relpath)
}

func buildNumberUncensorRule(c *config.Config) (ruleapi.ITester, error) {
	rule, err := readScriptStream(c.DataDir, c.RuleConfig.NumberUncensorTesterConfig)
	if err != nil {
		return nil, err
	}
	ck, err := dynscript.NewNumberUncensorChecker(string(rule))
	if err != nil {
		return nil, err
	}
	return ruleapi.WrapFuncAsTester(func(res string) (bool, error) {
		return ck.IsMatch(context.Background(), res)
	}), nil
}

func buildNumberCategoryRule(c *config.Config) (ruleapi.IMatcher, error) {
	rule, err := readScriptStream(c.DataDir, c.RuleConfig.NumberCategorierConfig)
	if err != nil {
		return nil, err
	}
	cater, err := dynscript.NewNumberCategorier(string(rule))
	if err != nil {
		return nil, err
	}
	return ruleapi.WrapFuncAsMatcher(func(res string) (string, bool, error) {
		return cater.Category(context.Background(), res)
	}), nil
}

func buildNumberRewriteRule(c *config.Config) (ruleapi.IRewriter, error) {
	rule, err := readScriptStream(c.DataDir, c.RuleConfig.NumberRewriterConfig)
	if err != nil {
		return nil, err
	}
	rewriter, err := dynscript.NewNumberRewriter(string(rule))
	if err != nil {
		return nil, err
	}
	return ruleapi.WrapFuncAsRewriter(func(res string) (string, error) {
		return rewriter.Rewrite(context.Background(), res)
	}), nil
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
