package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
)

func TestLoadPluginCaseJSONFileInfersPluginFromFileName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alpha.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "cases": [
    {"name":"a","input":"ABC-123","output":{"status":"success"}}
  ]
}`), 0o600))

	out, err := loadPluginCaseJSONFile(path)
	require.NoError(t, err)
	require.Equal(t, "alpha", out.Plugin)
	require.Len(t, out.Cases, 1)
}

func TestLoadPluginCaseFileFromDirScansJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha.json"), []byte(`{
  "cases": [
    {"name":"a","input":"ABC-123","output":{"status":"success"}}
  ]
}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "beta.json"), []byte(`{
  "cases": [
    {"name":"b","input":"XYZ-456","output":{"status":"not_found"}}
  ]
}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte(`{}`), 0o600))

	out, err := loadPluginCaseFile(dir)
	require.NoError(t, err)
	require.Len(t, out.Cases, 2)
	require.Equal(t, "a", out.Cases[0].Name)
	require.Equal(t, "b", out.Cases[1].Name)
}

func TestResolvePluginRuntimeName(t *testing.T) {
	resolved := &pluginbundle.ResolvedBundle{
		Plugins: map[string][]byte{
			"alpha":                         []byte("a"),
			"__bundle__SOURCE_A__bundle_a":  []byte("b"),
			"__bundle__COLLECTION__special": []byte("c"),
		},
	}

	name, err := resolvePluginRuntimeName(resolved, "alpha")
	require.NoError(t, err)
	require.Equal(t, "alpha", name)

	name, err = resolvePluginRuntimeName(resolved, "bundle_a")
	require.NoError(t, err)
	require.Equal(t, "__bundle__SOURCE_A__bundle_a", name)

	name, err = resolvePluginRuntimeName(resolved, "special")
	require.NoError(t, err)
	require.Equal(t, "__bundle__COLLECTION__special", name)
}
