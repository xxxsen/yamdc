package config

import (
	"encoding/json"
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
}

type ProxyConfig struct {
	Addr     string `json:"addr"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type NetworkConfig struct {
	Timeout int64  `json:"timeout"` //单位为秒
	Proxy   string `json:"proxy"`
}

type NumberRewriteRule struct {
	Remark  string `json:"remark"`
	Rule    string `json:"rule"`
	Rewrite string `json:"rewrite"`
}

type NumberCategoryRule struct {
	Remark   string   `json:"remark"`
	Rules    []string `json:"rules"`
	Category string   `json:"category"`
}

type NumberRule struct {
	NumberUncensorRules []string             `json:"number_uncensor_rules"`
	NumberRewriteRules  []NumberRewriteRule  `json:"number_rewrite_rules"`
	NumberCategoryRule  []NumberCategoryRule `json:"number_category_rules"`
}

type AIEngineConfig struct {
	Name string      `json:"name"`
	Args interface{} `json:"args"`
}

type TranslateConfig struct {
	DiscardTranslatedTitle bool `json:"discard_translated_title"`
	DiscardTranslatedPlot  bool `json:"discard_translated_plot"`
}

type Config struct {
	ScanDir           string                 `json:"scan_dir"`
	SaveDir           string                 `json:"save_dir"`
	DataDir           string                 `json:"data_dir"`
	Naming            string                 `json:"naming"`
	PluginConfig      map[string]interface{} `json:"plugin_config"`
	HandlerConfig     map[string]interface{} `json:"handler_config"`
	AIEngine          AIEngineConfig         `json:"ai_engine"`
	Plugins           []string               `json:"plugins"`
	CategoryPlugins   []CategoryPlugin       `json:"category_plugins"`
	Handlers          []string               `json:"handlers"`
	ExtraMediaExts    []string               `json:"extra_media_exts"`
	LogConfig         logger.LogConfig       `json:"log_config"`
	Dependencies      []Dependency           `json:"dependencies"`
	NetworkConfig     NetworkConfig          `json:"network_config"`
	NumberDefaultRule NumberRule             `json:"number_default_rule"` //默认规则
	NumberUserRule    NumberRule             `json:"number_user_rule"`    //用户自定义规则, 最终跟默认规则进行合并
	TranslateConfig   TranslateConfig        `json:"translate_config"`
}

func defaultConfig() *Config {
	return &Config{
		Plugins:           sysPlugins,
		CategoryPlugins:   sysCategoryPlugins,
		Handlers:          sysHandler,
		LogConfig:         sysLogConfig,
		Dependencies:      sysDependencies,
		NumberDefaultRule: sysNumberRule,
	}
}

func Parse(f string) (*Config, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	raw, err = hujson.Standardize(raw)
	if err != nil {
		return nil, err
	}
	c := defaultConfig()
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	return c, nil
}
