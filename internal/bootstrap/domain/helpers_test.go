package domain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
)

func TestResolveRuleSourcePathFindsYAMLFile(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, "rules.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("test"), 0o600))
	got, err := ResolveRuleSourcePath(dir, "rules.yaml")
	require.NoError(t, err)
	assert.Equal(t, yamlFile, got)
}

func TestResolveRuleSourcePathFindsDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "rules")
	require.NoError(t, os.Mkdir(subDir, 0o755))
	got, err := ResolveRuleSourcePath(dir, "rules")
	require.NoError(t, err)
	assert.Equal(t, subDir, got)
}

func TestResolveRuleSourcePathNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveRuleSourcePath(dir, "nonexistent")
	require.ErrorIs(t, err, ErrRuleSourceNotFound)
}

func TestResolveBundleSourcePathFindsDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "bundle")
	require.NoError(t, os.Mkdir(subDir, 0o755))
	got, err := ResolveBundleSourcePath(dir, "bundle")
	require.NoError(t, err)
	assert.Equal(t, subDir, got)
}

func TestResolveBundleSourcePathNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveBundleSourcePath(dir, "nonexistent")
	require.ErrorIs(t, err, ErrBundleSourceNotFound)
}

func TestConfiguredPluginSourcesFiltersBlank(t *testing.T) {
	sources := []PluginSource{
		{SourceType: "local", Location: "/some/path"},
		{SourceType: "remote", Location: ""},
		{SourceType: "remote", Location: "   "},
	}
	got := ConfiguredPluginSources(sources)
	require.Len(t, got, 1)
	assert.Equal(t, "/some/path", got[0].Location)
}

func TestHasMovieIDRulesetSource(t *testing.T) {
	assert.False(t, HasMovieIDRulesetSource(""))
	assert.False(t, HasMovieIDRulesetSource("  "))
	assert.True(t, HasMovieIDRulesetSource("/some/path"))
}

func TestResolvedPluginConfigNilBundle(t *testing.T) {
	plugins, categories := ResolvedPluginConfig(nil)
	assert.Nil(t, plugins)
	assert.Nil(t, categories)
}

func TestResolvedPluginConfigSortsCategoryNames(t *testing.T) {
	resolved := &pluginbundle.ResolvedBundle{
		DefaultPlugins: []string{"a", "b"},
		CategoryChains: map[string][]string{
			"Z_CAT": {"p1"},
			"A_CAT": {"p2"},
		},
	}
	plugins, categories := ResolvedPluginConfig(resolved)
	require.Equal(t, []string{"a", "b"}, plugins)
	require.Len(t, categories, 2)
	assert.Equal(t, "A_CAT", categories[0].Name)
	assert.Equal(t, "Z_CAT", categories[1].Name)
}

func TestCategoryPluginMap(t *testing.T) {
	items := []CategoryPlugin{
		{Name: "A", Plugins: []string{"p1", "p2"}},
		{Name: "B", Plugins: []string{"p3"}},
	}
	m := CategoryPluginMap(items)
	require.Equal(t, []string{"p1", "p2"}, m["A"])
	require.Equal(t, []string{"p3"}, m["B"])
}
