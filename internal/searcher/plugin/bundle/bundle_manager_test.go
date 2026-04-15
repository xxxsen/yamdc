package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"

	"github.com/stretchr/testify/require"
)

func TestLoadBundleFromDir(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: bundle-a
entry: plugins
chains:
  all:
    - name: beta
      priority: 200
  source_a:
    - name: alpha
      priority: 200
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
		"plugins/beta.yaml":  samplePluginYAML("beta"),
	})
	bundle, files, err := LoadBundleFromDir(dir)
	require.NoError(t, err)
	require.Equal(t, "bundle-a", bundle.Manifest.Name)
	require.Len(t, bundle.Plugins, 2)
	require.Equal(t, []string{"plugins/alpha.yaml", "plugins/beta.yaml"}, files)
}

func TestResolveBundles(t *testing.T) {
	left := &Bundle{
		Manifest: &Manifest{
			Version: 1,
			Name:    "left",
			Entry:   "plugins",
			Chains: map[string][]*PluginChainItem{
				"all": {
					{Name: "beta", Priority: 200},
					{Name: "alpha", Priority: 200},
				},
				"SOURCE_A": {
					{Name: "cat-only", Priority: 150},
				},
			},
		},
		Plugins: map[string]*PluginFile{
			"alpha":    {Name: "alpha", Data: []byte(samplePluginYAML("alpha-left"))},
			"beta":     {Name: "beta", Data: []byte(samplePluginYAML("beta"))},
			"cat-only": {Name: "cat-only", Data: []byte(samplePluginYAML("cat-only"))},
		},
		Order: 0,
	}
	right := &Bundle{
		Manifest: &Manifest{
			Version: 1,
			Name:    "right",
			Entry:   "plugins",
			Chains: map[string][]*PluginChainItem{
				"all": {
					{Name: "alpha", Priority: 200},
				},
				"SOURCE_A": {
					{Name: "alpha", Priority: 100},
					{Name: "beta", Priority: 100},
				},
			},
		},
		Plugins: map[string]*PluginFile{
			"alpha": {Name: "alpha", Data: []byte(samplePluginYAML("alpha-right"))},
			"beta":  {Name: "beta", Data: []byte(samplePluginYAML("beta"))},
		},
		Order: 1,
	}
	resolved, err := resolveBundles([]*Bundle{left, right})
	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "beta"}, resolved.DefaultPlugins)
	require.Equal(t, []string{"__bundle__SOURCE_A__alpha", "__bundle__SOURCE_A__beta", "__bundle__SOURCE_A__cat-only"}, resolved.CategoryChains["SOURCE_A"])
	require.Equal(t, samplePluginYAML("alpha-left"), string(resolved.Plugins["alpha"]))
	require.Equal(t, samplePluginYAML("alpha-right"), string(resolved.Plugins["__bundle__SOURCE_A__alpha"]))
	require.Equal(t, samplePluginYAML("beta"), string(resolved.Plugins["__bundle__SOURCE_A__beta"]))
	require.NotEmpty(t, resolved.Warnings)
}

