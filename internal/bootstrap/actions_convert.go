package bootstrap

import (
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/bootstrap/infra"
	bootrt "github.com/xxxsen/yamdc/internal/bootstrap/runtime"
	"github.com/xxxsen/yamdc/internal/config"
)

// config → sub-package type conversions.
//
// 这里集中放所有 `to*` / `from*` 适配器:
//   - 面向 infra / runtime / domain 子包的入参转换 (to*)
//   - 面向 config 的反向转换 (from*)
//
// 保持这些函数纯粹 (无副作用, 无 logger), 方便表驱动测试。

func toHTTPClientConfig(c *config.Config) infra.HTTPClientConfig {
	return infra.HTTPClientConfig{
		TimeoutSec: c.NetworkConfig.Timeout,
		Proxy:      c.NetworkConfig.Proxy,
	}
}

func toDependencySpecs(deps []config.Dependency) []infra.DependencySpec {
	out := make([]infra.DependencySpec, 0, len(deps))
	for _, d := range deps {
		out = append(out, infra.DependencySpec{
			URL: d.Link, RelPath: d.RelPath, Refresh: d.Refresh,
		})
	}
	return out
}

func toTranslatorConfig(c *config.Config) bootrt.TranslatorConfig {
	ec := c.TranslateConfig.EngineConfig
	return bootrt.TranslatorConfig{
		Engine:   c.TranslateConfig.Engine,
		Fallback: c.TranslateConfig.Fallback,
		Proxy:    c.NetworkConfig.Proxy,
		Google: bootrt.GoogleTranslatorConfig{
			Enable:   ec.Google.Enable,
			UseProxy: ec.Google.UseProxy,
		},
		AI: bootrt.AITranslatorConfig{
			Enable: ec.AI.Enable,
			Prompt: ec.AI.Prompt,
		},
	}
}

func toPluginOptions(m map[string]config.PluginConfig) map[string]domain.PluginOption {
	out := make(map[string]domain.PluginOption, len(m))
	for k, v := range m {
		out[k] = domain.PluginOption{Disable: v.Disable}
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

func toCategoryPlugins(items []config.CategoryPlugin) []domain.CategoryPlugin {
	out := make([]domain.CategoryPlugin, 0, len(items))
	for _, item := range items {
		out = append(out, domain.CategoryPlugin{
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

func toCaptureConfig(c *config.Config) domain.CaptureConfig {
	return domain.CaptureConfig{
		Naming:                 c.Naming,
		ScanDir:                c.ScanDir,
		SaveDir:                c.SaveDir,
		ExtraMediaExts:         c.ExtraMediaExts,
		DiscardTranslatedTitle: c.TranslateConfig.DiscardTranslatedTitle,
		DiscardTranslatedPlot:  c.TranslateConfig.DiscardTranslatedPlot,
		EnableLinkMode:         c.SwitchConfig.EnableLinkMode,
	}
}

func categoryPluginStringMap(c *config.Config) map[string][]string {
	m := make(map[string][]string, len(c.CategoryPlugins))
	for _, item := range c.CategoryPlugins {
		m[item.Name] = append([]string(nil), item.Plugins...)
	}
	return m
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
