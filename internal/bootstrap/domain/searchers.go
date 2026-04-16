package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

func BuildSearcher(
	ctx context.Context,
	cli client.IHTTPClient,
	storage store.IStorage,
	enableCache bool,
	plgs []string,
	m map[string]PluginOption,
) ([]searcher.ISearcher, error) {
	return BuildSearcherWithCreators(ctx, cli, storage, enableCache, plgs, m, factory.Snapshot())
}

func BuildSearcherWithCreators(
	ctx context.Context,
	cli client.IHTTPClient,
	storage store.IStorage,
	enableCache bool,
	plgs []string,
	m map[string]PluginOption,
	creators map[string]factory.CreatorFunc,
) ([]searcher.ISearcher, error) {
	rs := make([]searcher.ISearcher, 0, len(plgs))
	for _, name := range plgs {
		plugOpt := m[name]
		if plugOpt.Disable {
			logutil.GetLogger(ctx).Info("plugin is disabled, skip create", zap.String("plugin", name))
			continue
		}
		cr, ok := creators[name]
		if !ok {
			return nil, fmt.Errorf("create plugin failed, name:%s: %w", name, ErrPluginNotFound)
		}
		plg, err := cr(struct{}{})
		if err != nil {
			return nil, fmt.Errorf("create plugin failed, name:%s, err:%w", name, err)
		}
		sr, err := searcher.NewDefaultSearcher(name, plg,
			searcher.WithHTTPClient(cli),
			searcher.WithStorage(storage),
			searcher.WithSearchCache(enableCache),
		)
		if err != nil {
			return nil, fmt.Errorf("create searcher failed, plugin:%s, err:%w", name, err)
		}
		logutil.GetLogger(ctx).Info("create search succ",
			zap.String("plugin", name),
			zap.Strings("domains", plg.OnGetHosts(ctx)),
		)
		rs = append(rs, sr)
	}
	return rs, nil
}

func BuildCatSearcher(
	ctx context.Context,
	cli client.IHTTPClient,
	storage store.IStorage,
	enableCache bool,
	cplgs []CategoryPlugin,
	m map[string]PluginOption,
) (map[string][]searcher.ISearcher, error) {
	return BuildCatSearcherWithCreators(ctx, cli, storage, enableCache, cplgs, m, factory.Snapshot())
}

func BuildCatSearcherWithCreators(
	ctx context.Context,
	cli client.IHTTPClient,
	storage store.IStorage,
	enableCache bool,
	cplgs []CategoryPlugin,
	m map[string]PluginOption,
	creators map[string]factory.CreatorFunc,
) (map[string][]searcher.ISearcher, error) {
	rs := make(map[string][]searcher.ISearcher, len(cplgs))
	for _, plg := range cplgs {
		ss, err := BuildSearcherWithCreators(ctx, cli, storage, enableCache, plg.Plugins, m, creators)
		if err != nil {
			return nil, err
		}
		rs[strings.ToUpper(plg.Name)] = ss
	}
	return rs, nil
}
