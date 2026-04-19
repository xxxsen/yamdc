package movieidcleaner

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/cronscheduler"
)

// cronJobPrefix 给本包的 bundle manager 产出的 remote sync job 加一个固定
// 前缀, 和 searcher plugin bundle 区分开 (Job.Name 需要全局唯一, 否则
// cronscheduler.Register 会拒掉)。前端看不到, 纯运维视角命名, 改名要同步
// 更新排障文档里的示例。
const cronJobPrefix = "movieid_cleaner"

const (
	SourceTypeLocal  = basebundle.SourceTypeLocal
	SourceTypeRemote = basebundle.SourceTypeRemote

	defaultRemoteEntry = "ruleset"
)

var (
	errCleanerCallbackRequired     = errors.New("movieid cleaner bundle callback is required")
	errBundleManagerNil            = errors.New("bundle manager is nil")
	errBundleDataRequired          = errors.New("bundle data is required")
	errBundleManifestEntryRequired = errors.New("bundle manifest entry is required")
	errInvalidBundleEntry          = errors.New("invalid bundle manifest entry")
)

type OnDataReadyFunc func(context.Context, *RuleSet, []string) error

type Manager struct {
	manager *basebundle.Manager
}

type BundleManifest struct {
	Entry string `yaml:"entry"`
}

func NewManager(
	dataDir string,
	cli client.IHTTPClient,
	sourceType,
	location string,
	cb OnDataReadyFunc,
) (*Manager, error) {
	if cb == nil {
		return nil, errCleanerCallbackRequired
	}
	manager, err := basebundle.NewManager("number_cleaner", dataDir, cli, sourceType, location, "remote-rules",
		func(ctx context.Context, data *basebundle.Data) error {
			rs, files, err := LoadRuleSetFromBundleData(data)
			if err != nil {
				return err
			}
			return cb(ctx, rs, files)
		})
	if err != nil {
		return nil, fmt.Errorf("create bundle manager failed: %w", err)
	}
	return &Manager{manager: manager}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	if m == nil || m.manager == nil {
		return errBundleManagerNil
	}
	if err := m.manager.Start(ctx); err != nil {
		return fmt.Errorf("start bundle manager failed: %w", err)
	}
	return nil
}

// CronJob 返回本 Manager 对应的 remote 周期同步 job, 给 bootstrap 注册进
// 全局 cronscheduler。Local 类型 / nil manager 返回 nil, 调用方需要判空 —
// 让 "本实例就不该有周期任务" 这件事在 API 层面明示, 而不是藏在 job 内部
// 静默 skip。
func (m *Manager) CronJob() cronscheduler.Job {
	if m == nil || m.manager == nil {
		return nil
	}
	return m.manager.RemoteSyncJob(cronJobPrefix)
}

func LoadRuleSetFromBundleData(data *basebundle.Data) (*RuleSet, []string, error) {
	if data == nil {
		return nil, nil, errBundleDataRequired
	}
	entry, err := resolveBundleEntry(data.FS, data.Base, defaultRemoteEntry)
	if err != nil {
		return nil, nil, err
	}
	rs, err := LoadRuleSetFromFS(data.FS, entry)
	if err != nil {
		return nil, nil, err
	}
	files, err := ListRuleSetFilesFromFS(data.FS, entry)
	if err != nil {
		return nil, nil, err
	}
	return rs, files, nil
}

func LoadRuleSetFromZip(zipPath string) (*RuleSet, []string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open zip %s failed: %w", zipPath, err)
	}
	defer func() {
		_ = reader.Close()
	}()
	base := "."
	if root := detectZipRoot(reader.File); root != "" {
		base = root
	}
	return LoadRuleSetFromBundleData(&basebundle.Data{
		FS:   &reader.Reader,
		Base: base,
	})
}

func resolveBundleEntry(fsys fs.FS, base, defaultEntry string) (string, error) {
	entry := defaultEntry
	if manifest, ok, err := readManifest(fsys, base); err != nil {
		return "", err
	} else if ok {
		if strings.TrimSpace(manifest.Entry) == "" {
			return "", errBundleManifestEntryRequired
		}
		entry = manifest.Entry
	}
	clean, err := cleanBundleEntry(entry)
	if err != nil {
		return "", err
	}
	if base == "" || base == "." {
		return clean, nil
	}
	return path.Join(base, clean), nil
}

func readManifest(fsys fs.FS, base string) (*BundleManifest, bool, error) {
	candidates := []string{"manifest.yaml", "manifest.yml"}
	for _, name := range candidates {
		if base != "" && base != "." {
			name = path.Join(base, name)
		}
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, fmt.Errorf("read manifest %s failed: %w", name, err)
		}
		manifest := &BundleManifest{}
		if err := yaml.Unmarshal(raw, manifest); err != nil {
			return nil, false, fmt.Errorf("unmarshal manifest %s failed: %w", name, err)
		}
		return manifest, true, nil
	}
	return nil, false, nil
}

func cleanBundleEntry(raw string) (string, error) {
	clean := path.Clean(strings.TrimSpace(raw))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid bundle manifest entry: %s: %w", raw, errInvalidBundleEntry)
	}
	return clean, nil
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
