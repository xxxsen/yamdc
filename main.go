package main

import (
	"av-capture/capture"
	"av-capture/config"
	"av-capture/image"
	"av-capture/processor"
	"av-capture/processor/handler"
	"av-capture/searcher"
	"av-capture/store"
	"av-capture/translater"
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"av-capture/searcher/plugin"
)

var conf = flag.String("config", "./config.json", "config file")

func main() {
	flag.Parse()
	c, err := config.Parse(*conf)
	if err != nil {
		log.Fatalf("parse config failed, err:%v", err)
	}
	logkit := logger.Init(c.LogConfig.File, c.LogConfig.Level, int(c.LogConfig.FileCount), int(c.LogConfig.FileSize), int(c.LogConfig.KeepDays), c.LogConfig.Console)

	logkit.Info("support plugins", zap.Strings("plugins", plugin.Plugins()))
	logkit.Info("support handlers", zap.Strings("handlers", handler.Handlers()))
	logkit.Info("current use plugins", zap.Strings("plugins", c.Plugins))
	logkit.Info("current use handlers", zap.Strings("handlers", c.Handlers))
	logkit.Info("use naming rule", zap.String("rule", c.Naming))
	logkit.Info("scrape from dir", zap.String("dir", c.ScanDir))
	logkit.Info("save to dir", zap.String("dir", c.SaveDir))
	logkit.Info("use data dir", zap.String("dir", c.DataDir))
	logkit.Info("current switch options", zap.Any("options", c.SwitchConfig))

	if err := precheckDir(c); err != nil {
		logkit.Fatal("precheck dir failed", zap.Error(err))
	}
	if err := store.Init(filepath.Join(c.DataDir, "cache")); err != nil {
		logkit.Fatal("init store failed", zap.Error(err))
	}
	if err := translater.Init(); err != nil {
		logkit.Fatal("init translater failed", zap.Error(err))
	}
	if err := image.Init(c.ModelDir); err != nil {
		logkit.Fatal("init image recognizer failed", zap.Error(err))
	}
	ss, err := buildSearcher(c.Plugins, c.PluginConfig)
	if err != nil {
		logkit.Fatal("build searcher failed", zap.Error(err))
	}
	ps, err := buildProcessor(c.Handlers, c.HandlerConfig)
	if err != nil {
		logkit.Fatal("build processor failed", zap.Error(err))
	}
	cap, err := buildCapture(c, ss, ps)
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

func buildCapture(c *config.Config, ss []searcher.ISearcher, ps []processor.IProcessor) (*capture.Capture, error) {
	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(c.Naming),
		capture.WithScanDir(c.ScanDir),
		capture.WithSaveDir(c.SaveDir),
		capture.WithSeacher(searcher.NewGroup(ss)),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithEnableLinkMode(c.SwitchConfig.EnableLinkMode),
	)
	return capture.New(opts...)
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
