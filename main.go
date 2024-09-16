package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"yamdc/capture"
	"yamdc/client"
	"yamdc/config"
	"yamdc/dependency"
	"yamdc/face"
	"yamdc/ffmpeg"
	"yamdc/number"
	"yamdc/processor"
	"yamdc/processor/handler"
	"yamdc/searcher"
	"yamdc/store"
	"yamdc/translator"

	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"yamdc/searcher/plugin"
	_ "yamdc/searcher/plugin/airav"
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
	if err := ensureDependencies(c.DataDir, c.Dependencies); err != nil {
		logkit.Fatal("ensure dependencies failed", zap.Error(err))
	}
	store.SetStorage(store.NewDiskStorage(filepath.Join(c.DataDir, "cache")))
	if err := translator.Init(); err != nil {
		logkit.Error("init translater failed", zap.Error(err))
	}
	if err := initFace(filepath.Join(c.DataDir, "models")); err != nil {
		logkit.Error("init face recognizer failed", zap.Error(err))
	}
	logkit.Info("support plugins", zap.Strings("plugins", plugin.Plugins()))
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
	logkit.Info("current switch options", zap.Any("options", c.SwitchConfig))
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

func buildCapture(c *config.Config, ss []searcher.ISearcher, catSs map[number.Category][]searcher.ISearcher, ps []processor.IProcessor) (*capture.Capture, error) {
	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(c.Naming),
		capture.WithScanDir(c.ScanDir),
		capture.WithSaveDir(c.SaveDir),
		capture.WithSeacher(searcher.NewCategorySearcher(ss, catSs)),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithEnableLinkMode(c.SwitchConfig.EnableLinkMode),
		capture.WithExtraMediaExtList(c.ExtraMediaExts),
	)
	return capture.New(opts...)
}

func buildCatSearcher(cplgs []config.CategoryPlugin, m map[string]interface{}) (map[number.Category][]searcher.ISearcher, error) {
	rs := make(map[number.Category][]searcher.ISearcher, len(cplgs))
	for _, plg := range cplgs {
		ss, err := buildSearcher(plg.Plugins, m)
		if err != nil {
			return nil, err
		}
		rs[number.Category(strings.ToUpper(plg.Name))] = ss
	}
	return rs, nil
}

func buildSearcher(plgs []string, m map[string]interface{}) ([]searcher.ISearcher, error) {
	rs := make([]searcher.ISearcher, 0, len(plgs))
	for _, name := range plgs {
		args, ok := m[name]
		if !ok {
			args = struct{}{}
		}
		plg, err := plugin.CreatePlugin(name, args)
		if err != nil {
			return nil, fmt.Errorf("create plugin failed, name:%s, err:%w", name, err)
		}
		sr, err := searcher.NewDefaultSearcher(name, plg)
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

func ensureDependencies(datadir string, cdeps []config.Dependency) error {
	deps := make([]*dependency.Dependency, 0, len(cdeps))
	for _, item := range cdeps {
		deps = append(deps, &dependency.Dependency{
			URL:    item.Link,
			Target: filepath.Join(datadir, item.RelPath),
		})
	}
	return dependency.Resolve(client.NewClient(), deps)
}

func initFace(models string) error {
	inst, err := face.NewGoFace(models)
	if err != nil {
		return err
	}

	face.SetFaceRec(inst)
	return nil
}
