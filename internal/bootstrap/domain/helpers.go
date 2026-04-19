package domain

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
)

func ResolveRuleSourcePath(datadir, raw string) (string, error) {
	paths := []string{raw, path.Join(datadir, raw)}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() || strings.HasSuffix(strings.ToLower(p), ".yaml") || strings.HasSuffix(strings.ToLower(p), ".yml") {
			abs, absErr := filepath.Abs(p)
			if absErr != nil {
				return "", fmt.Errorf("resolve absolute path %q: %w", p, absErr)
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("no rule source found in paths, raw:%s: %w", raw, ErrRuleSourceNotFound)
}

func ResolveBundleSourcePath(datadir, raw string) (string, error) {
	paths := []string{raw, path.Join(datadir, raw)}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			abs, absErr := filepath.Abs(p)
			if absErr != nil {
				return "", fmt.Errorf("resolve absolute path %q: %w", p, absErr)
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("no bundle source found in paths, raw:%s: %w", raw, ErrBundleSourceNotFound)
}

func ConfiguredPluginSources(raw []PluginSource) []PluginSource {
	out := make([]PluginSource, 0, len(raw))
	for _, item := range raw {
		if strings.TrimSpace(item.Location) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func HasMovieIDRulesetSource(location string) bool {
	return strings.TrimSpace(location) != ""
}

func LogSearcherPluginConfigMissing(ctx context.Context) {
	msg := "searcher plugin repo is not configured, skip bundle loading;" +
		" configure searcher_plugin_config.sources to use your own plugin repo"
	logutil.GetLogger(ctx).Warn(msg)
}

func LogMovieIDRulesetConfigMissing(ctx context.Context) {
	msg := "movieid ruleset repo is not configured, fallback to passthrough cleaner;" +
		" configure movieid_ruleset_config to use your own script repo"
	logutil.GetLogger(ctx).Warn(msg)
}

func LogPluginBundleWarnings(ctx context.Context, warnings []string) {
	for _, warning := range warnings {
		logutil.GetLogger(ctx).Warn("plugin bundle conflict", zap.String("detail", warning))
	}
}

func ResolvedPluginConfig(resolved *pluginbundle.ResolvedBundle) ([]string, []CategoryPlugin) {
	if resolved == nil {
		return nil, nil
	}
	defaultPlugins := append([]string(nil), resolved.DefaultPlugins...)
	categoryPlugins := make([]CategoryPlugin, 0, len(resolved.CategoryChains))
	categoryNames := make([]string, 0, len(resolved.CategoryChains))
	for category := range resolved.CategoryChains {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)
	for _, category := range categoryNames {
		categoryPlugins = append(categoryPlugins, CategoryPlugin{
			Name:    category,
			Plugins: append([]string(nil), resolved.CategoryChains[category]...),
		})
	}
	return defaultPlugins, categoryPlugins
}

func CategoryPluginMap(items []CategoryPlugin) map[string][]string {
	out := make(map[string][]string, len(items))
	for _, item := range items {
		out[item.Name] = append([]string(nil), item.Plugins...)
	}
	return out
}