func TestRemoteBundleManagerLoadFallsBackToCachedZip(t *testing.T) {
	dataDir := t.TempDir()
	archive := buildTestBundleZip(t, map[string]string{
		"yamdc-plugins-v1/manifest.yaml": `
version: 1
name: remote
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"yamdc-plugins-v1/plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	fail := false
	var latest *ResolvedBundle
	manager, err := NewManager("searcher_plugin", dataDir, stubHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		if fail {
			return nil, fmt.Errorf("network down")
		}
		switch {
		case req.URL.Host == "api.github.com" && req.URL.Path == "/repos/xxxsen/yamdc-plugins/tags":
			return newHTTPResponse(http.StatusOK, []byte(`[{"name":"v1.0.0"}]`), "application/json"), nil
		case req.URL.Host == "codeload.github.com" && req.URL.Path == "/xxxsen/yamdc-plugins/zip/refs/tags/v1.0.0":
			return newHTTPResponse(http.StatusOK, archive, "application/zip"), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
		}
	}}, []Source{{SourceType: SourceTypeRemote, Location: "https://github.com/xxxsen/yamdc-plugins"}}, func(_ context.Context, resolved *ResolvedBundle, _ []string) error {
		latest = resolved
		return nil
	})
	require.NoError(t, err)

	err = manager.Start(context.Background())
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, []string{"alpha"}, latest.DefaultPlugins)
	require.Equal(t, []string{"yamdc-plugins-v1/plugins/alpha.yaml"}, latest.Files)

	fail = true
	latest = nil
	manager, err = NewManager("searcher_plugin", dataDir, stubHTTPClient{do: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network down")
	}}, []Source{{SourceType: SourceTypeRemote, Location: "https://github.com/xxxsen/yamdc-plugins"}}, func(_ context.Context, resolved *ResolvedBundle, _ []string) error {
		latest = resolved
		return nil
	})
	require.NoError(t, err)
	err = manager.Start(context.Background())
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, []string{"alpha"}, latest.DefaultPlugins)
	require.Equal(t, []string{"yamdc-plugins-v1/plugins/alpha.yaml"}, latest.Files)
}

func TestManagerEmitSerializesCallbacks(t *testing.T) {
	var (
		inFlight    atomic.Int64
		maxInFlight atomic.Int64
	)
	manager := &Manager{
		cb: func(_ context.Context, _ *ResolvedBundle, _ []string) error {
			active := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				current := maxInFlight.Load()
				if active <= current || maxInFlight.CompareAndSwap(current, active) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			return nil
		},
		bundles: map[int]*Bundle{
			0: {
				Manifest: &Manifest{
					Version: 1,
					Name:    "bundle-a",
					Entry:   "plugins",
					Chains: map[string][]*PluginChainItem{
						"all": {
							{Name: "alpha", Priority: 100},
						},
					},
				},
				Plugins: map[string]*PluginFile{
					"alpha": {Name: "alpha", Data: []byte(samplePluginYAML("alpha"))},
				},
			},
		},
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, manager.emit(context.Background()))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, manager.emit(context.Background()))
	}()
	wg.Wait()
	require.EqualValues(t, 1, maxInFlight.Load())
}

func samplePluginYAML(name string) string {
	return fmt.Sprintf(`
version: 1
name: %s
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
`, name)
}

func writeBundleFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(content), 0o600))
	}
}

func buildTestBundleZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(f, content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

type stubHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (s stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if s.do == nil {
		return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
	}
	return s.do(req)
}

func newHTTPResponse(status int, body []byte, contentType string) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

// --- LoadResolvedBundleFromDir ---

func TestLoadResolvedBundleFromDir(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: bundle-a
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	resolved, files, err := LoadResolvedBundleFromDir(dir)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, []string{"alpha"}, resolved.DefaultPlugins)
	require.NotEmpty(t, files)
}

func TestLoadResolvedBundleFromDir_InvalidDir(t *testing.T) {
	_, _, err := LoadResolvedBundleFromDir("/nonexistent/path")
	require.Error(t, err)
}

func TestLoadResolvedBundleFromDir_NotADir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notadir.txt")
	require.NoError(t, os.WriteFile(file, []byte("hello"), 0o600))
	_, _, err := LoadResolvedBundleFromDir(file)
	require.Error(t, err)
}

// --- LoadResolvedBundleFromData ---

func TestLoadResolvedBundleFromData(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: bundle-data
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	archive := buildTestBundleZip(t, map[string]string{
		"manifest.yaml":      readFileString(t, filepath.Join(dir, "manifest.yaml")),
		"plugins/alpha.yaml": readFileString(t, filepath.Join(dir, "plugins/alpha.yaml")),
	})
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	data := &basebundle.Data{
		FS:     zipReader,
		Base:   ".",
		Source: "test-source",
	}
	resolved, files, err := LoadResolvedBundleFromData(data)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.NotEmpty(t, files)
}

func TestLoadResolvedBundleFromData_Nil(t *testing.T) {
	_, _, err := LoadResolvedBundleFromData(nil)
	require.Error(t, err)
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// --- LoadBundleFromZip ---

func TestLoadBundleFromZip(t *testing.T) {
	dir := t.TempDir()
	archive := buildTestBundleZip(t, map[string]string{
		"yamdc-plugins/manifest.yaml": `
version: 1
name: zip-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"yamdc-plugins/plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	zipPath := filepath.Join(dir, "bundle.zip")
	require.NoError(t, os.WriteFile(zipPath, archive, 0o600))

	bundle, files, err := LoadBundleFromZip(zipPath, 0)
	require.NoError(t, err)
	require.Equal(t, "zip-bundle", bundle.Manifest.Name)
	require.NotEmpty(t, files)
}

func TestLoadBundleFromZip_BadFile(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.zip")
	require.NoError(t, os.WriteFile(badFile, []byte("not a zip"), 0o600))
	_, _, err := LoadBundleFromZip(badFile, 0)
	require.Error(t, err)
}

// --- detectZipRoot ---

func TestDetectZipRoot_SingleRoot(t *testing.T) {
	archive := buildTestBundleZip(t, map[string]string{
		"myroot/file1.txt": "a",
		"myroot/file2.txt": "b",
	})
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	root := detectZipRoot(reader.File)
	require.Equal(t, "myroot", root)
}

func TestDetectZipRoot_MultipleRoots(t *testing.T) {
	archive := buildTestBundleZip(t, map[string]string{
		"a/file1.txt": "a",
		"b/file2.txt": "b",
	})
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	root := detectZipRoot(reader.File)
	require.Equal(t, "", root)
}

func TestDetectZipRoot_FlatFiles(t *testing.T) {
	archive := buildTestBundleZip(t, map[string]string{
		"file1.txt": "a",
		"file2.txt": "b",
	})
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	root := detectZipRoot(reader.File)
	require.Equal(t, "", root)
}

// --- validateManifest: all error paths ---

func TestValidateManifest_AllErrors(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		wantErr  bool
	}{
		{name: "nil", manifest: nil, wantErr: true},
		{name: "bad_version", manifest: &Manifest{Version: 2, Name: "x", Entry: "p"}, wantErr: true},
		{name: "no_name", manifest: &Manifest{Version: 1, Entry: "p"}, wantErr: true},
		{name: "no_entry", manifest: &Manifest{Version: 1, Name: "x"}, wantErr: true},
		{name: "nil_chain_item", manifest: &Manifest{Version: 1, Name: "x", Entry: "p", Chains: map[string][]*PluginChainItem{"all": {nil}}}, wantErr: true},
		{name: "empty_chain_name", manifest: &Manifest{Version: 1, Name: "x", Entry: "p", Chains: map[string][]*PluginChainItem{"all": {{Name: "", Priority: 100}}}}, wantErr: true},
		{name: "priority_zero", manifest: &Manifest{Version: 1, Name: "x", Entry: "p", Chains: map[string][]*PluginChainItem{"all": {{Name: "a", Priority: 0}}}}, wantErr: true},
		{name: "priority_too_high", manifest: &Manifest{Version: 1, Name: "x", Entry: "p", Chains: map[string][]*PluginChainItem{"all": {{Name: "a", Priority: 1001}}}}, wantErr: true},
		{name: "duplicate_chain_item", manifest: &Manifest{Version: 1, Name: "x", Entry: "p", Chains: map[string][]*PluginChainItem{"all": {{Name: "a", Priority: 100}, {Name: "a", Priority: 200}}}}, wantErr: true},
		{name: "valid", manifest: &Manifest{Version: 1, Name: "x", Entry: "p", Chains: map[string][]*PluginChainItem{"all": {{Name: "a", Priority: 100}}}}, wantErr: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateManifest(tc.manifest)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- decodePluginName: error paths ---

func TestDecodePluginName_Errors(t *testing.T) {
	_, err := decodePluginName([]byte(`invalid: {yaml`))
	require.Error(t, err)

	_, err = decodePluginName([]byte(`version: 1`))
	require.Error(t, err)
}

// --- cleanBundleEntry ---

func TestCleanBundleEntry(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "plugins", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "dot", input: ".", wantErr: true},
		{name: "dotdot", input: "..", wantErr: true},
		{name: "parent_escape", input: "../foo", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cleanBundleEntry(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- runtimePluginKey / normalizeCategory ---

func TestRuntimePluginKey(t *testing.T) {
	require.Equal(t, "myplug", runtimePluginKey("all", "myplug"))
	require.Equal(t, "myplug", runtimePluginKey("", "myplug"))
	require.Equal(t, "__bundle__CAT__myplug", runtimePluginKey("cat", "myplug"))
}

func TestNormalizeCategory(t *testing.T) {
	require.Equal(t, "all", normalizeCategory("all"))
	require.Equal(t, "all", normalizeCategory("ALL"))
	require.Equal(t, "all", normalizeCategory(""))
	require.Equal(t, "MYSRC", normalizeCategory("mysrc"))
}

// --- NewManager: nil callback ---

func TestNewManager_NilCallback(t *testing.T) {
	_, err := NewManager("test", t.TempDir(), stubHTTPClient{}, nil, nil)
	require.Error(t, err)
}

// --- validateBundlePlugins: unknown plugin ---

func TestValidateBundlePlugins_UnknownPlugin(t *testing.T) {
	manifest := &Manifest{
		Version: 1, Name: "x", Entry: "p",
		Chains: map[string][]*PluginChainItem{"all": {{Name: "nonexistent", Priority: 100}}},
	}
	err := validateBundlePlugins(manifest, map[string]*PluginFile{})
	require.Error(t, err)
}

// --- loadBundleFromFS: no plugins ---

func TestLoadBundleFromDir_NoPlugins(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: empty-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
	})
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "plugins"), 0o755))
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- loadBundleFromFS: manifest not found ---

func TestLoadBundleFromDir_NoManifest(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "plugins"), 0o755))
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- loadPluginFilesFromFS: duplicate plugin name ---

