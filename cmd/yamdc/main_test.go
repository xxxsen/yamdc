package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	bootrt "github.com/xxxsen/yamdc/internal/bootstrap/runtime"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestPrecheckCaptureDirDoesNotRequireLibraryDir(t *testing.T) {
	tmp := t.TempDir()
	c := &config.Config{
		DataDir: filepath.Join(tmp, "data"),
		ScanDir: filepath.Join(tmp, "scan"),
		SaveDir: filepath.Join(tmp, "save"),
	}
	require.NoError(t, config.ValidateForCapture(c))
}

func TestPrecheckServerDirRequiresLibraryDir(t *testing.T) {
	tmp := t.TempDir()
	c := &config.Config{
		DataDir: filepath.Join(tmp, "data"),
		ScanDir: filepath.Join(tmp, "scan"),
		SaveDir: filepath.Join(tmp, "save"),
	}
	require.EqualError(t, config.ValidateForServer(c), "no library dir")
}

func TestBuildMovieIDCleanerReturnsNonNilManagerOnSuccess(t *testing.T) {
	dataDir := t.TempDir()
	ruleDir := filepath.Join(t.TempDir(), "rules")
	require.NoError(t, os.MkdirAll(filepath.Join(ruleDir, "ruleset"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ruleDir, "manifest.yaml"), []byte(`
entry: ruleset
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ruleDir, "ruleset", "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0o600))

	cleaner, manager, err := domain.BuildMovieIDCleaner(
		context.Background(), client.MustNewClient(),
		dataDir, movieidcleaner.SourceTypeLocal, ruleDir,
	)
	require.NoError(t, err)
	require.NotNil(t, cleaner)
	require.NotNil(t, manager)
}

func TestBuildMovieIDCleanerAllowsMissingSource(t *testing.T) {
	cleaner, manager, err := domain.BuildMovieIDCleaner(
		context.Background(), client.MustNewClient(),
		t.TempDir(), "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, cleaner)
	require.Nil(t, manager)

	result, err := cleaner.Clean("abc-123")
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPrepareSearcherPluginsAllowsBlankRemoteLocation(t *testing.T) {
	sources := []domain.PluginSource{
		{SourceType: "remote", Location: ""},
	}
	manager, err := domain.PrepareSearcherPlugins(
		context.Background(), client.MustNewClient(),
		t.TempDir(), sources, nil,
	)
	require.ErrorIs(t, err, domain.ErrNoPluginSources)
	require.Nil(t, manager)
}

func TestBuildTranslatorSelectsConfiguredOrderAndDedupes(t *testing.T) {
	cfg := bootrt.TranslatorConfig{
		Engine:   "ai",
		Fallback: []string{"google", "ai"},
		Google:   bootrt.GoogleTranslatorConfig{Enable: true},
		AI:       bootrt.AITranslatorConfig{Enable: true},
	}
	tr, err := bootrt.BuildTranslator(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, tr)
	require.Equal(t, "G:[ai,google]", tr.Name())
}

func TestBuildSearcherWithCreatorsSupportsMergedPluginBundles(t *testing.T) {
	dataDir := t.TempDir()
	leftDir := filepath.Join(t.TempDir(), "bundle-left")
	rightDir := filepath.Join(t.TempDir(), "bundle-right")
	writePluginBundleDir(t, leftDir, map[string]string{
		"manifest.yaml": `
version: 1
name: left
entry: plugins
chains:
  all:
    - name: alpha
      priority: 200
`,
		"plugins/alpha.yaml": sampleTestPluginYAML("alpha"),
	})
	writePluginBundleDir(t, rightDir, map[string]string{
		"manifest.yaml": `
version: 1
name: right
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
    - name: beta
      priority: 150
  source_a:
    - name: alpha
      priority: 120
`,
		"plugins/alpha.yaml": sampleTestPluginYAML("alpha"),
		"plugins/beta.yaml":  sampleTestPluginYAML("beta"),
	})

	var latest *pluginbundle.ResolvedBundle
	manager, err := pluginbundle.NewManager("searcher_plugin", dataDir, client.MustNewClient(),
		[]pluginbundle.Source{
			{SourceType: pluginbundle.SourceTypeLocal, Location: leftDir},
			{SourceType: pluginbundle.SourceTypeLocal, Location: rightDir},
		},
		func(_ context.Context, resolved *pluginbundle.ResolvedBundle, _ []string) error {
			latest = resolved
			return nil
		},
	)
	require.NoError(t, err)
	require.NoError(t, manager.Start(context.Background()))
	require.NotNil(t, latest)
	require.Equal(t, []string{"alpha", "beta"}, latest.DefaultPlugins)
	require.Equal(t, []string{"__bundle__SOURCE_A__alpha"}, latest.CategoryChains["SOURCE_A"])

	registerCtx := pluginyaml.BuildRegisterContext(latest.Plugins)
	creators := registerCtx.Snapshot()
	searchers, err := domain.BuildSearcherWithCreators(
		context.Background(), client.MustNewClient(), store.NewMemStorage(),
		false, latest.DefaultPlugins, nil, creators,
	)
	require.NoError(t, err)
	require.Len(t, searchers, 2)

	categorySearchers, err := domain.BuildCatSearcherWithCreators(
		context.Background(), client.MustNewClient(), store.NewMemStorage(),
		false, []domain.CategoryPlugin{
			{
				Name:    "SOURCE_A",
				Plugins: latest.CategoryChains["SOURCE_A"],
			},
		}, nil, creators,
	)
	require.NoError(t, err)
	require.Len(t, categorySearchers["SOURCE_A"], 1)
}

func writePluginBundleDir(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(content), 0o600))
	}
}

func sampleTestPluginYAML(name string) string {
	return `
version: 1
name: ` + name + `
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true
`
}
