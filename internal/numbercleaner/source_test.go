package numbercleaner

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRuleSetFromDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "002-normalizers.yaml"), []byte(`
version: v1
normalizers:
  - name: basename
    type: builtin
    builtin: basename
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "003-matchers.yaml"), []byte(`
version: v1
matchers:
  - name: generic
    pattern: '(?i)\b([A-Z]{2,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 80
`), 0644))

	rs, err := LoadRuleSetFromPath(dir)
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.Equal(t, "v1", rs.Version)
	require.Len(t, rs.Normalizers, 1)
	require.Len(t, rs.Matchers, 1)
}

func TestLoadRuleSetFromDirConflict(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-a.yaml"), []byte(`
version: v1
matchers:
  - name: dup
    pattern: '(?i)AAA([0-9]+)'
    normalize_template: 'AAA-$1'
    score: 80
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "002-b.yaml"), []byte(`
version: v1
matchers:
  - name: dup
    pattern: '(?i)BBB([0-9]+)'
    normalize_template: 'BBB-$1'
    score: 80
`), 0644))

	_, err := LoadRuleSetFromPath(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate rule name across fragments")
}

func TestLoadRuleSetFromZipUsesManifestEntry(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "rules.zip")
	require.NoError(t, os.WriteFile(zipPath, buildTestBundleZip(t, map[string]string{
		"yamdc-script-v1/manifest.yaml": `entry: custom-rules`,
		"yamdc-script-v1/custom-rules/001-base.yaml": `
version: v1
options:
  case_mode: upper
`,
		"yamdc-script-v1/custom-rules/002-matchers.yaml": `
version: v1
matchers:
  - name: generic
    pattern: '(?i)\b([A-Z]{2,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 80
`,
	}), 0644))

	rs, files, err := LoadRuleSetFromZip(zipPath)
	require.NoError(t, err)
	require.Equal(t, "v1", rs.Version)
	require.Len(t, rs.Matchers, 1)
	require.Equal(t, []string{"yamdc-script-v1/custom-rules/001-base.yaml", "yamdc-script-v1/custom-rules/002-matchers.yaml"}, files)
}

func TestLoadRuleSetFromZipUsesDefaultRulesetEntry(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "rules.zip")
	require.NoError(t, os.WriteFile(zipPath, buildTestBundleZip(t, map[string]string{
		"yamdc-script-v1/ruleset/001-base.yaml": `
version: v1
options:
  case_mode: upper
`,
		"yamdc-script-v1/ruleset/002-matchers.yaml": `
version: v1
matchers:
  - name: generic
    pattern: '(?i)\b([A-Z]{2,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 80
`,
	}), 0644))

	rs, files, err := LoadRuleSetFromZip(zipPath)
	require.NoError(t, err)
	require.Equal(t, "v1", rs.Version)
	require.Len(t, rs.Matchers, 1)
	require.Equal(t, []string{"yamdc-script-v1/ruleset/001-base.yaml", "yamdc-script-v1/ruleset/002-matchers.yaml"}, files)
}

func TestRemoteBundleManagerLoadFallsBackToCachedZip(t *testing.T) {
	dataDir := t.TempDir()
	archive := buildTestBundleZip(t, map[string]string{
		"yamdc-script-v1/manifest.yaml": `entry: ruleset`,
		"yamdc-script-v1/ruleset/001-base.yaml": `
version: v1
options:
  case_mode: upper
`,
		"yamdc-script-v1/ruleset/002-matchers.yaml": `
version: v1
matchers:
  - name: generic
    pattern: '(?i)\b([A-Z]{2,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 80
`,
	})
	fail := false
	manager, err := NewBundleManager(dataDir, stubHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		if fail {
			return nil, fmt.Errorf("network down")
		}
		switch {
		case req.URL.Host == "api.github.com" && req.URL.Path == "/repos/xxxsen/yamdc-script/tags":
			return newHTTPResponse(http.StatusOK, []byte(`[{"name":"v2026.03.31"}]`), "application/json"), nil
		case req.URL.Host == "codeload.github.com" && req.URL.Path == "/xxxsen/yamdc-script/zip/refs/tags/v2026.03.31":
			return newHTTPResponse(http.StatusOK, archive, "application/zip"), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
		}
	}}, SourceTypeRemote, "https://github.com/xxxsen/yamdc-script")
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "remote-rules"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "remote-rules", "xxxsen-yamdc-script.zip.temp"), []byte("stale"), 0644))

	rs, files, err := manager.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v1", rs.Version)
	require.Equal(t, []string{"yamdc-script-v1/ruleset/001-base.yaml", "yamdc-script-v1/ruleset/002-matchers.yaml"}, files)
	require.FileExists(t, filepath.Join(dataDir, "remote-rules", "xxxsen-yamdc-script.zip"))
	require.NoFileExists(t, filepath.Join(dataDir, "remote-rules", "xxxsen-yamdc-script.zip.temp"))

	fail = true
	rs, files, err = manager.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v1", rs.Version)
	require.Equal(t, []string{"yamdc-script-v1/ruleset/001-base.yaml", "yamdc-script-v1/ruleset/002-matchers.yaml"}, files)
}

func TestLocalBundleManagerLoad(t *testing.T) {
	ruleDir := filepath.Join(t.TempDir(), "rules")
	require.NoError(t, os.MkdirAll(ruleDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(ruleDir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0644))

	manager, err := NewBundleManager(t.TempDir(), stubHTTPClient{}, SourceTypeLocal, ruleDir)
	require.NoError(t, err)

	rs, files, err := manager.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v1", rs.Version)
	require.Equal(t, []string{"001-base.yaml"}, files)
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

func buildTestBundleZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}
