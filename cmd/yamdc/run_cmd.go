package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/bootstrap/infra"
	bootrt "github.com/xxxsen/yamdc/internal/bootstrap/runtime"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/searcher"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
)

func newRunCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one full scraping pass from scan dir to save dir",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := config.LoadAppConfig(configPath, config.ModeCapture)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return runCapture(c)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./config.json", "config file")
	return cmd
}

func runCapture(c *config.Config) error {
	ctx := context.Background()
	logkit := infra.InitLogger(
		c.LogConfig.File, c.LogConfig.Level,
		int(c.LogConfig.FileCount), //nolint:gosec // bounded config value
		int(c.LogConfig.FileSize),  //nolint:gosec // bounded config value
		int(c.LogConfig.KeepDays),
		c.LogConfig.Console,
	)
	if logkit != nil {
		defer func() { _ = logkit.Sync() }()
	}
	if err := infra.NormalizeDirPaths(&c.DataDir, &c.ScanDir, &c.SaveDir, &c.LibraryDir); err != nil {
		return fmt.Errorf("normalize paths: %w", err)
	}
	logutil.GetLogger(ctx).Info("start capture run",
		zap.String("scan_dir", c.ScanDir),
		zap.String("save_dir", c.SaveDir),
		zap.String("data_dir", c.DataDir),
	)

	rt, err := domain.BuildCaptureRuntime(ctx, toCaptureRuntimeConfig(c))
	if err != nil {
		return fmt.Errorf("build capture runtime: %w", err)
	}
	defer rt.Close()

	capt, err := buildCaptureFromConfig(ctx, c, rt)
	if err != nil {
		return err
	}
	if err := capt.Run(ctx); err != nil {
		return fmt.Errorf("run capture: %w", err)
	}
	logutil.GetLogger(ctx).Info("capture run finished")
	return nil
}

func prepareCapturePlugins(ctx context.Context, c *config.Config, rt *domain.CaptureRuntime) error {
	sources := toPluginSources(c.SearcherPluginConfig.Sources)
	_, err := domain.PrepareSearcherPlugins(ctx, rt.CLI, c.DataDir, sources,
		func(resolveCtx context.Context, resolved *pluginbundle.ResolvedBundle) error {
			defaultPlugins, catPlugins := domain.ResolvedPluginConfig(resolved)
			domain.LogPluginBundleWarnings(resolveCtx, resolved.Warnings)
			c.Plugins = defaultPlugins
			c.CategoryPlugins = fromDomainCategoryPlugins(catPlugins)
			return nil
		},
	)
	if err != nil && !errors.Is(err, domain.ErrNoPluginSources) {
		return fmt.Errorf("prepare searcher plugins: %w", err)
	}
	return nil
}

func buildCaptureFromConfig(
	ctx context.Context,
	c *config.Config,
	rt *domain.CaptureRuntime,
) (*capture.Capture, error) {
	if err := prepareCapturePlugins(ctx, c, rt); err != nil {
		return nil, err
	}
	plugOpts := toPluginOptions(c.PluginConfig)
	ss, err := domain.BuildSearcher(
		ctx, rt.CLI, rt.CacheStore, c.SwitchConfig.EnableSearchMetaCache,
		c.Plugins, plugOpts,
	)
	if err != nil {
		return nil, fmt.Errorf("build searchers: %w", err)
	}
	catSs, err := domain.BuildCatSearcher(
		ctx, rt.CLI, rt.CacheStore, c.SwitchConfig.EnableSearchMetaCache,
		toCategoryPlugins(c.CategoryPlugins), plugOpts,
	)
	if err != nil {
		return nil, fmt.Errorf("build category searchers: %w", err)
	}
	processors, err := domain.BuildProcessor(ctx, appdeps.Runtime{
		HTTPClient: rt.CLI, Storage: rt.CacheStore,
		Translator: rt.Translator, AIEngine: rt.Engine, FaceRec: rt.FaceRec,
	}, c.Handlers, toHandlerOptions(c.HandlerConfig))
	if err != nil {
		return nil, fmt.Errorf("build processors: %w", err)
	}
	cleaner, _, err := domain.BuildMovieIDCleaner(
		ctx, rt.CLI, c.DataDir,
		c.MovieIDRulesetConfig.SourceType, c.MovieIDRulesetConfig.Location,
	)
	if err != nil {
		return nil, fmt.Errorf("build movieid cleaner: %w", err)
	}
	capt, err := domain.BuildCapture(
		domain.CaptureConfig{
			Naming:                 c.Naming,
			ScanDir:                c.ScanDir,
			SaveDir:                c.SaveDir,
			ExtraMediaExts:         c.ExtraMediaExts,
			DiscardTranslatedTitle: c.TranslateConfig.DiscardTranslatedTitle,
			DiscardTranslatedPlot:  c.TranslateConfig.DiscardTranslatedPlot,
			EnableLinkMode:         c.SwitchConfig.EnableLinkMode,
		},
		rt.CacheStore,
		searcher.NewCategorySearcher(ss, catSs),
		processors, cleaner,
	)
	if err != nil {
		return nil, fmt.Errorf("build capture: %w", err)
	}
	return capt, nil
}

