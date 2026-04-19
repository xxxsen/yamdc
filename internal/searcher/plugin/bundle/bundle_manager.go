package bundle

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/cronscheduler"
)

var errPluginBundleCallbackRequired = errors.New("plugin bundle callback is required")

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
	emitMu      sync.Mutex
}

func NewManager(name, dataDir string, cli client.IHTTPClient, sources []Source, cb OnDataReadyFunc) (*Manager, error) {
	if cb == nil {
		return nil, errPluginBundleCallbackRequired
	}
	out := &Manager{
		name:    name,
		cb:      cb,
		bundles: make(map[int]*Bundle, len(sources)),
	}
	managers := make([]*basebundle.Manager, 0, len(sources))
	for index, source := range sources {
		// 给每个 sub-manager 一个带 index 的唯一 name。这个 name 同时决定
		// 两件事: (1) basebundle 内部报错消息的前缀 ("sync remote %s bundle
		// failed"), 加 index 排障时能一眼看出是哪一路 source 出问题; (2) 经
		// RemoteSyncJob 转成 "<prefix>_<subName>_remote_sync" 的全局 Job name,
		// cronscheduler 要求注册名全局唯一 — 如果所有 sub 都共用 name 参数,
		// 多 remote source 配置下会在启动期被 errDuplicateJobName 直接拒掉,
		// 进程起不来。
		subName := fmt.Sprintf("%s_source%d", name, index)
		manager, err := basebundle.NewManager(subName, dataDir, cli, source.SourceType, source.Location, "remote-plugins",
			func(ctx context.Context, data *basebundle.Data) error {
				// Go 1.22+ 每轮迭代独立作用域, 闭包直接捕获 index 即可
				bundle, err := LoadBundleFromData(data, index)
				if err != nil {
					return err
				}
				out.mu.Lock()
				out.bundles[index] = bundle
				initialized := out.initialized
				out.mu.Unlock()
				if !initialized {
					return nil
				}
				return out.emit(ctx)
			})
		if err != nil {
			return nil, fmt.Errorf("create bundle manager for source %d: %w", index, err)
		}
		managers = append(managers, manager)
	}
	out.managers = managers
	return out, nil
}

func (m *Manager) Start(ctx context.Context) error {
	for _, manager := range m.managers {
		if err := manager.Start(ctx); err != nil {
			return fmt.Errorf("start bundle manager: %w", err)
		}
	}
	m.mu.Lock()
	m.initialized = true
	m.mu.Unlock()
	return m.emit(ctx)
}

