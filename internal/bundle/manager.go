package bundle

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
	"sort"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
)

const (
	SourceTypeLocal  = "local"
	SourceTypeRemote = "remote"

	defaultRemoteSyncInterval = 24 * time.Hour
)

type OnDataReadyFunc func(context.Context, *BundleData) error

type BundleData struct {
	Source string
	FS     fs.FS
	Base   string
	Files  []string
	close  func() error
}

func (d *BundleData) Close() error {
	if d == nil || d.close == nil {
		return nil
	}
	return d.close()
}

type Manager struct {
	name         string
	sourceType   string
	location     string
	cb           OnDataReadyFunc
	cli          client.IHTTPClient
	cacheDir     string
	zipPath      string
	tempPath     string
	syncInterval time.Duration
}

type githubRepo struct {
	owner string
	repo  string
}

func NewManager(name string, dataDir string, cli client.IHTTPClient, sourceType string, location string, cacheSubDir string, cb OnDataReadyFunc) (*Manager, error) {
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	if sourceType == "" {
		sourceType = SourceTypeLocal
	}
	if cb == nil {
		return nil, fmt.Errorf("bundle manager callback is required")
	}
	m := &Manager{
		name:         strings.TrimSpace(name),
		sourceType:   sourceType,
		location:     strings.TrimSpace(location),
		cb:           cb,
		cli:          cli,
		syncInterval: defaultRemoteSyncInterval,
	}
	switch sourceType {
	case SourceTypeLocal:
		return m, nil
	case SourceTypeRemote:
		repo, ok := parseGitHubRepoURL(m.location)
		if !ok {
			return nil, fmt.Errorf("invalid remote bundle location: %s", location)
		}
		filename := fmt.Sprintf("%s-%s.zip", repo.owner, repo.repo)
		m.cacheDir = filepath.Join(dataDir, cacheSubDir)
		m.zipPath = filepath.Join(m.cacheDir, filename)
		m.tempPath = filepath.Join(m.cacheDir, filename+".temp")
		return m, nil
	default:
		return nil, fmt.Errorf("unsupported bundle source type: %s", sourceType)
	}
}

func (m *Manager) Start(ctx context.Context) error {
	switch m.sourceType {
	case SourceTypeLocal:
		data, err := openLocalBundleData(m.location)
		if err != nil {
			return err
		}
		defer func() {
			_ = data.Close()
		}()
		return m.cb(ctx, data)
	case SourceTypeRemote:
		if err := m.startRemote(ctx); err != nil {
			return err
		}
		go m.watchRemote(ctx)
		return nil
	default:
		return fmt.Errorf("unsupported bundle source type: %s", m.sourceType)
	}
}

func (m *Manager) startRemote(ctx context.Context) error {
	if err := m.cleanupTemp(); err != nil {
		return err
	}
	updated, err := m.syncAndActivate(ctx)
	if err != nil {
		if _, statErr := os.Stat(m.zipPath); statErr != nil {
			return fmt.Errorf("sync remote %s bundle failed: %w", m.name, err)
		}
		data, openErr := openZipBundleData(m.zipPath)
		if openErr != nil {
			return openErr
		}
		defer func() {
			_ = data.Close()
		}()
		return m.cb(ctx, data)
	}
	if updated {
		return nil
	}
	data, err := openZipBundleData(m.zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = data.Close()
	}()
	return m.cb(ctx, data)
}

func (m *Manager) watchRemote(ctx context.Context) {
	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.syncAndActivate(ctx)
		}
	}
}

func (m *Manager) syncAndActivate(ctx context.Context) (bool, error) {
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
	downloadURL := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/tags/%s", parseRepoOrPanic(m.location).owner, parseRepoOrPanic(m.location).repo, url.PathEscape(tag))
	raw, err := m.downloadBundle(ctx, downloadURL)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(m.tempPath, raw, 0644); err != nil {
		return false, err
	}
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(m.tempPath)
		}
	}()
	if exists, err := fileExists(m.zipPath); err != nil {
		return false, err
	} else if exists {
		same, err := filesEqual(m.zipPath, m.tempPath)
		if err != nil {
			return false, err
		}
		if same {
			return false, nil
		}
	}
	data, err := openZipBundleData(m.tempPath)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = data.Close()
	}()
	if err := m.cb(ctx, data); err != nil {
		return false, err
	}
	if err := os.Rename(m.tempPath, m.zipPath); err != nil {
		return false, err
	}
	keepTemp = true
	return true, nil
}

func (m *Manager) cleanupTemp() error {
	if err := os.Remove(m.tempPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *Manager) downloadBundle(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode != http.StatusOK {
		defer func() {
			_ = rsp.Body.Close()
		}()
		return nil, fmt.Errorf("download bundle failed, status:%d", rsp.StatusCode)
	}
	return client.ReadHTTPData(rsp)
}

func (m *Manager) fetchLatestGitHubTag(ctx context.Context) (string, error) {
	repo := parseRepoOrPanic(m.location)
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=1", repo.owner, repo.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
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
		return "", fmt.Errorf("no github tags found for repo: %s/%s", repo.owner, repo.repo)
	}
	return tags[0].Name, nil
}

func openLocalBundleData(dir string) (*BundleData, error) {
	absDir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local bundle path must be a directory: %s", absDir)
	}
	files, err := listFilesFromFS(os.DirFS(absDir), ".")
	if err != nil {
		return nil, err
	}
	return &BundleData{
		Source: absDir,
		FS:     os.DirFS(absDir),
		Base:   ".",
		Files:  files,
	}, nil
}

func openZipBundleData(zipPath string) (*BundleData, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	root := detectZipRoot(reader.File)
	base := "."
	if root != "" {
		base = root
	}
	files, err := listFilesFromFS(&reader.Reader, base)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	return &BundleData{
		Source: zipPath,
		FS:     &reader.Reader,
		Base:   base,
		Files:  files,
		close: func() error {
			return reader.Close()
		},
	}, nil
}

func listFilesFromFS(fsys fs.FS, base string) ([]string, error) {
	files := make([]string, 0, 8)
	err := fs.WalkDir(fsys, base, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
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

func parseRepoOrPanic(raw string) githubRepo {
	repo, ok := parseGitHubRepoURL(raw)
	if !ok {
		panic("invalid github repo url")
	}
	return *repo
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