func TestLoadBundleFromDir_DuplicatePlugin(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: dup-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yaml":    samplePluginYAML("alpha"),
		"plugins/alpha2.yaml":   samplePluginYAML("alpha"),
	})
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- joinBundlePath ---

func TestJoinBundlePath(t *testing.T) {
	require.Equal(t, "foo", joinBundlePath("", "foo"))
	require.Equal(t, "foo", joinBundlePath(".", "foo"))
	require.Equal(t, "base/foo", joinBundlePath("base", "foo"))
}

// --- collectBundleCandidates: nil bundle ---

func TestCollectBundleCandidates_NilBundle(t *testing.T) {
	plugins, chains, files := collectBundleCandidates([]*Bundle{nil})
	require.Empty(t, plugins)
	require.Empty(t, chains)
	require.Empty(t, files)
}

// --- resolvePluginWinners: single candidate ---

func TestResolvePluginWinners_SingleCandidate(t *testing.T) {
	candidates := map[string][]pluginCandidate{
		"key1": {{name: "alpha", category: "all", runtimeKey: "alpha", priority: 100, data: []byte("data")}},
	}
	plugins, warnings := resolvePluginWinners(candidates)
	require.Len(t, plugins, 1)
	require.Empty(t, warnings)
}

// --- readManifestFromFS: .yml extension ---

