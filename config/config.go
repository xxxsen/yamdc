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

type AIEngineConfig struct {
	Name string      `json:"name"`
	Args interface{} `json:"args"`
}

type TranslateConfig struct {
	Enable                 bool   `json:"enable"`
	Engine                 string `json:"engine"`
	DiscardTranslatedTitle bool   `json:"discard_translated_title"`
	DiscardTranslatedPlot  bool   `json:"discard_translated_plot"`
}

type HandlerConfig struct {
	Disable bool        `json:"disable"`
	Data    interface{} `json:"data"`
}

type PluginConfig struct {
	Disable       bool        `json:"disable"`
	EnableFlarerr bool        `json:"enable_flarerr"` //是否启用flaresolverr
	Data          interface{} `json:"data"`
}

type FlareSolverrConfig struct {
	Enable     bool     `json:"enable"`
	Host       string   `json:"host"`
	DomainList []string `json:"domain_list"` //需要使用flaresolverr的域名列表
}

type Config struct {
	ScanDir            string                   `json:"scan_dir"`
	SaveDir            string                   `json:"save_dir"`
	DataDir            string                   `json:"data_dir"`
	Naming             string                   `json:"naming"`
	PluginConfig       map[string]PluginConfig  `json:"plugin_config"`
	HandlerConfig      map[string]HandlerConfig `json:"handler_config"`
	AIEngine           AIEngineConfig           `json:"ai_engine"`
	Plugins            []string                 `json:"plugins"`
	CategoryPlugins    []CategoryPlugin         `json:"category_plugins"`
	Handlers           []string                 `json:"handlers"`
	ExtraMediaExts     []string                 `json:"extra_media_exts"`
	LogConfig          logger.LogConfig         `json:"log_config"`
	Dependencies       []Dependency             `json:"dependencies"`
	NetworkConfig      NetworkConfig            `json:"network_config"`
	TranslateConfig    TranslateConfig          `json:"translate_config"`
	RuleConfig         RuleConfig               `json:"rule_config"`
	SwitchConfig       SwitchConfig             `json:"switch_config"`
	FlareSolverrConfig FlareSolverrConfig       `json:"flare_solverr_config"`
}

type SwitchConfig struct {
	EnableSearchMetaCache    bool `json:"enable_search_meta_cache"`    //开启搜索缓存
	EnableLinkMode           bool `json:"enable_link_mode"`            //测试场景下使用, 开启链接模式
	EnableGoFaceRecognizer   bool `json:"enable_go_face_recognizer"`   //开启goface人脸识别
	EnablePigoFaceRecognizer bool `json:"enable_pigo_face_recognizer"` //开启pigo人脸识别
	EnableSearcherCheck      bool `json:"enable_searcher_check"`       //测试场景使用, 检查插件的目标域名是否还能访问
}

type LinkConfig struct {
	Type string `json:"type"`
	Link string `json:"link"`
}

type RuleConfig struct {
	NumberRewriter       LinkConfig `json:"number_rewriter"`
	NumberCategorier     LinkConfig `json:"number_categorier"`
	NumberUncensorTester LinkConfig `json:"number_uncensor_tester"`
}

func defaultConfig() *Config {
	return &Config{
		Plugins:         sysPlugins,
		CategoryPlugins: sysCategoryPlugins,
		Handlers:        sysHandler,
		LogConfig:       sysLogConfig,
		Dependencies:    sysDependencies,
		RuleConfig:      sysRuleConfig,
		PluginConfig:    sysPluginConfig,
		SwitchConfig: SwitchConfig{
			EnableSearchMetaCache:    true,
			EnableLinkMode:           false,
			EnableGoFaceRecognizer:   true,
			EnablePigoFaceRecognizer: true,
			EnableSearcherCheck:      false,
		},
		TranslateConfig: TranslateConfig{
			Enable: true,
			Engine: "google",
		},
		FlareSolverrConfig: FlareSolverrConfig{
			Enable: false, //默认不启用, 毕竟还要额外配置
			Host:   "http://127.0.0.1:8191",
		},
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
