package bundle

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SourceTypeLocal  = "local"
	SourceTypeRemote = "remote"

	defaultRemoteEntry = "plugins"
	allCategory        = "all"
)

type BundleManifest struct {
	Version       int                  `yaml:"version"`
	Name          string               `yaml:"name"`
	Desc          string               `yaml:"desc"`
	BundleVersion string               `yaml:"bundle_version"`
	Entry         string               `yaml:"entry"`
	Configuration []*PluginChainConfig `yaml:"configuration"`
}

type PluginChainConfig struct {
	Name     string `yaml:"name"`
	Priority int    `yaml:"priority"`
	Category string `yaml:"category"`
}

type pluginHeader struct {
	Name string `yaml:"name"`
}

type PluginFile struct {
	Name string
	Path string
	Data []byte
}

type Bundle struct {
	Manifest *BundleManifest
	Plugins  map[string]*PluginFile
	Files    []string
	Source   string
	Order    int
}

type ChainItem struct {
	Name         string
	Category     string
	Priority     int
	BundleName   string
	BundleSource string
	Order        int
}

type ResolvedBundle struct {
	Plugins        map[string][]byte
	DefaultPlugins []string
	CategoryChains map[string][]string
	Warnings       []string
	Files          []string
}

func normalizeCategory(raw string) string {
	cat := strings.TrimSpace(raw)
	if cat == "" {
		return allCategory
	}
	return strings.ToUpper(cat)
}

func validateManifest(manifest *BundleManifest) error {
	if manifest == nil {
		return fmt.Errorf("bundle manifest is required")
	}
	if manifest.Version != 1 {
		return fmt.Errorf("unsupported bundle manifest version: %d", manifest.Version)
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("bundle manifest name is required")
	}
	if strings.TrimSpace(manifest.Entry) == "" {
		return fmt.Errorf("bundle manifest entry is required")
	}
	seen := make(map[string]struct{}, len(manifest.Configuration))
	for _, item := range manifest.Configuration {
		if item == nil {
			return fmt.Errorf("bundle manifest configuration item is required")
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return fmt.Errorf("bundle manifest configuration plugin name is required")
		}
		if item.Priority < 1 || item.Priority > 1000 {
			return fmt.Errorf("bundle manifest plugin priority out of range: %d", item.Priority)
		}
		cat := normalizeCategory(item.Category)
		key := cat + "\x00" + name
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate bundle manifest configuration item: category=%s, name=%s", cat, name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func decodePluginName(data []byte) (string, error) {
	var head pluginHeader
	if err := yaml.Unmarshal(data, &head); err != nil {
		return "", err
	}
	name := strings.TrimSpace(head.Name)
	if name == "" {
		return "", fmt.Errorf("plugin name is required")
	}
	return name, nil
}