func TestLoadBundleFromDir_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yml": `
version: 1
name: yml-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yml": samplePluginYAML("alpha"),
	})
	bundle, _, err := LoadBundleFromDir(dir)
	require.NoError(t, err)
	require.Equal(t, "yml-bundle", bundle.Manifest.Name)
}

// --- resolveBundles: no all chain ---

func TestResolveBundles_NoAllChain(t *testing.T) {
	bundle := &Bundle{
		Manifest: &Manifest{
			Version: 1, Name: "x", Entry: "p",
			Chains: map[string][]*PluginChainItem{
				"SOURCE_A": {{Name: "alpha", Priority: 100}},
			},
		},
		Plugins: map[string]*PluginFile{
			"alpha": {Name: "alpha", Data: []byte("data")},
		},
	}
	resolved, err := resolveBundles([]*Bundle{bundle})
	require.NoError(t, err)
	require.Empty(t, resolved.DefaultPlugins)
	found := false
	for _, w := range resolved.Warnings {
		if strings.Contains(w, "no all chain") {
			found = true
		}
	}
	require.True(t, found, "expected 'no all chain' warning")
}

// --- resolveBundles: empty chain name should be ignored ---

func TestCollectBundleCandidates_EmptyChainItemName(t *testing.T) {
	bundle := &Bundle{
		Manifest: &Manifest{
			Version: 1, Name: "x", Entry: "p",
			Chains: map[string][]*PluginChainItem{
				"all": {{Name: "  ", Priority: 100}, {Name: "alpha", Priority: 100}},
			},
		},
		Plugins: map[string]*PluginFile{
			"alpha": {Name: "alpha", Data: []byte("data")},
		},
	}
	plugins, _, _ := collectBundleCandidates([]*Bundle{bundle})
	require.Len(t, plugins, 1)
}

