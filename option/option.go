package option

import "sync/atomic"

var atmSwitch atomic.Value

type SwitchConfig struct {
	EnableMetaCache  bool `json:"enable_meta_cahce"`
	EnableMediaCache bool `json:"enable_media_cache"`
}

func SetSwitchConfig(sw *SwitchConfig) {
	atmSwitch.Store(sw)
}

func GetSwitchConfig() *SwitchConfig {
	return atmSwitch.Load().(*SwitchConfig)
}
