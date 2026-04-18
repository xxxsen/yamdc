package bundle

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/xxxsen/yamdc/internal/cronscheduler"
)

const (
	SourceTypeLocal  = "local"
	SourceTypeRemote = "remote"

	defaultRemoteSyncInterval = 24 * time.Hour
)

var (
	errBundleCallbackRequired   = errors.New("bundle manager callback is required")
	errInvalidRemoteLocation    = errors.New("invalid remote bundle location")
	errUnsupportedSourceType    = errors.New("unsupported bundle source type")
	errDownloadBundleFailed     = errors.New("download bundle failed")
	errQueryGitHubTagFailed     = errors.New("query latest github tag failed")
	errNoGitHubTags             = errors.New("no github tags found for repo")
	errLocalBundleNotADirectory = errors.New("local bundle path must be a directory")
)

type OnDataReadyFunc func(context.Context, *Data) error

type Data struct {
	Source string
	FS     fs.FS
	Base   string
	Files  []string
	close  func() error
}

func (d *Data) Close() error {
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

func NewManager(
	name,
	dataDir string,
	cli client.IHTTPClient,
	sourceType,
	location,
	cacheSubDir string,
	cb OnDataReadyFunc,
) (*Manager, error) {
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	if sourceType == "" {
		sourceType = SourceTypeLocal
	}
	if cb == nil {
		return nil, errBundleCallbackRequired
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
			return nil, fmt.Errorf("invalid remote bundle location: %s: %w", location, errInvalidRemoteLocation)
		}
		filename := fmt.Sprintf("%s-%s.zip", repo.owner, repo.repo)
		m.cacheDir = filepath.Join(dataDir, cacheSubDir)
		m.zipPath = filepath.Join(m.cacheDir, filename)
		m.tempPath = filepath.Join(m.cacheDir, filename+".temp")
		return m, nil
	default:
		return nil, fmt.Errorf("unsupported bundle source type: %s: %w", sourceType, errUnsupportedSourceType)
	}
}

// Start 负责 Manager 的一次性初始化: local 直接读目录回调, remote 拉一次
// 并激活 zip。周期性 remote 同步不再由 Manager 自己起 goroutine 负责, 而
// 是由调用方拿 RemoteSyncJob 去注册到全局 cronscheduler — 这样所有进程
// 级定时任务的日志/生命周期/panic 兜底走同一套, 而不是各家自写一份。
func (m *Manager) Start(ctx context.Context) error {
	switch m.sourceType {
	case SourceTypeLocal:
		data, err := openLocalData(m.location)
		if err != nil {
			return err
		}
		defer func() {
			_ = data.Close()
		}()
		return m.cb(ctx, data)
	case SourceTypeRemote:
		return m.startRemote(ctx)
	default:
		return fmt.Errorf("unsupported bundle source type: %s: %w", m.sourceType, errUnsupportedSourceType)
	}
}

// RemoteSyncJob 返回一个包装 syncAndActivate 的 cron job, 用于按 syncInterval
// 做周期性远程同步。对 Local 类型返回 nil, 调用方需要自己判空 — local 没有
// 周期同步这回事, 让 caller 显式跳过比让 job 里再静默 skip 一次更清晰。
//
// namePrefix 由调用方传: 同一进程里可能既有 searcher_plugin bundle, 又有
// movieid cleaner bundle, 子 manager 的 name 不保证全局唯一 (配置文件里
// 作者可以随便起), 由上层补稳定前缀形成 "prefix_name_remote_sync" 的全局
// 唯一 job name, 直接喂给 cronscheduler.Register 不会撞车。
func (m *Manager) RemoteSyncJob(namePrefix string) cronscheduler.Job {
	if m == nil || m.sourceType != SourceTypeRemote {
		return nil
	}
	name := fmt.Sprintf("%s_%s_remote_sync", namePrefix, m.name)
	return cronscheduler.NewFuncJob(
		name,
		"@every "+m.syncInterval.String(),
		func(ctx context.Context) error {
			if _, err := m.syncAndActivate(ctx); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return nil
		},
	)
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
		data, openErr := openZipData(m.zipPath)
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
	data, err := openZipData(m.zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = data.Close()
	}()
	return m.cb(ctx, data)
}

