package bundle

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/client"
)

type OnDataReadyFunc func(context.Context, *ResolvedBundle, []string) error

type Source struct {
	SourceType string
	Location   string
}

type Manager struct {
	name        string
	cb          OnDataReadyFunc
	managers    []*basebundle.Manager
	bundles     map[int]*Bundle
	initialized bool
	mu          sync.Mutex
}

func NewManager(name string, dataDir string, cli client.IHTTPClient, sources []Source, cb OnDataReadyFunc) (*Manager, error) {
	if cb == nil {
		return nil, fmt.Errorf("plugin bundle callback is required")
	}
	out := &Manager{
		name:    name,
		cb:      cb,
		bundles: make(map[int]*Bundle, len(sources)),
	}
	managers := make([]*basebundle.Manager, 0, len(sources))
	for index, source := range sources {
		sourceIndex := index
		manager, err := basebundle.NewManager(name, dataDir, cli, source.SourceType, source.Location, "remote-plugins", func(ctx context.Context, data *basebundle.BundleData) error {
			bundle, err := LoadBundleFromData(data, sourceIndex)
			if err != nil {
				return err
			}
			out.mu.Lock()
			out.bundles[sourceIndex] = bundle
			initialized := out.initialized
			out.mu.Unlock()
			if !initialized {
				return nil
			}
			return out.emit(ctx)
		})
		if err != nil {
			return nil, err
		}
		managers = append(managers, manager)
	}
	out.managers = managers
	return out, nil
}

func (m *Manager) Start(ctx context.Context) error {
	for _, manager := range m.managers {
		if err := manager.Start(ctx); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.initialized = true
	m.mu.Unlock()
	return m.emit(ctx)
}

func (m *Manager) emit(ctx context.Context) error {
	m.mu.Lock()
	bundles := make([]*Bundle, 0, len(m.bundles))
	for _, bundle := range m.bundles {
		bundles = append(bundles, bundle)
	}
	m.mu.Unlock()
	sort.SliceStable(bundles, func(i, j int) bool {
		return bundles[i].Order < bundles[j].Order
	})
	resolved, err := resolveBundles(bundles)
	if err != nil {
		return err
	}
	return m.cb(ctx, resolved, append([]string(nil), resolved.Files...))
}

func LoadResolvedBundleFromData(data *basebundle.BundleData) (*ResolvedBundle, []string, error) {
	bundle, err := LoadBundleFromData(data, 0)
	if err != nil {
		return nil, nil, err
	}
	resolved, err := resolveBundles([]*Bundle{bundle})
	if err != nil {
		return nil, nil, err
	}
	return resolved, append([]string(nil), resolved.Files...), nil
}

func resolveBundles(bundles []*Bundle) (*ResolvedBundle, error) {
	out := &ResolvedBundle{
		Plugins:        make(map[string][]byte),
		CategoryChains: make(map[string][]string),
	}
	type pluginCandidate struct {
		name       string
		priority   int
		bundleName string
		source     string
		order      int
		data       []byte
	}
	pluginCandidates := make(map[string][]pluginCandidate)
	chainGroups := make(map[string][]ChainItem)
	for _, bundle := range bundles {
		if bundle == nil || bundle.Manifest == nil {
			continue
		}
		minPriorityByPlugin := make(map[string]int)
		for _, item := range bundle.Manifest.Configuration {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			if current, ok := minPriorityByPlugin[name]; !ok || item.Priority < current {
				minPriorityByPlugin[name] = item.Priority
			}
			cat := normalizeCategory(item.Category)
			key := cat + "\x00" + name
			chainGroups[key] = append(chainGroups[key], ChainItem{
				Name:         name,
				Category:     cat,
				Priority:     item.Priority,
				BundleName:   bundle.Manifest.Name,
				BundleSource: bundle.Source,
				Order:        bundle.Order,
			})
		}
		for name, priority := range minPriorityByPlugin {
			plugin := bundle.Plugins[name]
			pluginCandidates[name] = append(pluginCandidates[name], pluginCandidate{
				name:       name,
				priority:   priority,
				bundleName: bundle.Manifest.Name,
				source:     bundle.Source,
				order:      bundle.Order,
				data:       plugin.Data,
			})
		}
		out.Files = append(out.Files, bundle.Files...)
	}
	for name, candidates := range pluginCandidates {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].priority != candidates[j].priority {
				return candidates[i].priority < candidates[j].priority
			}
			if candidates[i].name != candidates[j].name {
				return candidates[i].name < candidates[j].name
			}
			return candidates[i].order < candidates[j].order
		})
		out.Plugins[name] = candidates[0].data
		for i := 1; i < len(candidates); i++ {
			if candidates[i].priority == candidates[0].priority {
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"plugin %s from bundle %s ignored because bundle %s already wins at priority %d",
					name, candidates[i].bundleName, candidates[0].bundleName, candidates[0].priority,
				))
			}
		}
	}
	allItems := make([]ChainItem, 0, len(chainGroups))
	categoryItems := make(map[string][]ChainItem)
	for _, items := range chainGroups {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Priority != items[j].Priority {
				return items[i].Priority < items[j].Priority
			}
			if items[i].Name != items[j].Name {
				return items[i].Name < items[j].Name
			}
			return items[i].Order < items[j].Order
		})
		winner := items[0]
		if winner.Category == allCategory {
			allItems = append(allItems, winner)
		} else {
			categoryItems[winner.Category] = append(categoryItems[winner.Category], winner)
		}
		for i := 1; i < len(items); i++ {
			if items[i].Priority == winner.Priority {
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"plugin %s in category %s from bundle %s ignored because bundle %s already wins at priority %d",
					items[i].Name, items[i].Category, items[i].BundleName, winner.BundleName, winner.Priority,
				))
			}
		}
	}
	sortChainItems(allItems)
	out.DefaultPlugins = chainItemNames(allItems)
	for category, items := range categoryItems {
		sortChainItems(items)
		names := chainItemNames(items)
		if len(out.DefaultPlugins) != 0 {
			seen := make(map[string]struct{}, len(names))
			for _, name := range names {
				seen[name] = struct{}{}
			}
			for _, name := range out.DefaultPlugins {
				if _, ok := seen[name]; ok {
					continue
				}
				names = append(names, name)
			}
		}
		out.CategoryChains[category] = names
	}
	sort.Strings(out.Files)
	return out, nil
}

func sortChainItems(items []ChainItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Order < items[j].Order
	})
}

func chainItemNames(items []ChainItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}
