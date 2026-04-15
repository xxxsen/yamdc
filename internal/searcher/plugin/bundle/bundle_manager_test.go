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
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