func (m *Manager) syncAndActivate(ctx context.Context) (bool, error) {
	if err := os.MkdirAll(m.cacheDir, 0o755); err != nil {
		return false, fmt.Errorf("create bundle cache dir: %w", err)
	}
	if err := m.cleanupTemp(); err != nil {
		return false, err
	}
	tag, err := m.fetchLatestGitHubTag(ctx)
	if err != nil {
		return false, err
	}
	downloadURL := fmt.Sprintf(
		"https://codeload.github.com/%s/%s/zip/refs/tags/%s",
		parseRepoOrPanic(m.location).owner,
		parseRepoOrPanic(m.location).repo,
		url.PathEscape(tag),
	)
	raw, err := m.downloadBundle(ctx, downloadURL)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(m.tempPath, raw, 0o600); err != nil {
		return false, fmt.Errorf("write bundle temp file: %w", err)
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
	data, err := openZipData(m.tempPath)
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
		return false, fmt.Errorf("rename bundle temp to final: %w", err)
	}
	keepTemp = true
	return true, nil
}

func (m *Manager) cleanupTemp() error {
	if err := os.Remove(m.tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove bundle temp file: %w", err)
	}
	return nil
}

func (m *Manager) downloadBundle(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create bundle download request: %w", err)
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute bundle download request: %w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		defer func() {
			_ = rsp.Body.Close()
		}()
		return nil, fmt.Errorf("download bundle failed, status:%d: %w", rsp.StatusCode, errDownloadBundleFailed)
	}
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read bundle download response: %w", err)
	}
	return data, nil
}

func (m *Manager) fetchLatestGitHubTag(ctx context.Context) (string, error) {
	repo := parseRepoOrPanic(m.location)
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=1", repo.owner, repo.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("create github tags request: %w", err)
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute github tags request: %w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query latest github tag failed, status:%d: %w", rsp.StatusCode, errQueryGitHubTagFailed)
	}
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return "", fmt.Errorf("read github tags response: %w", err)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &tags); err != nil {
		return "", fmt.Errorf("unmarshal github tags: %w", err)
	}
	if len(tags) == 0 || strings.TrimSpace(tags[0].Name) == "" {
		return "", fmt.Errorf("no github tags found for repo: %s/%s: %w", repo.owner, repo.repo, errNoGitHubTags)
	}
	return tags[0].Name, nil
}

func openLocalData(dir string) (*Data, error) {
	absDir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return nil, fmt.Errorf("resolve absolute bundle path: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("stat local bundle dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local bundle path must be a directory: %s: %w", absDir, errLocalBundleNotADirectory)
	}
	files, err := listFilesFromFS(os.DirFS(absDir), ".")
	if err != nil {
		return nil, err
	}
	return &Data{
		Source: absDir,
		FS:     os.DirFS(absDir),
		Base:   ".",
		Files:  files,
	}, nil
}

func openZipData(zipPath string) (*Data, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open bundle zip: %w", err)
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
	return &Data{
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
		return nil, fmt.Errorf("walk bundle filesystem: %w", err)
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
	return false, fmt.Errorf("stat file: %w", err)
}

func filesEqual(left, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, fmt.Errorf("stat left file: %w", err)
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, fmt.Errorf("stat right file: %w", err)
	}
	if leftInfo.Size() != rightInfo.Size() {
		return false, nil
	}
	leftData, err := os.ReadFile(left)
	if err != nil {
		return false, fmt.Errorf("read left file: %w", err)
	}
	rightData, err := os.ReadFile(right)
	if err != nil {
		return false, fmt.Errorf("read right file: %w", err)
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
