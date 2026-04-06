package bundle

import (
	"archive/zip"
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

func LoadBundleFromDir(dir string) (*Bundle, []string, error) {
	absDir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("local plugin bundle path must be a directory: %s", absDir)
	}
	bundle, err := loadBundleFromFS(os.DirFS(absDir), ".", absDir, 0)
	if err != nil {
		return nil, nil, err
	}
	return bundle, append([]string(nil), bundle.Files...), nil
}

func LoadBundleFromData(data *basebundle.BundleData, order int) (*Bundle, error) {
	if data == nil {
		return nil, fmt.Errorf("bundle data is required")
	}
	return loadBundleFromFS(data.FS, data.Base, data.Source, order)
}

func LoadBundleFromZip(zipPath string, order int) (*Bundle, []string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, err
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

func loadBundleFromFS(fsys fs.FS, base string, source string, order int) (*Bundle, error) {
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
		return nil, fmt.Errorf("no plugin files found under entry: %s", entry)
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

func readManifestFromFS(fsys fs.FS, base string) (*BundleManifest, error) {
	candidates := []string{"manifest.yaml", "manifest.yml"}
	for _, name := range candidates {
		target := joinBundlePath(base, name)
		raw, err := fs.ReadFile(fsys, target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		manifest := &BundleManifest{}
		if err := yaml.Unmarshal(raw, manifest); err != nil {
			return nil, err
		}
		return manifest, nil
	}
	return nil, fmt.Errorf("bundle manifest not found")
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
			return err
		}
		if _, err := pluginyaml.NewFromBytes(raw); err != nil {
			return fmt.Errorf("validate plugin file %s failed, err:%w", name, err)
		}
		pluginName, err := decodePluginName(raw)
		if err != nil {
			return fmt.Errorf("decode plugin name from %s failed, err:%w", name, err)
		}
		if _, ok := plugins[pluginName]; ok {
			return fmt.Errorf("duplicate plugin name in bundle: %s", pluginName)
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
		return nil, nil, err
	}
	sort.Strings(files)
	return plugins, files, nil
}

func validateBundlePlugins(manifest *BundleManifest, plugins map[string]*PluginFile) error {
	for _, items := range manifest.Chains {
		for _, item := range items {
			if _, ok := plugins[strings.TrimSpace(item.Name)]; !ok {
				return fmt.Errorf("bundle manifest references unknown plugin: %s", item.Name)
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
		return "", fmt.Errorf("invalid bundle manifest entry: %s", raw)
	}
	return clean, nil
}

func joinBundlePath(base string, elem string) string {
	if strings.TrimSpace(base) == "" || base == "." {
		return path.Clean(strings.TrimPrefix(elem, "/"))
	}
	return path.Join(base, elem)
}
