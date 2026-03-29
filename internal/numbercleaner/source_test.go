package numbercleaner

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/client"
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

func TestBundleManagerRemoteSync(t *testing.T) {
	dataDir := t.TempDir()
	archive := buildTestBundleZip(t, map[string]string{
		"manifest.yaml": `name: test-rules
version: 2026.03.29
format: yamdc-ruleset-v1
entry: ruleset`,
		"ruleset/001-base.yaml": `
version: v1
options:
  case_mode: upper
`,
		"ruleset/002-normalizers.yaml": `
version: v1
normalizers:
  - name: basename
    type: builtin
    builtin: basename
  - name: strip_ext
    type: builtin
    builtin: strip_ext
  - name: to_upper
    type: builtin
    builtin: to_upper
`,
		"ruleset/003-matchers.yaml": `
version: v1
matchers:
  - name: generic
    pattern: '(?i)\b([A-Z]{2,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 80
`,
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	manager := NewBundleManager(dataDir, client.MustNewClient(), SourceTypeRemote, server.URL+"/rules.zip", "")
	rulePath, updated, err := manager.SyncRemote(context.Background())
	require.NoError(t, err)
	require.True(t, updated)
	require.DirExists(t, rulePath)

	activePath, err := manager.CurrentRulePath()
	require.NoError(t, err)
	require.Equal(t, rulePath, activePath)

	raw, err := os.ReadFile(filepath.Join(dataDir, "rule-bundles", "state.json"))
	require.NoError(t, err)
	state := &BundleState{}
	require.NoError(t, json.Unmarshal(raw, state))
	require.Equal(t, SourceTypeRemote, state.SourceType)
	require.Equal(t, "2026.03.29", state.ActiveVersion)
	require.Equal(t, rulePath, state.ActiveRulePath)
}

func TestResolveBundleEntryWithoutManifest(t *testing.T) {
	root := t.TempDir()
	rulesetDir := filepath.Join(root, "yamdc-script-v1", "ruleset")
	require.NoError(t, os.MkdirAll(rulesetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesetDir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0644))

	entry, version, err := resolveBundleEntry(root, "v2026.03.29")
	require.NoError(t, err)
	require.Equal(t, rulesetDir, entry)
	require.Equal(t, "v2026.03.29", version)
}

func TestParseGitHubRepoURL(t *testing.T) {
	repo, ok := parseGitHubRepoURL("https://github.com/xxxsen/yamdc-script")
	require.True(t, ok)
	require.Equal(t, "xxxsen", repo.owner)
	require.Equal(t, "yamdc-script", repo.repo)
}

func TestBundleManagerLocalState(t *testing.T) {
	dataDir := t.TempDir()
	ruleDir := filepath.Join(t.TempDir(), "rules")
	require.NoError(t, os.MkdirAll(ruleDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(ruleDir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0644))

	manager := NewBundleManager(dataDir, client.MustNewClient(), SourceTypeLocal, "", ruleDir)
	path, err := manager.CurrentRulePath()
	require.NoError(t, err)
	require.Equal(t, ruleDir, path)

	raw, err := os.ReadFile(filepath.Join(dataDir, "rule-bundles", "state.json"))
	require.NoError(t, err)
	state := &BundleState{}
	require.NoError(t, json.Unmarshal(raw, state))
	require.Equal(t, SourceTypeLocal, state.SourceType)
	require.Equal(t, ruleDir, state.ActiveRulePath)
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
