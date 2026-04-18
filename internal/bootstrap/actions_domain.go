package bootstrap

import (
	"context"
	"fmt"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/searcher"
)

// 业务域 (domain) action 集合:
//   - Searcher / CategorySearcher (含 YAML 插件 bundle 路径)
//   - Processor handler 链
//   - 番号清洗 (MovieIDCleaner)
//   - 搜索器 / handler 调试器
//   - Capture 主流程编排对象
//
// 这一层依赖 infra (HTTPClient/CacheStore) 与 runtime (Translator/AIEngine/FaceRec),
// 但不应碰 App 层资源 (DB/Service)。

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
	cleaner, manager, err := domain.BuildMovieIDCleaner(
		ctx, sc.Infra.HTTPClient,
		c.DataDir, c.MovieIDRulesetConfig.SourceType, c.MovieIDRulesetConfig.Location,
	)
	if err != nil {
		return fmt.Errorf("build movieid cleaner: %w", err)
	}
	sc.Domain.MovieIDCleaner = cleaner
	sc.Domain.MovieIDCleanerMgr = manager
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
