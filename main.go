package main

import (
	"av-capture/capture"
	"av-capture/config"
	"av-capture/image"
	"av-capture/processor"
	"av-capture/searcher"
	"av-capture/store"
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/xxxsen/common/logger"
	"go.uber.org/zap"

	_ "av-capture/searcher/plugin"
)

var conf = flag.String("config", "./config.json", "config file")

func main() {
	flag.Parse()
	c, err := config.Parse(*conf)
	if err != nil {
		log.Fatalf("parse config failed, err:%v", err)
	}
	logkit := logger.Init(c.LogConfig.File, c.LogConfig.Level, int(c.LogConfig.FileCount), int(c.LogConfig.FileSize), int(c.LogConfig.KeepDays), c.LogConfig.Console)
	store.Init(filepath.Join(c.DataDir, "cache"))
	if err := image.Init(c.ModelDir); err != nil {
		logkit.Fatal("init image recognizer failed", zap.Error(err))
	}
	ss, err := buildSearcher(c.Searchers, c.SearcherConfig)
	if err != nil {
		logkit.Fatal("build searcher failed", zap.Error(err))
	}
	ps, err := buildProcessor(c.Processors, c.ProcessorConfig)
	if err != nil {
		logkit.Fatal("build processor failed", zap.Error(err))
	}
	cap, err := buildCapture(c, ss, ps)
	if err != nil {
		logkit.Fatal("build capture runner failed", zap.Error(err))
	}
	if err := cap.Run(context.Background()); err != nil {
		logkit.Fatal("run capture logic failed", zap.Error(err))
	}
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

func buildSearcher(ss []string, m map[string]interface{}) ([]searcher.ISearcher, error) {
	rs := make([]searcher.ISearcher, 0, len(ss))
	for _, s := range ss {
		sr, ok := searcher.Get(s)
		if !ok {
			return nil, fmt.Errorf("searcher not found, name:%s", s)
		}
		rs = append(rs, sr)
	}
	return rs, nil
}

func buildProcessor(ps []string, m map[string]interface{}) ([]processor.IProcessor, error) {
	def := make(map[string]interface{})
	rs := make([]processor.IProcessor, 0, len(ps))
	for _, item := range ps {
		data, ok := m[item]
		if !ok {
			data = def
		}
		pr, err := processor.MakeProcessor(item, data)
		if err != nil {
			return nil, fmt.Errorf("make processor failed, name:%s, err:%w", item, err)
		}
		rs = append(rs, pr)
	}
	return rs, nil
}
