package config

import "fmt"

// LoadAppConfig is the unified config entry point. It parses the config file,
// applies environment variable overrides, and validates according to mode.
func LoadAppConfig(path string, mode AppMode) (*Config, error) {
	c, err := Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}
	ApplyEnvOverrides(&c.SwitchConfig)
	switch mode {
	case ModeCapture:
		if err := ValidateForCapture(c); err != nil {
			return nil, fmt.Errorf("config validation failed for %s mode: %w", mode, err)
		}
	case ModeServer:
		// Server mode validation is deferred to bootstrap (after dir path normalization).
	}
	return c, nil
}
