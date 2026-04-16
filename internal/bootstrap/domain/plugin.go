package domain

import (
	"context"
	"fmt"
	"strings"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/client"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
)

// BundleResolveFunc is called each time the plugin bundle is resolved/reloaded.
type BundleResolveFunc func(ctx context.Context, resolved *pluginbundle.ResolvedBundle) error

func PrepareSearcherPlugins(
	ctx context.Context,
	cli client.IHTTPClient,
	dataDir string,
	sources []PluginSource,
	onResolve BundleResolveFunc,
) (*pluginbundle.Manager, error) {
	filtered := ConfiguredPluginSources(sources)
	if len(filtered) == 0 {
		LogSearcherPluginConfigMissing(ctx)
		return nil, ErrNoPluginSources
	}
	bundleSources := make([]pluginbundle.Source, 0, len(filtered))
	for _, source := range filtered {
		item := pluginbundle.Source{
			SourceType: source.SourceType,
			Location:   source.Location,
		}
		st := strings.ToLower(strings.TrimSpace(item.SourceType))
		if st == "" || strings.EqualFold(item.SourceType, basebundle.SourceTypeLocal) {
			resolved, err := ResolveBundleSourcePath(dataDir, item.Location)
			if err != nil {
				return nil, err
			}
			item.Location = resolved
			item.SourceType = basebundle.SourceTypeLocal
		}
		bundleSources = append(bundleSources, item)
	}
	manager, err := pluginbundle.NewManager("searcher_plugin", dataDir, cli, bundleSources,
		func(ctx context.Context, resolved *pluginbundle.ResolvedBundle, _ []string) error {
			pluginyaml.SyncBundle(resolved.Plugins)
			if onResolve != nil {
				return onResolve(ctx, resolved)
			}
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("create plugin bundle manager: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("start plugin bundle manager: %w", err)
	}
	return manager, nil
}