// --- resolvePluginWinners: same priority generates warning ---

func TestResolvePluginWinners_SamePriorityWarning(t *testing.T) {
	candidates := map[string][]pluginCandidate{
		"key1": {
			{name: "alpha", category: "all", runtimeKey: "alpha", priority: 100, bundleName: "left", data: []byte("a"), order: 0},
			{name: "alpha", category: "all", runtimeKey: "alpha", priority: 100, bundleName: "right", data: []byte("b"), order: 1},
		},
	}
	plugins, warnings := resolvePluginWinners(candidates)
	require.Len(t, plugins, 1)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "ignored")
}

// --- resolveChainWinners: same priority generates warning ---

func TestResolveChainWinners_SamePriorityWarning(t *testing.T) {
	chainGroups := map[string][]ChainItem{
		"all\x00alpha": {
			{Name: "alpha", Category: allCategory, Priority: 100, BundleName: "b1", Order: 0},
			{Name: "alpha", Category: allCategory, Priority: 100, BundleName: "b2", Order: 1},
		},
	}
	allItems, _, warnings := resolveChainWinners(chainGroups)
	require.Len(t, allItems, 1)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "ignored")
}

// --- resolveChainWinners: category chain ---

func TestResolveChainWinners_CategoryChain(t *testing.T) {
	chainGroups := map[string][]ChainItem{
		"SOURCE_A\x00beta": {
			{Name: "beta", Category: "SOURCE_A", Priority: 100, BundleName: "b1", Order: 0},
		},
	}
	allItems, categoryItems, warnings := resolveChainWinners(chainGroups)
	require.Empty(t, allItems)
	require.Len(t, categoryItems["SOURCE_A"], 1)
	require.Empty(t, warnings)
}

// --- loadBundleFromFS: invalid manifest entry ---

func TestLoadBundleFromDir_BadEntry(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: bad-entry
entry: "../escape"
chains:
  all:
    - name: alpha
      priority: 100
`,
	})
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- loadBundleFromFS: invalid manifest yaml ---

func TestLoadBundleFromDir_InvalidManifestYAML(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `invalid: {yaml`,
	})
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- loadPluginFilesFromFS: non-yaml file is skipped ---

func TestLoadBundleFromDir_NonYAMLFileSkipped(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: skip-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
		"plugins/readme.txt": "just a readme",
	})
	bundle, files, err := LoadBundleFromDir(dir)
	require.NoError(t, err)
	require.Equal(t, "skip-bundle", bundle.Manifest.Name)
	require.Len(t, files, 1)
}

// --- loadPluginFilesFromFS: invalid plugin yaml ---

func TestLoadBundleFromDir_InvalidPluginYAML(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: bad-plugin
entry: plugins
chains:
  all:
    - name: broken
      priority: 100
`,
		"plugins/broken.yaml": `invalid: {yaml`,
	})
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- loadPluginFilesFromFS: plugin without name field ---

