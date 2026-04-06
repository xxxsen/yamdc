package numbercleaner

import (
	"archive/zip"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/client"
	"gopkg.in/yaml.v3"
)

const (
	SourceTypeLocal  = basebundle.SourceTypeLocal
	SourceTypeRemote = basebundle.SourceTypeRemote

	defaultRemoteEntry = "ruleset"
)

type OnDataReadyFunc func(context.Context, *RuleSet, []string) error

type Manager struct {
	manager *basebundle.Manager
}

type BundleManifest struct {
	Entry string `yaml:"entry"`
}

func NewManager(dataDir string, cli client.IHTTPClient, sourceType string, location string, cb OnDataReadyFunc) (*Manager, error) {
	if cb == nil {
		return nil, fmt.Errorf("number cleaner bundle callback is required")
	}
	manager, err := basebundle.NewManager("number_cleaner", dataDir, cli, sourceType, location, "remote-rules", func(ctx context.Context, data *basebundle.BundleData) error {
		rs, files, err := LoadRuleSetFromBundleData(data)
		if err != nil {
			return err
		}
		return cb(ctx, rs, files)
	})
	if err != nil {
		return nil, err
	}
	return &Manager{manager: manager}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	if m == nil || m.manager == nil {
		return fmt.Errorf("bundle manager is nil")
	}
	return m.manager.Start(ctx)
}

func LoadRuleSetFromBundleData(data *basebundle.BundleData) (*RuleSet, []string, error) {
	if data == nil {
		return nil, nil, fmt.Errorf("bundle data is required")
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
		return nil, nil, err
	}
	defer func() {
		_ = reader.Close()
	}()
	base := "."
	if root := detectZipRoot(reader.File); root != "" {
		base = root
	}
	return LoadRuleSetFromBundleData(&basebundle.BundleData{
		FS:   &reader.Reader,
		Base: base,
	})
}

func resolveBundleEntry(fsys fs.FS, base string, defaultEntry string) (string, error) {
	entry := defaultEntry
	if manifest, ok, err := readManifest(fsys, base); err != nil {
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
	if base == "" || base == "." {
		return clean, nil
	}
	return path.Join(base, clean), nil
}

func readManifest(fsys fs.FS, base string) (*BundleManifest, bool, error) {
	candidates := []string{"manifest.yaml", "manifest.yml"}
	for _, name := range candidates {
		target := name
		if base != "" && base != "." {
			target = path.Join(base, name)
		}
		raw, err := fs.ReadFile(fsys, target)
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
