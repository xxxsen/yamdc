package bundle

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"gopkg.in/yaml.v3"
)

var (
	errNotADirectory        = errors.New("local plugin bundle path must be a directory")
	errBundleDataRequired   = errors.New("bundle data is required")
	errNoPluginFiles        = errors.New("no plugin files found under entry")
	errManifestNotFound     = errors.New("bundle manifest not found")
	errDuplicatePluginName  = errors.New("duplicate plugin name in bundle")
	errUnknownPluginRef     = errors.New("bundle manifest references unknown plugin")
	errInvalidManifestEntry = errors.New("invalid bundle manifest entry")
)

func LoadBundleFromDir(dir string) (*Bundle, []string, error) {
	absDir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve abs path: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, nil, fmt.Errorf("stat bundle dir: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("local plugin bundle path must be a directory: %s: %w", absDir, errNotADirectory)
	}
	bundle, err := loadBundleFromFS(os.DirFS(absDir), ".", absDir, 0)
	if err != nil {
		return nil, nil, err
	}
	return bundle, append([]string(nil), bundle.Files...), nil
}

func LoadBundleFromData(data *basebundle.Data, order int) (*Bundle, error) {
	if data == nil {
		return nil, errBundleDataRequired
	}
	return loadBundleFromFS(data.FS, data.Base, data.Source, order)
}

func LoadBundleFromZip(zipPath string, order int) (*Bundle, []string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() {
		_ = reader.Close()
	}()
	root := detectZipRoot(reader.File)
	base := "."
	if root != "" {
		base = root
	}
	bundle, err := loadBundleFromFS(&reader.Reader, base, zipPath, order)
	if err != nil {
		return nil, nil, err
	}
	return bundle, append([]string(nil), bundle.Files...), nil
}

func loadBundleFromFS(fsys fs.FS, base, source string, order int) (*Bundle, error) {
	manifest, err := readManifestFromFS(fsys, base)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	entry, err := cleanBundleEntry(manifest.Entry)
	if err != nil {
		return nil, err
	}
	entry = joinBundlePath(base, entry)
	plugins, files, err := loadPluginFilesFromFS(fsys, entry)
	if err != nil {
		return nil, err
	}
	if len(plugins) == 0 {
		return nil, fmt.Errorf("no plugin files found under entry: %s: %w", entry, errNoPluginFiles)
	}
	if err := validateBundlePlugins(manifest, plugins); err != nil {
		return nil, err
	}
	return &Bundle{
		Manifest: manifest,
		Plugins:  plugins,
		Files:    files,
		Source:   source,
		Order:    order,
	}, nil
}

func readManifestFromFS(fsys fs.FS, base string) (*Manifest, error) {
	candidates := []string{"manifest.yaml", "manifest.yml"}
	for _, name := range candidates {
		target := joinBundlePath(base, name)
		raw, err := fs.ReadFile(fsys, target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read manifest %s: %w", target, err)
		}
		manifest := &Manifest{}
		if err := yaml.Unmarshal(raw, manifest); err != nil {
			return nil, fmt.Errorf("unmarshal manifest %s: %w", target, err)
		}
		return manifest, nil
	}
	return nil, errManifestNotFound
}

func loadPluginFilesFromFS(fsys fs.FS, entry string) (map[string]*PluginFile, []string, error) {
	files := make([]string, 0, 8)
	plugins := make(map[string]*PluginFile)
	err := fs.WalkDir(fsys, entry, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			return nil
		}
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read plugin file %s: %w", name, err)
		}
		if _, err := pluginyaml.NewFromBytes(raw); err != nil {
			return fmt.Errorf("validate plugin file %s failed, err:%w", name, err)
		}
		pluginName, err := decodePluginName(raw)
		if err != nil {
			return fmt.Errorf("decode plugin name from %s failed, err:%w", name, err)
		}
		if _, ok := plugins[pluginName]; ok {
			return fmt.Errorf("duplicate plugin name in bundle: %s: %w", pluginName, errDuplicatePluginName)
		}
		plugins[pluginName] = &PluginFile{
			Name: pluginName,
			Path: name,
			Data: raw,
		}
		files = append(files, name)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk plugin dir %s: %w", entry, err)
	}
	sort.Strings(files)
	return plugins, files, nil
}

func validateBundlePlugins(manifest *Manifest, plugins map[string]*PluginFile) error {
	for _, items := range manifest.Chains {
		for _, item := range items {
			if _, ok := plugins[strings.TrimSpace(item.Name)]; !ok {
				return fmt.Errorf("bundle manifest references unknown plugin: %s: %w", item.Name, errUnknownPluginRef)
			}
		}
	}
	return nil
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

func cleanBundleEntry(raw string) (string, error) {
	clean := path.Clean(strings.TrimSpace(raw))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid bundle manifest entry: %s: %w", raw, errInvalidManifestEntry)
	}
	return clean, nil
}

func joinBundlePath(base, elem string) string {
	if strings.TrimSpace(base) == "" || base == "." {
		return path.Clean(strings.TrimPrefix(elem, "/"))
	}
	return path.Join(base, elem)
}