func TestLoadBundleFromDir_PluginNoName(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: no-name-plugin
entry: plugins
chains:
  all:
    - name: noname
      priority: 100
`,
		"plugins/noname.yaml": `
version: 1
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
`,
	})
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- detectZipRoot: empty file names ---

func TestDetectZipRoot_EmptyFileNames(t *testing.T) {
	archive := buildTestBundleZip(t, map[string]string{
		"root/file1.txt": "a",
	})
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	origName := reader.File[0].Name
	reader.File[0].Name = ""
	root := detectZipRoot(reader.File)
	require.Equal(t, "", root)
	reader.File[0].Name = origName
}

func TestDetectZipRoot_NoFiles(t *testing.T) {
	archive := buildTestBundleZip(t, map[string]string{})
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	root := detectZipRoot(reader.File)
	require.Equal(t, "", root)
}

// --- LoadBundleFromZip: non-existent file ---

func TestLoadBundleFromZip_NonExistent(t *testing.T) {
	_, _, err := LoadBundleFromZip("/nonexistent/file.zip", 0)
	require.Error(t, err)
}

// --- LoadBundleFromZip: flat zip (no root) ---

func TestLoadBundleFromZip_FlatZip(t *testing.T) {
	dir := t.TempDir()
	archive := buildTestBundleZip(t, map[string]string{
		"manifest.yaml": `
version: 1
name: flat-zip
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	zipPath := filepath.Join(dir, "flat.zip")
	require.NoError(t, os.WriteFile(zipPath, archive, 0o600))
	bundle, files, err := LoadBundleFromZip(zipPath, 0)
	require.NoError(t, err)
	require.Equal(t, "flat-zip", bundle.Manifest.Name)
	require.NotEmpty(t, files)
}

// --- Manager.emit: callback returns error ---

func TestManagerEmit_CallbackError(t *testing.T) {
	manager := &Manager{
		cb: func(_ context.Context, _ *ResolvedBundle, _ []string) error {
			return fmt.Errorf("callback error")
		},
		bundles: map[int]*Bundle{
			0: {
				Manifest: &Manifest{
					Version: 1, Name: "a", Entry: "p",
					Chains: map[string][]*PluginChainItem{
						"all": {{Name: "alpha", Priority: 100}},
					},
				},
				Plugins: map[string]*PluginFile{
					"alpha": {Name: "alpha", Data: []byte(samplePluginYAML("alpha"))},
				},
			},
		},
	}
	err := manager.emit(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "callback error")
}

// --- Manager.Start: triggers emit ---

func TestManagerStart_TriggersEmit(t *testing.T) {
	called := false
	manager := &Manager{
		cb: func(_ context.Context, _ *ResolvedBundle, _ []string) error {
			called = true
			return nil
		},
		bundles: map[int]*Bundle{
			0: {
				Manifest: &Manifest{
					Version: 1, Name: "a", Entry: "p",
					Chains: map[string][]*PluginChainItem{
						"all": {{Name: "alpha", Priority: 100}},
					},
				},
				Plugins: map[string]*PluginFile{
					"alpha": {Name: "alpha", Data: []byte(samplePluginYAML("alpha"))},
				},
			},
		},
	}
	err := manager.Start(context.Background())
	require.NoError(t, err)
	require.True(t, called)
}

// --- LoadResolvedBundleFromData: bad data in zip ---

func TestLoadResolvedBundleFromData_InvalidPlugin(t *testing.T) {
	archive := buildTestBundleZip(t, map[string]string{
		"manifest.yaml": `
version: 1
name: bad-data
entry: plugins
chains:
  all:
    - name: bad
      priority: 100
`,
		"plugins/bad.yaml": `invalid: {yaml`,
	})
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	data := &basebundle.Data{
		FS:     zipReader,
		Base:   ".",
		Source: "test-source",
	}
	_, _, err = LoadResolvedBundleFromData(data)
	require.Error(t, err)
}

