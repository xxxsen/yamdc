package config

import (
	"encoding/json"
	"os"

	"github.com/xxxsen/common/logger"
)

type CategoryPlugin struct {
	Name    string   `json:"name"`
	Plugins []string `json:"plugins"`
}

type Config struct {
	ScanDir         string                 `json:"scan_dir"`
	SaveDir         string                 `json:"save_dir"`
	DataDir         string                 `json:"data_dir"`
	Naming          string                 `json:"naming"`
	PluginConfig    map[string]interface{} `json:"plugin_config"`
	HandlerConfig   map[string]interface{} `json:"handler_config"`
	Plugins         []string               `json:"plugins"`
	CategoryPlugins []CategoryPlugin       `json:"category_plugins"`
	Handlers        []string               `json:"handlers"`
	ExtraMediaExts  []string               `json:"extra_media_exts"`
	LogConfig       logger.LogConfig       `json:"log_config"`
	SwitchConfig    SwitchConfig           `json:"switch_config"`
}

type SwitchConfig struct {
	EnableLinkMode bool `json:"enable_link_mode"`
}

func defaultConfig() *Config {
	return &Config{
		Plugins: []string{
			"javbus",
			"javhoo",
			"airav",
			"javdb",
			"jav321",
			"caribpr",
			"18av",
			"freejavbt",
			"tktube",
			"avsox",
		},
		CategoryPlugins: []CategoryPlugin{
			//如果存在分配配置, 那么当番号被识别为特定分类的场景下, 将会使用分类插件直接查询
			{Name: "FC2", Plugins: []string{"fc2", "18av", "freejavbt", "tktube", "avsox"}},
		},
		Handlers: []string{
			"image_transcoder",
			"poster_cropper",
			"watermark_maker",
			"tag_padder",
			"duration_fixer",
			"translater",
		},
		LogConfig: logger.LogConfig{
			Level:   "info",
			Console: true,
		},
	}
}

func Parse(f string) (*Config, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	c := defaultConfig()
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	return c, nil
}
