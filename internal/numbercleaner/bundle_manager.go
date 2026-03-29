package numbercleaner

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
	"gopkg.in/yaml.v3"
)

const (
	SourceTypeLocal  = "local"
	SourceTypeRemote = "remote"
)

type BundleState struct {
	SourceType     string `json:"source_type"`
	SourceID       string `json:"source_id"`
	ActiveVersion  string `json:"active_version"`
	ActiveRulePath string `json:"active_rule_path"`
	UpdatedAt      int64  `json:"updated_at"`
}

type BundleManifest struct {
	Name             string `yaml:"name"`
	Version          string `yaml:"version"`
	MinEngineVersion int    `yaml:"min_engine_version"`
	Format           string `yaml:"format"`
	Entry            string `yaml:"entry"`
}

type BundleManager struct {
	dataDir    string
	sourceType string
	sourceID   string
	remoteURL  string
	localPath  string
	cli        client.IHTTPClient
}

type remoteBundleMeta struct {
	downloadURL string
	version     string
}

func NewBundleManager(dataDir string, cli client.IHTTPClient, sourceType string, remoteURL string, localPath string) *BundleManager {
	manager := &BundleManager{
		dataDir:    dataDir,
		sourceType: strings.ToLower(strings.TrimSpace(sourceType)),
		remoteURL:  strings.TrimSpace(remoteURL),
		localPath:  strings.TrimSpace(localPath),
		cli:        cli,
	}
	switch manager.sourceType {
	case SourceTypeLocal:
		manager.sourceID = normalizeSourceID(SourceTypeLocal, localPath)
	case SourceTypeRemote:
		manager.sourceID = normalizeSourceID(SourceTypeRemote, remoteURL)
	}
	return manager
}

func (m *BundleManager) CurrentRulePath() (string, error) {
	switch m.sourceType {
	case SourceTypeLocal:
		if strings.TrimSpace(m.localPath) == "" {
			return "", fmt.Errorf("local bundle path is empty")
		}
		path, err := filepath.Abs(m.localPath)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() && !isYAMLPath(path) {
			return "", fmt.Errorf("local rule path must be a yaml file or directory: %s", path)
		}
		if err := m.writeState(&BundleState{
			SourceType:     SourceTypeLocal,
			SourceID:       m.sourceID,
			ActiveVersion:  "local",
			ActiveRulePath: path,
			UpdatedAt:      time.Now().Unix(),
		}); err != nil {
			return "", err
		}
		return path, nil
	case SourceTypeRemote:
		state, err := m.readState()
		if err != nil {
			return "", err
		}
		if state == nil {
			return "", fmt.Errorf("no active remote rule bundle found")
		}
		if state.SourceType != SourceTypeRemote || state.SourceID != m.sourceID {
			return "", fmt.Errorf("active rule bundle source mismatch")
		}
		if strings.TrimSpace(state.ActiveRulePath) == "" {
			return "", fmt.Errorf("active remote rule path is empty")
		}
		if _, err := os.Stat(state.ActiveRulePath); err != nil {
			return "", err
		}
		return state.ActiveRulePath, nil
	default:
		return "", fmt.Errorf("unsupported rule source type: %s", m.sourceType)
	}
}

func (m *BundleManager) SyncRemote(ctx context.Context) (string, bool, error) {
	if m.sourceType != SourceTypeRemote {
		return "", false, nil
	}
	if strings.TrimSpace(m.remoteURL) == "" {
		return "", false, fmt.Errorf("remote bundle url is empty")
	}
	meta, err := m.resolveRemoteBundleMeta(ctx)
	if err != nil {
		return "", false, err
	}
	raw, err := m.downloadBundle(ctx, meta.downloadURL)
	if err != nil {
		return "", false, err
	}
	root := m.bundleRootDir()
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", false, err
	}
	tempDir, err := os.MkdirTemp(root, "bundle-*")
	if err != nil {
		return "", false, err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	if err := extractBundleArchive(raw, tempDir); err != nil {
		return "", false, err
	}
	entryPath, version, err := resolveBundleEntry(tempDir, meta.version)
	if err != nil {
		return "", false, err
	}
	if _, err := LoadRuleSetFromPath(entryPath); err != nil {
		return "", false, fmt.Errorf("validate bundle rule set failed: %w", err)
	}
	versionDir := filepath.Join(root, "versions", sanitizeVersion(version))
	entryRel, err := filepath.Rel(tempDir, entryPath)
	if err != nil {
		return "", false, err
	}
	activeRulePath := filepath.Join(versionDir, entryRel)
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(versionDir), 0755); err != nil {
			return "", false, err
		}
		if err := os.Rename(tempDir, versionDir); err != nil {
			return "", false, err
		}
		tempDir = ""
	}
	prev, err := m.readState()
	if err != nil {
		return "", false, err
	}
	updated := prev == nil || prev.SourceType != SourceTypeRemote || prev.SourceID != m.sourceID || prev.ActiveVersion != version || prev.ActiveRulePath != activeRulePath
	if err := m.writeState(&BundleState{
		SourceType:     SourceTypeRemote,
		SourceID:       m.sourceID,
		ActiveVersion:  version,
		ActiveRulePath: activeRulePath,
		UpdatedAt:      time.Now().Unix(),
	}); err != nil {
		return "", false, err
	}
	return activeRulePath, updated, nil
}