// CronJobs 聚合所有 remote 子 manager 的周期同步 job, 交由 bootstrap 统一注册
// 进全局 cronscheduler。同一个 pluginbundle.Manager 可能有 N 个 source (配置
// 多 bundle 源时), 每个 source 对应一个 basebundle.Manager — 调用方只需要
// "拿一组 job 全塞进 scheduler" 的接口, 不关心有几条。
//
// Job Name 形如 "searcher_plugin_searcher_plugin_source<N>_remote_sync": 外层
// 前缀 (m.name, 当前取 "searcher_plugin") 负责和 movieidcleaner 那路区分;
// 内层 subName 由 NewManager 按 sourceIndex 生成 ("<name>_source<N>") 保证
// 多 source 时每条 job 全局唯一 — 这个全局唯一性是硬要求, 重名会被
// cronscheduler.Register 直接拒掉导致启动失败。Local 类型 sub 返回 nil, 自动
// 跳过, 这里不收集。
func (m *Manager) CronJobs() []cronscheduler.Job {
	if m == nil {
		return nil
	}
	jobs := make([]cronscheduler.Job, 0, len(m.managers))
	for _, sub := range m.managers {
		if job := sub.RemoteSyncJob(m.name); job != nil {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func (m *Manager) emit(ctx context.Context) error {
	m.emitMu.Lock()
	defer m.emitMu.Unlock()
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

func LoadResolvedBundleFromData(data *basebundle.Data) (*ResolvedBundle, []string, error) {
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

func LoadResolvedBundleFromDir(dir string) (*ResolvedBundle, []string, error) {
	bundle, files, err := LoadBundleFromDir(dir)
	if err != nil {
		return nil, nil, err
	}
	resolved, err := resolveBundles([]*Bundle{bundle})
	if err != nil {
		return nil, nil, err
	}
	return resolved, files, nil
}

type pluginCandidate struct {
	name       string
	category   string
	runtimeKey string
	priority   int
	bundleName string
	source     string
	order      int
	data       []byte
}

func collectBundleCandidates(bundles []*Bundle) (
	map[string][]pluginCandidate, map[string][]ChainItem, []string,
) {
	pluginCandidates := make(map[string][]pluginCandidate)
	chainGroups := make(map[string][]ChainItem)
	var files []string
	for _, bundle := range bundles {
		if bundle == nil || bundle.Manifest == nil {
			continue
		}
		for rawChain, items := range bundle.Manifest.Chains {
			cat := normalizeCategory(rawChain)
			for _, item := range items {
				name := strings.TrimSpace(item.Name)
				if name == "" {
					continue
				}
				key := cat + "\x00" + name
				chainGroups[key] = append(chainGroups[key], ChainItem{
					Name:         name,
					Category:     cat,
					Priority:     item.Priority,
					BundleName:   bundle.Manifest.Name,
					BundleSource: bundle.Source,
					Order:        bundle.Order,
				})
				plugin := bundle.Plugins[name]
				runtimeKey := runtimePluginKey(cat, name)
				pluginCandidates[key] = append(pluginCandidates[key], pluginCandidate{
					name: name, category: cat, runtimeKey: runtimeKey,
					priority: item.Priority, bundleName: bundle.Manifest.Name,
					source: bundle.Source, order: bundle.Order, data: plugin.Data,
				})
			}
		}
		files = append(files, bundle.Files...)
	}
	return pluginCandidates, chainGroups, files
}

func resolvePluginWinners(candidates map[string][]pluginCandidate) (map[string][]byte, []string) {
	plugins := make(map[string][]byte)
	var warnings []string
	for _, group := range candidates {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].priority != group[j].priority {
				return group[i].priority < group[j].priority
			}
			if group[i].name != group[j].name {
				return group[i].name < group[j].name
			}
			return group[i].order < group[j].order
		})
		winner := group[0]
		plugins[winner.runtimeKey] = winner.data
		for i := 1; i < len(group); i++ {
			if group[i].priority == winner.priority {
				warnings = append(warnings, fmt.Sprintf(
					"plugin %s in category %s from bundle %s ignored because bundle %s already wins at priority %d",
					winner.name, winner.category, group[i].bundleName, winner.bundleName, winner.priority,
				))
			}
		}
	}
	return plugins, warnings
}

func resolveChainWinners(chainGroups map[string][]ChainItem) ([]ChainItem, map[string][]ChainItem, []string) {
	allItems := make([]ChainItem, 0, len(chainGroups))
	categoryItems := make(map[string][]ChainItem)
	var warnings []string
	for _, items := range chainGroups {
		sortChainItems(items)
		winner := items[0]
		if winner.Category == allCategory {
			allItems = append(allItems, winner)
		} else {
			categoryItems[winner.Category] = append(categoryItems[winner.Category], winner)
		}
		for i := 1; i < len(items); i++ {
			if items[i].Priority == winner.Priority {
				warnings = append(warnings, fmt.Sprintf(
					"plugin %s in category %s from bundle %s ignored because bundle %s already wins at priority %d",
					items[i].Name, items[i].Category, items[i].BundleName, winner.BundleName, winner.Priority,
				))
			}
		}
	}
	return allItems, categoryItems, warnings
}

func resolveBundles(bundles []*Bundle) (*ResolvedBundle, error) { //nolint:unparam // error kept for future use
	out := &ResolvedBundle{
		Plugins:        make(map[string][]byte),
		CategoryChains: make(map[string][]string),
	}
	pluginCandidates, chainGroups, files := collectBundleCandidates(bundles)
	out.Files = files
	out.Plugins, out.Warnings = resolvePluginWinners(pluginCandidates)
	allItems, categoryItems, chainWarnings := resolveChainWinners(chainGroups)
	out.Warnings = append(out.Warnings, chainWarnings...)
	sortChainItems(allItems)
	out.DefaultPlugins = chainItemRuntimeNames(allItems)
	if len(out.DefaultPlugins) == 0 {
		out.Warnings = append(out.Warnings, "no all chain found after resolving plugin bundles")
	}
	for category, items := range categoryItems {
		sortChainItems(items)
		out.CategoryChains[category] = chainItemRuntimeNames(items)
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

func chainItemRuntimeNames(items []ChainItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, runtimePluginKey(item.Category, item.Name))
	}
	return out
}