// --- loadBundleFromFS: validateBundlePlugins error ---

func TestLoadBundleFromDir_MissingChainPlugin(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: missing-ref
entry: plugins
chains:
  all:
    - name: nonexistent
      priority: 100
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	_, _, err := LoadBundleFromDir(dir)
	require.Error(t, err)
}

// --- resolveBundles: multiple bundles, second overrides first ---

func TestResolveBundles_MultiBundle_HigherPriorityWins(t *testing.T) {
	b1 := &Bundle{
		Manifest: &Manifest{
			Version: 1, Name: "b1", Entry: "p",
			Chains: map[string][]*PluginChainItem{
				"all": {{Name: "alpha", Priority: 200}},
			},
		},
		Plugins: map[string]*PluginFile{
			"alpha": {Name: "alpha", Data: []byte("b1-data")},
		},
		Order: 0,
	}
	b2 := &Bundle{
		Manifest: &Manifest{
			Version: 1, Name: "b2", Entry: "p",
			Chains: map[string][]*PluginChainItem{
				"all": {{Name: "alpha", Priority: 100}},
			},
		},
		Plugins: map[string]*PluginFile{
			"alpha": {Name: "alpha", Data: []byte("b2-data")},
		},
		Order: 1,
	}
	resolved, err := resolveBundles([]*Bundle{b1, b2})
	require.NoError(t, err)
	require.Equal(t, "b2-data", string(resolved.Plugins["alpha"]))
}

// --- Manager.emit: empty bundles ---

func TestManagerEmit_EmptyBundles(t *testing.T) {
	var result *ResolvedBundle
	manager := &Manager{
		cb: func(_ context.Context, resolved *ResolvedBundle, _ []string) error {
			result = resolved
			return nil
		},
		bundles: map[int]*Bundle{},
	}
	err := manager.emit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.DefaultPlugins)
}

// --- chainItemRuntimeNames ---

func TestChainItemRuntimeNames(t *testing.T) {
	items := []ChainItem{
		{Name: "alpha", Category: allCategory},
		{Name: "beta", Category: "SOURCE_A"},
	}
	names := chainItemRuntimeNames(items)
	require.Equal(t, []string{"alpha", "__bundle__SOURCE_A__beta"}, names)
}

// --- sortChainItems ---

func TestSortChainItems(t *testing.T) {
	items := []ChainItem{
		{Name: "beta", Priority: 200, Order: 0},
		{Name: "alpha", Priority: 100, Order: 0},
		{Name: "gamma", Priority: 100, Order: 1},
	}
	sortChainItems(items)
	require.Equal(t, "alpha", items[0].Name)
	require.Equal(t, "gamma", items[1].Name)
	require.Equal(t, "beta", items[2].Name)
}

// --- detectZipRoot: slash-only entries ---

func TestDetectZipRoot_SlashOnlyEntries(t *testing.T) {
	root := detectZipRoot([]*zip.File{})
	require.Equal(t, "", root)
}

// --- readManifestFromFS: unreadable manifest ---

func TestReadManifestFromFS_UnreadableDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "manifest.yaml"), 0o755))
	_, err := readManifestFromFS(os.DirFS(dir), ".")
	require.Error(t, err)
}

// --- loadPluginFilesFromFS: directory walk error ---

func TestLoadPluginFilesFromFS_EntryNotExists(t *testing.T) {
	dir := t.TempDir()
	_, _, err := loadPluginFilesFromFS(os.DirFS(dir), "nonexistent")
	require.Error(t, err)
}

// --- loadPluginFilesFromFS: unreadable plugin file ---

func TestLoadPluginFilesFromFS_UnreadablePluginFile(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(pluginsDir, "bad.yaml"), 0o000))
	_, _, err := loadPluginFilesFromFS(os.DirFS(dir), "plugins")
	require.Error(t, err)
}

// --- resolvePluginWinners: different priority candidates ---

