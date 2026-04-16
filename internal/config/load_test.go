package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAppConfigCaptureModeHappyPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	content := `{
		"data_dir": "/data",
		"scan_dir": "/scan",
		"save_dir": "/save"
	}`
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	c, err := LoadAppConfig(p, ModeCapture)
	require.NoError(t, err)
	assert.Equal(t, "/data", c.DataDir)
	assert.Equal(t, "/scan", c.ScanDir)
	assert.Equal(t, "/save", c.SaveDir)
}

func TestLoadAppConfigCaptureModeValidationFailure(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{}`), 0o600))
	_, err := LoadAppConfig(p, ModeCapture)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
}

func TestLoadAppConfigServerModeSkipsValidation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{}`), 0o600))
	c, err := LoadAppConfig(p, ModeServer)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestLoadAppConfigBadFilePath(t *testing.T) {
	_, err := LoadAppConfig(filepath.Join(t.TempDir(), "nope.json"), ModeCapture)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config failed")
}

func TestLoadAppConfigAppliesEnvOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	content := `{
		"data_dir": "/data",
		"scan_dir": "/scan",
		"save_dir": "/save"
	}`
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	t.Setenv("ENABLE_SEARCH_META_CACHE", "false")
	c, err := LoadAppConfig(p, ModeCapture)
	require.NoError(t, err)
	assert.False(t, c.SwitchConfig.EnableSearchMetaCache)
}

func TestAppModeString(t *testing.T) {
	assert.Equal(t, "capture", ModeCapture.String())
	assert.Equal(t, "server", ModeServer.String())
	assert.Equal(t, "unknown", AppMode(99).String())
}
