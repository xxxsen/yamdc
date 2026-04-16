package config

import "os"

// ApplyEnvOverrides applies environment variable overrides to the switch config.
// This provides backward compatibility for legacy env-based configuration.
func ApplyEnvOverrides(c *SwitchConfig) {
	if os.Getenv("ENABLE_SEARCH_META_CACHE") == "false" {
		c.EnableSearchMetaCache = false
	}
	if os.Getenv("ENABLE_PIGO_FACE_RECOGNIZER") == "false" {
		c.EnablePigoFaceRecognizer = false
	}
}
