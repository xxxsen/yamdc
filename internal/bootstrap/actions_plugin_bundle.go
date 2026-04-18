package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/searcher"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"go.uber.org/zap"
)

// YAML 搜索器插件 bundle 的启动与热重载逻辑。
// 被 `buildSearchersAction` 在配置存在插件源时引用。
//
// 拆出来的原因:
//   - reloadSearcherPluginBundle 闭包里持有 StartContext / runtimeSearcher,
//     与普通 InitAction 的签名不同, 放在 actions_domain.go 会增加噪音。
//   - prepareSearcherPluginsForServer 的工作是装配 Manager, 属于一次性初始化,
//     与热重载回调 (reload) 是一组职责, 适合单独一个文件。

func reloadSearcherPluginBundle(
	cbCtx context.Context,
	sc *StartContext,
	runtimeSearcher *searcher.RuntimeCategorySearcher,
	resolved *pluginbundle.ResolvedBundle,
) error {
	c := sc.Infra.Config
	nextDefaultPlugins, nextCategoryPlugins := domain.ResolvedPluginConfig(resolved)
	registerCtx := pluginyaml.BuildRegisterContext(resolved.Plugins)
	creatorSnapshot := registerCtx.Snapshot()
	plugOpts := toPluginOptions(c.PluginConfig)
	ss, err := domain.BuildSearcherWithCreators(
		cbCtx, sc.Infra.HTTPClient, sc.Infra.CacheStore,
		c.SwitchConfig.EnableSearchMetaCache, nextDefaultPlugins, plugOpts, creatorSnapshot,
	)
	if err != nil {
		return fmt.Errorf("rebuild searchers: %w", err)
	}
	catSs, err := domain.BuildCatSearcherWithCreators(
		cbCtx, sc.Infra.HTTPClient, sc.Infra.CacheStore,
		c.SwitchConfig.EnableSearchMetaCache, nextCategoryPlugins, plugOpts, creatorSnapshot,
	)
	if err != nil {
		return fmt.Errorf("rebuild category searchers: %w", err)
	}
	factory.Swap(registerCtx)
	domain.LogPluginBundleWarnings(cbCtx, resolved.Warnings)
	c.Plugins = nextDefaultPlugins
	c.CategoryPlugins = fromDomainCategoryPlugins(nextCategoryPlugins)
	logutil.GetLogger(cbCtx).Info("load searcher plugin bundles",
		zap.Strings("default_plugins", c.Plugins),
		zap.Int("category_count", len(c.CategoryPlugins)),
	)
	sc.Domain.Searchers = ss
	sc.Domain.CategorySearchers = catSs
	runtimeSearcher.Swap(ss, catSs)
	if sc.Domain.SearcherDebugger != nil {
		sc.Domain.SearcherDebugger.SwapState(
			nextDefaultPlugins,
			domain.CategoryPluginMap(nextCategoryPlugins),
			creatorSnapshot,
		)
	}
	logutil.GetLogger(cbCtx).Info("reload searcher plugin runtime",
		zap.Int("default_plugins", len(c.Plugins)),
		zap.Int("category_chains", len(c.CategoryPlugins)),
	)
	return nil
}

func prepareSearcherPluginsForServer(
	ctx context.Context,
	sc *StartContext,
	runtimeSearcher *searcher.RuntimeCategorySearcher,
) (*pluginbundle.Manager, error) {
	c := sc.Infra.Config
	sources := toPluginSources(c.SearcherPluginConfig.Sources)
	if len(domain.ConfiguredPluginSources(sources)) == 0 {
		return nil, domain.ErrNoPluginSources
	}
	bundleSources := make([]pluginbundle.Source, 0, len(sources))
	for _, source := range sources {
		item := pluginbundle.Source{
			SourceType: source.SourceType,
			Location:   source.Location,
		}
		st := strings.TrimSpace(item.SourceType)
		if st == "" || strings.EqualFold(st, "local") {
			resolved, err := domain.ResolveBundleSourcePath(c.DataDir, item.Location)
			if err != nil {
				return nil, fmt.Errorf("resolve bundle source path: %w", err)
			}
			item.Location = resolved
			item.SourceType = "local"
		}
		bundleSources = append(bundleSources, item)
	}
	manager, err := pluginbundle.NewManager(
		"searcher_plugin", c.DataDir, sc.Infra.HTTPClient, bundleSources,
		func(cbCtx context.Context, resolved *pluginbundle.ResolvedBundle, _ []string) error {
			pluginyaml.SyncBundle(resolved.Plugins)
			return reloadSearcherPluginBundle(cbCtx, sc, runtimeSearcher, resolved)
		})
	if err != nil {
		return nil, fmt.Errorf("create plugin bundle manager failed: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("start plugin bundle manager failed: %w", err)
	}
	return manager, nil
}
