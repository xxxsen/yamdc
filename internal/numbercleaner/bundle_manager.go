package numbercleaner

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
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

	defaultRemoteEntry        = "ruleset"
	defaultRemoteSyncInterval = 24 * time.Hour
)

type BundleManager interface {
	Load(context.Context) (*RuleSet, []string, error)
	StartWatch(context.Context, func(*RuleSet, []string))
}

type BundleManifest struct {
	Entry string `yaml:"entry"`
}

type localBundleManager struct {
	dir string
}

type remoteBundleManager struct {
	cli      client.IHTTPClient
	repo     githubRepo
	cacheDir string
	zipPath  string
	tempPath string
}

type githubRepo struct {
	owner string
	repo  string
}

func NewBundleManager(dataDir string, cli client.IHTTPClient, sourceType string, location string) (BundleManager, error) {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "", SourceTypeLocal:
		return &localBundleManager{dir: strings.TrimSpace(location)}, nil
	case SourceTypeRemote:
		repo, ok := parseGitHubRepoURL(location)
		if !ok {
			return nil, fmt.Errorf("invalid remote number cleaner location: %s", location)
		}
		cacheDir := filepath.Join(dataDir, "remote-rules")
		filename := fmt.Sprintf("%s-%s.zip", repo.owner, repo.repo)
		return &remoteBundleManager{
			cli:      cli,
			repo:     *repo,
			cacheDir: cacheDir,
			zipPath:  filepath.Join(cacheDir, filename),
			tempPath: filepath.Join(cacheDir, filename+".temp"),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported rule source type: %s", sourceType)
	}
}

func (m *localBundleManager) Load(_ context.Context) (*RuleSet, []string, error) {
	dir, err := filepath.Abs(strings.TrimSpace(m.dir))
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("local rule path must be a directory: %s", dir)
	}
	rs, err := LoadRuleSetFromDir(dir)
	if err != nil {
		return nil, nil, err
	}
	files, err := ListRuleSetFilesFromDir(dir)
	if err != nil {
		return nil, nil, err
	}
	return rs, files, nil
}

func (m *localBundleManager) StartWatch(_ context.Context, _ func(*RuleSet, []string)) {}

func (m *remoteBundleManager) Load(ctx context.Context) (*RuleSet, []string, error) {
	if err := m.cleanupTemp(); err != nil {
		return nil, nil, err
	}
	if _, err := m.sync(ctx); err != nil {
		if _, statErr := os.Stat(m.zipPath); statErr != nil {
			return nil, nil, fmt.Errorf("sync remote number cleaner bundle failed: %w", err)
		}
	}
	return LoadRuleSetFromZip(m.zipPath)
}

func (m *remoteBundleManager) StartWatch(ctx context.Context, onUpdate func(*RuleSet, []string)) {
	if onUpdate == nil {
		return
	}
	ticker := time.NewTicker(defaultRemoteSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updated, err := m.sync(ctx)
			if err != nil || !updated {
				continue
			}
			rs, files, err := LoadRuleSetFromZip(m.zipPath)
			if err != nil {
				continue
			}
			onUpdate(rs, files)
		}
	}
}

func (m *remoteBundleManager) sync(ctx context.Context) (bool, error) {
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return false, err
	}
	if err := m.cleanupTemp(); err != nil {
		return false, err
	}
	tag, err := m.fetchLatestGitHubTag(ctx)
	if err != nil {
		return false, err
	}
	downloadURL := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/tags/%s", m.repo.owner, m.repo.repo, url.PathEscape(tag))
	raw, err := m.downloadBundle(ctx, downloadURL)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(m.tempPath, raw, 0644); err != nil {
		return false, err
	}
	validated := false
	defer func() {
		if !validated {
			_ = os.Remove(m.tempPath)
		}
	}()
	if _, _, err := LoadRuleSetFromZip(m.tempPath); err != nil {
		return false, fmt.Errorf("validate remote number cleaner bundle failed: %w", err)
	}
	if exists, err := fileExists(m.zipPath); err != nil {
		return false, err
	} else if exists {
		same, err := filesEqual(m.zipPath, m.tempPath)
		if err != nil {
			return false, err
		}
		if same {
			validated = true
			if err := os.Remove(m.tempPath); err != nil && !os.IsNotExist(err) {
				return false, err
			}
			return false, nil
		}
	}
	if err := os.Rename(m.tempPath, m.zipPath); err != nil {
		return false, err
	}
	validated = true
	return true, nil
}