func TestResolvePluginWinners_DifferentPriority(t *testing.T) {
	candidates := map[string][]pluginCandidate{
		"key1": {
			{name: "alpha", category: "all", runtimeKey: "alpha", priority: 200, bundleName: "left", data: []byte("a"), order: 0},
			{name: "alpha", category: "all", runtimeKey: "alpha", priority: 100, bundleName: "right", data: []byte("b"), order: 1},
		},
	}
	plugins, warnings := resolvePluginWinners(candidates)
	require.Len(t, plugins, 1)
	require.Equal(t, "b", string(plugins["alpha"]))
	require.Empty(t, warnings)
}

// --- NewManager: valid creation with sources ---

func TestNewManager_ValidWithSources(t *testing.T) {
	dataDir := t.TempDir()
	manager, err := NewManager("test", dataDir, stubHTTPClient{}, []Source{{SourceType: SourceTypeLocal, Location: dataDir}}, func(_ context.Context, _ *ResolvedBundle, _ []string) error {
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, manager)
}

// --- Manager.Start: emit error ---

func TestManagerStart_EmitError(t *testing.T) {
	manager := &Manager{
		cb: func(_ context.Context, _ *ResolvedBundle, _ []string) error {
			return fmt.Errorf("emit failed")
		},
		bundles: map[int]*Bundle{
			0: {
				Manifest: &Manifest{
					Version: 1, Name: "a", Entry: "p",
					Chains: map[string][]*PluginChainItem{
						"all": {{Name: "alpha", Priority: 100}},
					},
				},
				Plugins: map[string]*PluginFile{
					"alpha": {Name: "alpha", Data: []byte(samplePluginYAML("alpha"))},
				},
			},
		},
	}
	err := manager.Start(context.Background())
	require.Error(t, err)
}

// --- LoadBundleFromDir with whitespace dir path ---

func TestLoadBundleFromDir_WhitespacePath(t *testing.T) {
	dir := t.TempDir()
	writeBundleFiles(t, dir, map[string]string{
		"manifest.yaml": `
version: 1
name: ws-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
`,
		"plugins/alpha.yaml": samplePluginYAML("alpha"),
	})
	bundle, _, err := LoadBundleFromDir("  " + dir + "  ")
	require.NoError(t, err)
	require.Equal(t, "ws-bundle", bundle.Manifest.Name)
}

// --- LoadBundleFromZip: zip with bad manifest ---

func TestLoadBundleFromZip_BadManifest(t *testing.T) {
	dir := t.TempDir()
	archive := buildTestBundleZip(t, map[string]string{
		"root/manifest.yaml": `version: 99`,
	})
	zipPath := filepath.Join(dir, "bad.zip")
	require.NoError(t, os.WriteFile(zipPath, archive, 0o600))
	_, _, err := LoadBundleFromZip(zipPath, 0)
	require.Error(t, err)
}

// --- emit: multi-bundle sort by order ---

func TestManagerEmit_SortByOrder(t *testing.T) {
	var result *ResolvedBundle
	manager := &Manager{
		cb: func(_ context.Context, resolved *ResolvedBundle, _ []string) error {
			result = resolved
			return nil
		},
		bundles: map[int]*Bundle{
			1: {
				Manifest: &Manifest{
					Version: 1, Name: "b2", Entry: "p",
					Chains: map[string][]*PluginChainItem{
						"all": {{Name: "beta", Priority: 100}},
					},
				},
				Plugins: map[string]*PluginFile{
					"beta": {Name: "beta", Data: []byte("beta-data")},
				},
				Order: 1,
			},
			0: {
				Manifest: &Manifest{
					Version: 1, Name: "b1", Entry: "p",
					Chains: map[string][]*PluginChainItem{
						"all": {{Name: "alpha", Priority: 50}},
					},
				},
				Plugins: map[string]*PluginFile{
					"alpha": {Name: "alpha", Data: []byte("alpha-data")},
				},
				Order: 0,
			},
		},
	}
	err := manager.emit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{"alpha", "beta"}, result.DefaultPlugins)
}
