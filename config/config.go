package config

import (
	"encoding/json"
	"os"

	"github.com/xxxsen/common/logger"
)

type Config struct {
	ScanDir        string                 `json:"scan_dir"`
	SaveDir        string                 `json:"save_dir"`
	DataDir        string                 `json:"data_dir"`
	Naming         string                 `json:"naming"`
	PluginConfig   map[string]interface{} `json:"plugin_config"`
	HandlerConfig  map[string]interface{} `json:"handler_config"`
	Plugins        []string               `json:"plugins"`
	Handlers       []string               `json:"handlers"`
	ExtraMediaExts []string               `json:"extra_media_exts"`
	LogConfig      logger.LogConfig       `json:"log_config"`
	SwitchConfig   SwitchConfig           `json:"switch_config"`
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
			"jav321",
			"caribpr",
			"fc2",
			"freejavbt",
			"avsox",
		},
		Handlers: []string{
			"image_transcoder",
			"poster_cropper",
			"watermark_maker",
			"tag_padder",
			"duration_fixer",
			"plot_translater",
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
