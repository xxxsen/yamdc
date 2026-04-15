package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"
)

const (
	testData = `
{
	/* this is a test comment */
	"a": 1,
	"b": 3.14, // hello?
	"c": true,
	//also comment here
	"d": ["a", "b"], //asdasdsadasd
}
	`
)

type testSt struct {
	A int      `json:"a"`
	B float64  `json:"b"`
	C bool     `json:"c"`
	D []string `json:"d"`
}

func TestJsonWithComments(t *testing.T) {
	st := &testSt{}
	data, err := hujson.Standardize([]byte(testData))
	assert.NoError(t, err)
	err = json.Unmarshal(data, st)
	assert.NoError(t, err)
	t.Logf("%+v", *st)
	assert.Equal(t, 1, st.A)
	assert.Equal(t, 3.14, st.B)
	assert.Equal(t, true, st.C)
}

func TestDefaultConfigDoesNotPinRemoteBundleURLs(t *testing.T) {
	c := defaultConfig()
	require.Empty(t, c.MovieIDRulesetConfig.SourceType)
	require.Empty(t, c.MovieIDRulesetConfig.Location)
	require.Empty(t, c.SearcherPluginConfig.Sources)
}

func TestParseReadFileError(t *testing.T) {
	_, err := Parse(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "read config file")
}

func TestParseHujsonStandardizeError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.hujson")
	require.NoError(t, os.WriteFile(p, []byte{0xff, 0xfe, 0xfd}, 0o600))
	_, err := Parse(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "standardize config json")
}

func TestParseJSONUnmarshalError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad-types.json")
	// Valid JSON but wrong types for Config fields.
	require.NoError(t, os.WriteFile(p, []byte(`{"scan_dir": true}`), 0o600))
	_, err := Parse(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unmarshal config")
}

func TestParseSuccessMergesOntoDefaultsWithHujsonComments(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	content := `
{
	// library roots
	"scan_dir": "/media/incoming",
	"save_dir": "/media/staging",
	"switch_config": {
		"enable_link_mode": true
	}
}
`
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	c, err := Parse(p)
	require.NoError(t, err)
	require.Equal(t, "/media/incoming", c.ScanDir)
	require.Equal(t, "/media/staging", c.SaveDir)
	require.True(t, c.SwitchConfig.EnableLinkMode)
	// Defaults not mentioned in file remain from defaultConfig().
	require.True(t, c.SwitchConfig.EnableSearchMetaCache)
	require.NotEmpty(t, c.Handlers)
	require.Equal(t, "info", c.LogConfig.Level)
}

func TestParseEmptyJSONKeepsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.json")
	require.NoError(t, os.WriteFile(p, []byte(`{}`), 0o600))
	c, err := Parse(p)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.True(t, c.TranslateConfig.Enable)
	require.Equal(t, "google", c.TranslateConfig.Engine)
}