func toCaptureRuntimeConfig(c *config.Config) domain.CaptureRuntimeConfig {
	var flareCfg *infra.FlareSolverrConfig
	if c.FlareSolverrConfig.Enable {
		flareCfg = &infra.FlareSolverrConfig{
			Host: c.FlareSolverrConfig.Host,
		}
	}
	deps := make([]infra.DependencySpec, 0, len(c.Dependencies))
	for _, d := range c.Dependencies {
		deps = append(deps, infra.DependencySpec{
			URL: d.Link, RelPath: d.RelPath, Refresh: d.Refresh,
		})
	}
	ec := c.TranslateConfig.EngineConfig
	return domain.CaptureRuntimeConfig{
		DataDir:          c.DataDir,
		Proxy:            c.NetworkConfig.Proxy,
		BrowserRemoteURL: c.BrowserConfig.RemoteURL,
		HTTPClient: infra.HTTPClientConfig{
			TimeoutSec: c.NetworkConfig.Timeout,
			Proxy:      c.NetworkConfig.Proxy,
		},
		FlareSolverr:      flareCfg,
		Dependencies:      deps,
		AIEngineName:      c.AIEngine.Name,
		AIEngineArgs:      c.AIEngine.Args,
		EnablePigoFaceRec: c.SwitchConfig.EnablePigoFaceRecognizer,
		Translator: bootrt.TranslatorConfig{
			Engine: c.TranslateConfig.Engine, Fallback: c.TranslateConfig.Fallback,
			Proxy: c.NetworkConfig.Proxy,
			Google: bootrt.GoogleTranslatorConfig{
				Enable: ec.Google.Enable, UseProxy: ec.Google.UseProxy,
			},
			AI: bootrt.AITranslatorConfig{
				Enable: ec.AI.Enable, Prompt: ec.AI.Prompt,
			},
		},
	}
}

func fromDomainCategoryPlugins(items []domain.CategoryPlugin) []config.CategoryPlugin {
	out := make([]config.CategoryPlugin, 0, len(items))
	for _, item := range items {
		out = append(out, config.CategoryPlugin{
			Name: item.Name, Plugins: item.Plugins,
		})
	}
	return out
}

func toPluginSources(items []config.SearcherPluginSource) []domain.PluginSource {
	out := make([]domain.PluginSource, 0, len(items))
	for _, item := range items {
		out = append(out, domain.PluginSource{
			SourceType: item.SourceType, Location: item.Location,
		})
	}
	return out
}

func toPluginOptions(m map[string]config.PluginConfig) map[string]domain.PluginOption {
	out := make(map[string]domain.PluginOption, len(m))
	for k, v := range m {
		out[k] = domain.PluginOption{Disable: v.Disable}
	}
	return out
}

func toCategoryPlugins(items []config.CategoryPlugin) []domain.CategoryPlugin {
	out := make([]domain.CategoryPlugin, 0, len(items))
	for _, item := range items {
		out = append(out, domain.CategoryPlugin{
			Name: item.Name, Plugins: item.Plugins,
		})
	}
	return out
}

func toHandlerOptions(m map[string]config.HandlerConfig) map[string]domain.HandlerOption {
	out := make(map[string]domain.HandlerOption, len(m))
	for k, v := range m {
		out[k] = domain.HandlerOption{Disable: v.Disable, Args: v.Args}
	}
	return out
}
