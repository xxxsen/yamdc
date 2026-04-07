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
	path := filepath.Join(dir, "javbus.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "cases": [
    {"name":"a","input":"ABC-123","output":{"status":"success"}}
  ]
}`), 0644))

	out, err := loadPluginCaseJSONFile(path)
	require.NoError(t, err)
	require.Equal(t, "javbus", out.Plugin)
	require.Len(t, out.Cases, 1)
}

func TestLoadPluginCaseFileFromDirScansJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "javbus.json"), []byte(`{
  "cases": [
    {"name":"a","input":"ABC-123","output":{"status":"success"}}
  ]
}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "missav.json"), []byte(`{
  "cases": [
    {"name":"b","input":"XYZ-456","output":{"status":"not_found"}}
  ]
}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte(`{}`), 0644))

	out, err := loadPluginCaseFile(dir)
	require.NoError(t, err)
	require.Len(t, out.Cases, 2)
	require.Equal(t, "a", out.Cases[0].Name)
	require.Equal(t, "b", out.Cases[1].Name)
}

func TestResolvePluginRuntimeName(t *testing.T) {
	resolved := &pluginbundle.ResolvedBundle{
		Plugins: map[string][]byte{
			"javbus":                     []byte("a"),
			"__bundle__FC2__fc2":         []byte("b"),
			"__bundle__COSPURI__cospuri": []byte("c"),
		},
	}

	name, err := resolvePluginRuntimeName(resolved, "javbus")
	require.NoError(t, err)
	require.Equal(t, "javbus", name)

	name, err = resolvePluginRuntimeName(resolved, "fc2")
	require.NoError(t, err)
	require.Equal(t, "__bundle__FC2__fc2", name)

	name, err = resolvePluginRuntimeName(resolved, "cospuri")
	require.NoError(t, err)
	require.Equal(t, "__bundle__COSPURI__cospuri", name)
}
