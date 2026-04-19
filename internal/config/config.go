package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tailscale/hujson"
	"github.com/xxxsen/common/logger"
)

type CategoryPlugin struct {
	Name    string   `json:"name"`
	Plugins []string `json:"plugins"`
}

type Dependency struct {
	Link    string `json:"link"`
	RelPath string `json:"rel_path"`
	Refresh bool   `json:"refresh"`
}

type ProxyConfig struct {
	Addr     string `json:"addr"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type NetworkConfig struct {
	Timeout int64  `json:"timeout"` // 单位为秒
	Proxy   string `json:"proxy"`
}

type AIEngineConfig struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

type TranslateConfig struct {
	Enable                 bool                  `json:"enable"`
	Engine                 string                `json:"engine"`
	Fallback               []string              `json:"fallback"`
	DiscardTranslatedTitle bool                  `json:"discard_translated_title"`
	DiscardTranslatedPlot  bool                  `json:"discard_translated_plot"`
	EngineConfig           TranslateEngineConfig `json:"engine_config"`
}

type TranslateEngineConfig struct {
	Google GoogleTranslateEngineConfig `json:"google"`
	AI     AITranslateEngineConfig     `json:"ai"`
}

type GoogleTranslateEngineConfig struct {
	Enable   bool `json:"enable"`
	UseProxy bool `json:"use_proxy"`
}

type AITranslateEngineConfig struct {
	Enable bool   `json:"enable"`
	Prompt string `json:"prompt"`
}

type HandlerConfig struct {
	Disable bool `json:"disable"`
	Args    any  `json:"args"`
}

type PluginConfig struct {
	Disable bool `json:"disable"`
}

type FlareSolverrConfig struct {
	Enable bool   `json:"enable"`
	Host   string `json:"host"`
}

type TagMappingConfig struct {
	Enable   bool   `json:"enable"`    // 是否启用标签映射功能
	FilePath string `json:"file_path"` // 配置文件路径
}

type MovieIDRulesetConfig struct {
	SourceType string `json:"source_type"`
	Location   string `json:"location"`
}

type SearcherPluginSource struct {
	SourceType string `json:"source_type"`
	Location   string `json:"location"`
}

type SearcherPluginConfig struct {
	Sources []SearcherPluginSource `json:"sources"`
}

type BrowserConfig struct {
	RemoteURL string `json:"remote_url"`
}

type Config struct {
	ScanDir              string                   `json:"scan_dir"`
	SaveDir              string                   `json:"save_dir"`
	LibraryDir           string                   `json:"library_dir"`
	DataDir              string                   `json:"data_dir"`
	Naming               string                   `json:"naming"`
	PluginConfig         map[string]PluginConfig  `json:"plugin_config"`
	HandlerConfig        map[string]HandlerConfig `json:"handler_config"`
	AIEngine             AIEngineConfig           `json:"ai_engine"`
	Plugins              []string                 `json:"plugins"`
	CategoryPlugins      []CategoryPlugin         `json:"category_plugins"`
	Handlers             []string                 `json:"handlers"`
	ExtraMediaExts       []string                 `json:"extra_media_exts"`
	LogConfig            logger.LogConfig         `json:"log_config"`
	Dependencies         []Dependency             `json:"dependencies"`
	NetworkConfig        NetworkConfig            `json:"network_config"`
	TranslateConfig      TranslateConfig          `json:"translate_config"`
	SwitchConfig         SwitchConfig             `json:"switch_config"`
	FlareSolverrConfig   FlareSolverrConfig       `json:"flare_solverr_config"`
	TagMappingConfig     TagMappingConfig         `json:"tag_mapping_config"`
	MovieIDRulesetConfig MovieIDRulesetConfig     `json:"movieid_ruleset_config"`
	SearcherPluginConfig SearcherPluginConfig     `json:"searcher_plugin_config"`
	BrowserConfig        BrowserConfig            `json:"browser_config"`
}

type SwitchConfig struct {
	EnableSearchMetaCache    bool `json:"enable_search_meta_cache"`    // 开启搜索缓存
	EnableLinkMode           bool `json:"enable_link_mode"`            // 测试场景下使用, 开启链接模式
	EnablePigoFaceRecognizer bool `json:"enable_pigo_face_recognizer"` // 开启pigo人脸识别
	EnableSearcherCheck      bool `json:"enable_searcher_check"`       // 测试场景使用, 检查插件的目标域名是否还能访问
}

func defaultConfig() *Config {
	return &Config{
		Plugins:         sysPlugins,
		CategoryPlugins: sysCategoryPlugins,
		Handlers:        sysHandler,
		LogConfig:       sysLogConfig,
		Dependencies:    sysDependencies,
		SwitchConfig: SwitchConfig{
			EnableSearchMetaCache:    true,
			EnableLinkMode:           false,
			EnablePigoFaceRecognizer: true,
			EnableSearcherCheck:      false,
		},
		TranslateConfig: TranslateConfig{
			Enable:   true,
			Engine:   "google",
			Fallback: []string{"google"},
			EngineConfig: TranslateEngineConfig{
				Google: GoogleTranslateEngineConfig{
					Enable:   true,
					UseProxy: true,
				},
				AI: AITranslateEngineConfig{
					Enable: true,
					Prompt: "", // 不填则默认使用默认的prompt
				},
			},
		},
		FlareSolverrConfig: FlareSolverrConfig{
			Enable: false,
			Host:   "http://127.0.0.1:8191",
		},
		TagMappingConfig: TagMappingConfig{
			Enable:   false, // 默认不启用, 毕竟还要额外配置
			FilePath: "",    // 如果启用,需要指定配置文件路径
		},
		MovieIDRulesetConfig: MovieIDRulesetConfig{},
		SearcherPluginConfig: SearcherPluginConfig{
			Sources: []SearcherPluginSource{},
		},
	}
}

func Parse(f string) (*Config, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("read config file %s failed: %w", f, err)
	}
	raw, err = hujson.Standardize(raw)
	if err != nil {
		return nil, fmt.Errorf("standardize config json failed: %w", err)
	}
	c := defaultConfig()
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}
	return c, nil
}
