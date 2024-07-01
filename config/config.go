package config

import (
	"encoding/json"
	"os"

	"github.com/xxxsen/common/logger"
)

type Config struct {
	ScanDir         string                 `json:"scan_dir"`
	SaveDir         string                 `json:"save_dir"`
	DataDir         string                 `json:"data_dir"`
	ModelDir        string                 `json:"model_dir"`
	Naming          string                 `json:"naming"`
	SearcherConfig  map[string]interface{} `json:"searcher_config"`
	ProcessorConfig map[string]interface{} `json:"processor_config"`
	Searchers       []string               `json:"searchers"`
	Processors      []string               `json:"processors"`
	LogConfig       logger.LogConfig       `json:"log_config"`
	SwitchConfig    SwitchConfig           `json:"switch_config"`
}

type SwitchConfig struct {
	EnableLinkMode bool `json:"enable_link_mode"`
}

func Parse(f string) (*Config, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	return c, nil
}
