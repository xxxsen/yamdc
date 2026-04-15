package bundle

import (
	"errors"
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

var (
	errManifestRequired           = errors.New("bundle manifest is required")
	errUnsupportedManifestVersion = errors.New("unsupported bundle manifest version")
	errManifestNameRequired       = errors.New("bundle manifest name is required")
	errManifestEntryRequired      = errors.New("bundle manifest entry is required")
	errManifestChainItemRequired  = errors.New("bundle manifest chain item is required")
	errManifestChainNameRequired  = errors.New("bundle manifest chain plugin name is required")
	errManifestPriorityOutOfRange = errors.New("bundle manifest plugin priority out of range")
	errDuplicateManifestChainItem = errors.New("duplicate bundle manifest chain item")
	errPluginNameRequired         = errors.New("plugin name is required")
)

type Manifest struct {
	Version       int                           `yaml:"version"`
	Name          string                        `yaml:"name"`
	Desc          string                        `yaml:"desc"`
	BundleVersion string                        `yaml:"bundle_version"`
	Entry         string                        `yaml:"entry"`
	Chains        map[string][]*PluginChainItem `yaml:"chains"`
}

type PluginChainItem struct {
	Name     string `yaml:"name"`
	Priority int    `yaml:"priority"`
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
	Manifest *Manifest
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

func runtimePluginKey(category, name string) string {
	cat := normalizeCategory(category)
	if cat == allCategory {
		return name
	}
	return "__bundle__" + cat + "__" + name
}

func normalizeCategory(raw string) string {
	cat := strings.TrimSpace(raw)
	if cat == "" || strings.EqualFold(cat, allCategory) {
		return allCategory
	}
	return strings.ToUpper(cat)
}

func validateManifest(manifest *Manifest) error {
	if manifest == nil {
		return errManifestRequired
	}
	if manifest.Version != 1 {
		return fmt.Errorf("unsupported bundle manifest version: %d: %w", manifest.Version, errUnsupportedManifestVersion)
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return errManifestNameRequired
	}
	if strings.TrimSpace(manifest.Entry) == "" {
		return errManifestEntryRequired
	}
	seen := make(map[string]struct{})
	for rawChain, items := range manifest.Chains {
		chain := normalizeCategory(rawChain)
		for _, item := range items {
			if item == nil {
				return errManifestChainItemRequired
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				return errManifestChainNameRequired
			}
			if item.Priority < 1 || item.Priority > 1000 {
				return fmt.Errorf(
					"bundle manifest plugin priority out of range: %d: %w",
					item.Priority,
					errManifestPriorityOutOfRange,
				)
			}
			key := chain + "\x00" + name
			if _, ok := seen[key]; ok {
				return fmt.Errorf(
					"duplicate bundle manifest chain item: chain=%s, name=%s: %w",
					chain, name, errDuplicateManifestChainItem,
				)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func decodePluginName(data []byte) (string, error) {
	var head pluginHeader
	if err := yaml.Unmarshal(data, &head); err != nil {
		return "", fmt.Errorf("unmarshal plugin header: %w", err)
	}
	name := strings.TrimSpace(head.Name)
	if name == "" {
		return "", errPluginNameRequired
	}
	return name, nil
}