func (m *BundleManager) readState() (*BundleState, error) {
	raw, err := os.ReadFile(m.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state := &BundleState{}
	if err := json.Unmarshal(raw, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (m *BundleManager) writeState(state *BundleState) error {
	if state == nil {
		return nil
	}
	if err := os.MkdirAll(m.bundleRootDir(), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp := m.statePath() + ".tmp"
	if err := os.WriteFile(temp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(temp, m.statePath())
}

func (m *BundleManager) statePath() string {
	return filepath.Join(m.bundleRootDir(), "state.json")
}

func (m *BundleManager) bundleRootDir() string {
	return filepath.Join(m.dataDir, "rule-bundles")
}

func (m *BundleManager) downloadBundle(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return nil, err
	}
	return client.ReadHTTPData(rsp)
}

func (m *BundleManager) resolveRemoteBundleMeta(ctx context.Context) (*remoteBundleMeta, error) {
	repo, ok := parseGitHubRepoURL(m.remoteURL)
	if !ok {
		return &remoteBundleMeta{
			downloadURL: m.remoteURL,
			version:     "",
		}, nil
	}
	tag, err := m.fetchLatestGitHubTag(ctx, repo.owner, repo.repo)
	if err != nil {
		return nil, err
	}
	return &remoteBundleMeta{
		downloadURL: fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/refs/tags/%s", repo.owner, repo.repo, url.PathEscape(tag)),
		version:     tag,
	}, nil
}

type githubRepo struct {
	owner string
	repo  string
}

func parseGitHubRepoURL(raw string) (*githubRepo, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, false
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return nil, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, false
	}
	return &githubRepo{owner: parts[0], repo: strings.TrimSuffix(parts[1], ".git")}, true
}

func (m *BundleManager) fetchLatestGitHubTag(ctx context.Context, owner string, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=1", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query latest github tag failed, status:%d", rsp.StatusCode)
	}
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return "", err
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &tags); err != nil {
		return "", err
	}
	if len(tags) == 0 || strings.TrimSpace(tags[0].Name) == "" {
		return "", fmt.Errorf("no github tags found for repo: %s/%s", owner, repo)
	}
	return tags[0].Name, nil
}

func extractBundleArchive(raw []byte, dst string) error {
	if len(raw) >= 4 && bytes.Equal(raw[:4], []byte("PK\x03\x04")) {
		return extractZipArchive(raw, dst)
	}
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		return extractTarGzArchive(raw, dst)
	}
	return fmt.Errorf("unsupported bundle archive format")
}

func extractZipArchive(raw []byte, dst string) error {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target, err := safeJoin(dst, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		if err := writeFile(target, rc, file.Mode()); err != nil {
			_ = rc.Close()
			return err
		}
		_ = rc.Close()
	}
	return nil
}

func extractTarGzArchive(raw []byte, dst string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dst, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := writeFile(target, tr, fs.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}
}

func safeJoin(root string, name string) (string, error) {
	clean := filepath.Clean(name)
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("archive entry escapes root: %s", name)
	}
	return target, nil
}

func writeFile(path string, src io.Reader, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, src)
	return err
}

func findManifestPath(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if name == "manifest.yaml" || name == "manifest.yml" {
			found = path
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	if strings.TrimSpace(found) == "" {
		return "", fmt.Errorf("bundle manifest not found")
	}
	return found, nil
}

func readManifest(path string) (*BundleManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	manifest := &BundleManifest{}
	if err := yaml.Unmarshal(raw, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func resolveBundleEntry(root string, fallbackVersion string) (string, string, error) {
	manifestPath, err := findManifestPath(root)
	if err == nil {
		manifest, err := readManifest(manifestPath)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(manifest.Version) == "" {
			return "", "", fmt.Errorf("bundle manifest version is required")
		}
		if strings.TrimSpace(manifest.Entry) == "" {
			return "", "", fmt.Errorf("bundle manifest entry is required")
		}
		entryPath := filepath.Join(filepath.Dir(manifestPath), manifest.Entry)
		if _, err := os.Stat(entryPath); err != nil {
			return "", "", fmt.Errorf("bundle entry path not found: %w", err)
		}
		return entryPath, manifest.Version, nil
	}
	rulesetPath, rulesetErr := findRulesetDir(root)
	if rulesetErr != nil {
		return "", "", err
	}
	if strings.TrimSpace(fallbackVersion) == "" {
		return "", "", fmt.Errorf("bundle version is required when manifest is absent")
	}
	return rulesetPath, fallbackVersion, nil
}

func findRulesetDir(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if pathpkg := strings.ToLower(pathpkgBase(path)); pathpkg != "ruleset" {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		hasYAML := false
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				hasYAML = true
				break
			}
		}
		if hasYAML {
			found = path
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	if strings.TrimSpace(found) == "" {
		return "", fmt.Errorf("ruleset directory not found")
	}
	return found, nil
}

func pathpkgBase(in string) string {
	return path.Base(filepath.ToSlash(in))
}

func normalizeSourceID(sourceType string, value string) string {
	switch sourceType {
	case SourceTypeLocal:
		if abs, err := filepath.Abs(strings.TrimSpace(value)); err == nil {
			return abs
		}
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(value)
	}
}

func sanitizeVersion(version string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return replacer.Replace(strings.TrimSpace(version))
}

func isYAMLPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}