func (m *remoteBundleManager) cleanupTemp() error {
	if err := os.Remove(m.tempPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *remoteBundleManager) downloadBundle(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode != http.StatusOK {
		defer rsp.Body.Close()
		return nil, fmt.Errorf("download number cleaner bundle failed, status:%d", rsp.StatusCode)
	}
	return client.ReadHTTPData(rsp)
}

func (m *remoteBundleManager) fetchLatestGitHubTag(ctx context.Context) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=1", m.repo.owner, m.repo.repo)
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
		return "", fmt.Errorf("no github tags found for repo: %s/%s", m.repo.owner, m.repo.repo)
	}
	return tags[0].Name, nil
}

func LoadRuleSetFromZip(zipPath string) (*RuleSet, []string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	entry, err := resolveZipRuleSetEntry(&reader.Reader)
	if err != nil {
		return nil, nil, err
	}
	rs, err := LoadRuleSetFromFS(&reader.Reader, entry)
	if err != nil {
		return nil, nil, err
	}
	files, err := ListRuleSetFilesFromFS(&reader.Reader, entry)
	if err != nil {
		return nil, nil, err
	}
	return rs, files, nil
}

func resolveZipRuleSetEntry(reader *zip.Reader) (string, error) {
	root := detectZipRoot(reader.File)
	entry := defaultRemoteEntry
	if manifest, ok, err := readZipManifest(reader, root); err != nil {
		return "", err
	} else if ok {
		if strings.TrimSpace(manifest.Entry) == "" {
			return "", fmt.Errorf("bundle manifest entry is required")
		}
		entry = manifest.Entry
	}
	clean, err := cleanBundleEntry(entry)
	if err != nil {
		return "", err
	}
	if root == "" {
		return clean, nil
	}
	return path.Join(root, clean), nil
}

func detectZipRoot(files []*zip.File) string {
	root := ""
	for _, file := range files {
		name := strings.TrimSpace(file.Name)
		if name == "" {
			continue
		}
		clean := path.Clean(strings.TrimPrefix(name, "/"))
		if clean == "." || clean == "" {
			continue
		}
		parts := strings.Split(clean, "/")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		if root == "" {
			root = parts[0]
			continue
		}
		if root != parts[0] {
			return ""
		}
	}
	return root
}

func readZipManifest(reader *zip.Reader, root string) (*BundleManifest, bool, error) {
	candidates := []string{"manifest.yaml", "manifest.yml"}
	for _, name := range candidates {
		target := name
		if root != "" {
			target = path.Join(root, name)
		}
		raw, err := fs.ReadFile(reader, target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, err
		}
		manifest := &BundleManifest{}
		if err := yaml.Unmarshal(raw, manifest); err != nil {
			return nil, false, err
		}
		return manifest, true, nil
	}
	return nil, false, nil
}

func cleanBundleEntry(raw string) (string, error) {
	clean := path.Clean(strings.TrimSpace(raw))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid bundle manifest entry: %s", raw)
	}
	return clean, nil
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

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func filesEqual(left string, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, err
	}
	if leftInfo.Size() != rightInfo.Size() {
		return false, nil
	}
	leftData, err := os.ReadFile(left)
	if err != nil {
		return false, err
	}
	rightData, err := os.ReadFile(right)
	if err != nil {
		return false, err
	}
	if len(leftData) != len(rightData) {
		return false, nil
	}
	for i := range leftData {
		if leftData[i] != rightData[i] {
			return false, nil
		}
	}
	return true, nil
}
