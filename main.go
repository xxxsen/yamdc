package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"yamdc/capture"
	"yamdc/capture/ruleapi"
	"yamdc/client"
	"yamdc/config"
	"yamdc/dependency"
	"yamdc/envflag"
	"yamdc/face"
	"yamdc/face/goface"
	"yamdc/face/pigo"
	"yamdc/ffmpeg"
	"yamdc/processor"
	"yamdc/processor/handler"
	"yamdc/searcher"
	"yamdc/store"
	"yamdc/translator"
	"yamdc/translator/gemini"
	"yamdc/translator/google"

	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"yamdc/searcher/plugin/factory"
	_ "yamdc/searcher/plugin/register"
)

var conf = flag.String("config", "./config.json", "config file")

func main() {
	flag.Parse()
	c, err := config.Parse(*conf)
	if err != nil {
		log.Fatalf("parse config failed, err:%v", err)
	}
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

	if err := envflag.Init(); err != nil {
		logkit.Fatal("init envflag failed", zap.Error(err))
	}
	logkit.Info("read env flags", zap.Any("flag", *envflag.GetFlag()))

	store.SetStorage(store.MustNewSqliteStorage(filepath.Join(c.DataDir, "cache", "cache.db")))
	if err := setupTranslator(c); err != nil {
		logkit.Error("setup translator failed", zap.Error(err)) //非关键路径
	}
	if err := setupFace(filepath.Join(c.DataDir, "models")); err != nil {
		logkit.Error("init face recognizer failed", zap.Error(err))
	}
	logkit.Info("support plugins", zap.Strings("plugins", factory.Plugins()))
	logkit.Info("support handlers", zap.Strings("handlers", handler.Handlers()))
	logkit.Info("current use plugins", zap.Strings("plugins", c.Plugins))
	for _, ct := range c.CategoryPlugins {
		logkit.Info("-- cat plugins", zap.String("cat", ct.Name), zap.Strings("plugins", ct.Plugins))
	}
	logkit.Info("current use handlers", zap.Strings("handlers", c.Handlers))
	logkit.Info("use naming rule", zap.String("rule", c.Naming))
	logkit.Info("scrape from dir", zap.String("dir", c.ScanDir))
	logkit.Info("save to dir", zap.String("dir", c.SaveDir))
	logkit.Info("use data dir", zap.String("dir", c.DataDir))
	logkit.Info("check current feature list")
	logkit.Info("-- ffmpeg", zap.Bool("enable", ffmpeg.IsFFMpegEnabled()))
	logkit.Info("-- ffprobe", zap.Bool("enable", ffmpeg.IsFFProbeEnabled()))
	logkit.Info("-- translator", zap.Bool("enable", translator.IsTranslatorEnabled()))
	logkit.Info("-- face recognize", zap.Bool("enable", face.IsFaceRecognizeEnabled()))

	ss, err := buildSearcher(c.Plugins, c.PluginConfig)
	if err != nil {
		logkit.Fatal("build searcher failed", zap.Error(err))
	}
	catSs, err := buildCatSearcher(c.CategoryPlugins, c.PluginConfig)
	if err != nil {
		logkit.Fatal("build cat searcher failed", zap.Error(err))
	}
	tryTestSearcher(ss, catSs)
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
	)
	return capture.New(opts...)
}

func buildCatSearcher(cplgs []config.CategoryPlugin, m map[string]interface{}) (map[string][]searcher.ISearcher, error) {
	rs := make(map[string][]searcher.ISearcher, len(cplgs))
	for _, plg := range cplgs {
		ss, err := buildSearcher(plg.Plugins, m)
		if err != nil {
			return nil, err
		}
		rs[strings.ToUpper(plg.Name)] = ss
	}
	return rs, nil
}

func tryTestSearcher(ss []searcher.ISearcher, catSs map[string][]searcher.ISearcher) {
	if !envflag.IsEnableSearcherCheck() {
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

func buildSearcher(plgs []string, m map[string]interface{}) ([]searcher.ISearcher, error) {
	rs := make([]searcher.ISearcher, 0, len(plgs))
	for _, name := range plgs {
		args, ok := m[name]
		if !ok {
			args = struct{}{}
		}
		plg, err := factory.CreatePlugin(name, args)
		if err != nil {
			return nil, fmt.Errorf("create plugin failed, name:%s, err:%w", name, err)
		}
		sr, err := searcher.NewDefaultSearcher(name, plg, searcher.WithHTTPClient(client.DefaultClient()))
		if err != nil {
			return nil, fmt.Errorf("create searcher failed, plugin:%s, err:%w", name, err)
		}
		logutil.GetLogger(context.Background()).Info("create search succ", zap.String("plugin", name))
		rs = append(rs, sr)
	}
	return rs, nil
}

func buildProcessor(hs []string, m map[string]interface{}) ([]processor.IProcessor, error) {
	rs := make([]processor.IProcessor, 0, len(hs))
	for _, name := range hs {
		data, ok := m[name]
		if !ok {
			data = struct{}{}
		}
		h, err := handler.CreateHandler(name, data)
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
			URL:    item.Link,
			Target: filepath.Join(datadir, item.RelPath),
		})
	}
	return dependency.Resolve(client.DefaultClient(), deps)
}

func setupFace(models string) error {
	impls := make([]face.IFaceRec, 0, 2)
	var faceRecCreator = make([]func() (face.IFaceRec, error), 0, 2)
	if envflag.IsEnableGoFaceRecognizer() {
		faceRecCreator = append(faceRecCreator, func() (face.IFaceRec, error) {
			return goface.NewGoFace(models)
		})
	}
	if envflag.IsEnablePigoFaceRecognizer() {
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
	client.SetDefault(clientImpl)
	return nil
}

func setupTranslator(c *config.Config) error {
	translator.SetTranslator(
		translator.NewGroup(
			gemini.New(),
			google.New(google.WithProxyUrl(c.NetworkConfig.Proxy)),
		),
	)
	return nil
}

func buildNumberUncensorRule(c *config.Config) (ruleapi.ITester, error) {
	t := ruleapi.NewRegexpTester()
	//合并用户/默认规则
	c.NumberUserRule.NumberUncensorRules = append(c.NumberUserRule.NumberUncensorRules,
		c.NumberDefaultRule.NumberUncensorRules...)
	if err := t.AddRules(c.NumberUserRule.NumberUncensorRules...); err != nil {
		return nil, err
	}
	return t, nil
}

func buildNumberCategoryRule(c *config.Config) (ruleapi.IMatcher, error) {
	t := ruleapi.NewRegexpMatcher()
	c.NumberUserRule.NumberCategoryRule = append(c.NumberUserRule.NumberCategoryRule, c.NumberDefaultRule.NumberCategoryRule...)
	for _, item := range c.NumberUserRule.NumberCategoryRule {
		if err := t.AddRules(ruleapi.RegexpMatchRule{
			Regexp: item.Rules,
			Match:  item.Category,
		}); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func buildNumberRewriteRule(c *config.Config) (ruleapi.IRewriter, error) {
	t := ruleapi.NewRegexpRewriter()
	c.NumberUserRule.NumberRewriteRules = append(c.NumberUserRule.NumberRewriteRules, c.NumberDefaultRule.NumberRewriteRules...)
	for _, item := range c.NumberUserRule.NumberRewriteRules {
		t.AddRules(ruleapi.RegexpRewriteRule{
			Rule:    item.Rule,
			Rewrite: item.Rewrite,
		})
	}
	return t, nil
}
