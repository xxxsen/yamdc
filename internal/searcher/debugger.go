package searcher

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/number"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/store"
)

type DebugSearchOptions struct {
	Input      string   `json:"input"`
	Plugins    []string `json:"plugins"`
	UseCleaner bool     `json:"use_cleaner"`
	SkipAssets bool     `json:"skip_assets"`
}

type DebugSearchResult struct {
	Input          string                        `json:"input"`
	NumberID       string                        `json:"number_id"`
	RequestedInput string                        `json:"requested_input"`
	UsedPlugins    []string                      `json:"used_plugins"`
	MatchedPlugin  string                        `json:"matched_plugin"`
	Found          bool                          `json:"found"`
	Category       string                        `json:"category"`
	Uncensor       bool                          `json:"uncensor"`
	CleanerResult  *movieidcleaner.Result        `json:"cleaner_result,omitempty"`
	Meta           *model.MovieMeta              `json:"meta,omitempty"`
	PluginResults  []PluginDebugResult           `json:"plugin_results"`
	AvailableTools SearcherDebugPluginCollection `json:"available_tools"`
}

type SearcherDebugPluginCollection struct {
	Available []string            `json:"available"`
	Default   []string            `json:"default"`
	Category  map[string][]string `json:"category"`
}

type PluginDebugResult struct {
	Plugin string            `json:"plugin"`
	Found  bool              `json:"found"`
	Error  string            `json:"error,omitempty"`
	Meta   *model.MovieMeta  `json:"meta,omitempty"`
	Steps  []PluginDebugStep `json:"steps"`
}

type PluginDebugStep struct {
	Stage      string `json:"stage"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	URL        string `json:"url,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

type Debugger struct {
	cli             client.IHTTPClient
	storage         store.IStorage
	cleaner         movieidcleaner.Cleaner
	mu              sync.RWMutex
	defaultPlugins  []string
	categoryPlugins map[string][]string
	creators        map[string]factory.CreatorFunc
}

func NewDebugger(cli client.IHTTPClient, storage store.IStorage, cleaner movieidcleaner.Cleaner, defaultPlugins []string, categoryPlugins map[string][]string) *Debugger {
	d := &Debugger{
		cli:     cli,
		storage: storage,
		cleaner: cleaner,
	}
	d.SwapPlugins(defaultPlugins, categoryPlugins)
	return d
}

func (d *Debugger) SwapPlugins(defaultPlugins []string, categoryPlugins map[string][]string) {
	d.SwapState(defaultPlugins, categoryPlugins, factory.Snapshot())
}

func (d *Debugger) SwapState(defaultPlugins []string, categoryPlugins map[string][]string, creators map[string]factory.CreatorFunc) {
	cp := make(map[string][]string, len(categoryPlugins))
	for key, items := range categoryPlugins {
		cp[strings.ToUpper(strings.TrimSpace(key))] = append([]string(nil), items...)
	}
	nextCreators := make(map[string]factory.CreatorFunc, len(creators))
	for name, creator := range creators {
		nextCreators[name] = creator
	}
	d.mu.Lock()
	d.defaultPlugins = append([]string(nil), defaultPlugins...)
	d.categoryPlugins = cp
	d.creators = nextCreators
	d.mu.Unlock()
}

func (d *Debugger) Plugins() SearcherDebugPluginCollection {
	d.mu.RLock()
	defaultPlugins := append([]string(nil), d.defaultPlugins...)
	categoryPlugins := cloneStringMap(d.categoryPlugins)
	creators := cloneCreators(d.creators)
	d.mu.RUnlock()
	available := collectVisiblePlugins(defaultPlugins, categoryPlugins, creators)
	sort.Strings(defaultPlugins)
	return SearcherDebugPluginCollection{
		Available: available,
		Default:   defaultPlugins,
		Category:  categoryPlugins,
	}
}

func (d *Debugger) DebugSearch(ctx context.Context, opts DebugSearchOptions) (*DebugSearchResult, error) {
	input := strings.TrimSpace(opts.Input)
	if input == "" {
		return nil, fmt.Errorf("input is required")
	}
	useCleaner := true
	if !opts.UseCleaner {
		useCleaner = false
	}
	result := &DebugSearchResult{
		Input:          input,
		RequestedInput: input,
		AvailableTools: d.Plugins(),
	}

	var num *number.Number
	var err error
	if useCleaner && d.cleaner != nil {
		cleanRes, cleanErr := d.cleaner.Clean(input)
		if cleanErr != nil {
			return nil, cleanErr
		}
		result.CleanerResult = cleanRes
		if cleanRes != nil && strings.TrimSpace(cleanRes.Normalized) != "" {
			num, err = number.Parse(cleanRes.Normalized)
			if err != nil {
				return nil, err
			}
			if cleanRes.CategoryMatched {
				num.SetExternalFieldCategory(cleanRes.Category)
				result.Category = cleanRes.Category
			}
			if cleanRes.UncensorMatched {
				num.SetExternalFieldUncensor(cleanRes.Uncensor)
				result.Uncensor = cleanRes.Uncensor
			}
		}
	}
	if num == nil {
		num, err = number.Parse(input)
		if err != nil {
			return nil, err
		}
	}
	result.NumberID = num.GetNumberID()

	plugins := normalizePluginList(opts.Plugins)
	if len(plugins) == 0 {
		plugins = d.resolvePlugins(num)
	}
	result.UsedPlugins = append([]string(nil), plugins...)

	for _, name := range plugins {
		trace, err := d.debugOnePlugin(ctx, name, num, opts.SkipAssets)
		if err != nil {
			return nil, err
		}
		result.PluginResults = append(result.PluginResults, *trace)
		if trace.Found && trace.Meta != nil {
			result.Found = true
			result.MatchedPlugin = trace.Plugin
			result.Meta = trace.Meta
			break
		}
	}
	return result, nil
}

func collectVisiblePlugins(defaultPlugins []string, categoryPlugins map[string][]string, creators map[string]factory.CreatorFunc) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(defaultPlugins))
	for _, name := range defaultPlugins {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		if _, ok := creators[name]; !ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, items := range categoryPlugins {
		for _, name := range items {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			if _, ok := creators[name]; !ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (d *Debugger) resolvePlugins(num *number.Number) []string {
	d.mu.RLock()
	defaultPlugins := append([]string(nil), d.defaultPlugins...)
	categoryPlugins := cloneStringMap(d.categoryPlugins)
	d.mu.RUnlock()
	if num == nil {
		return defaultPlugins
	}
	cat := strings.ToUpper(strings.TrimSpace(num.GetExternalFieldCategory()))
	if cat != "" {
		if chain, ok := categoryPlugins[cat]; ok && len(chain) != 0 {
			return append([]string(nil), chain...)
		}
	}
	return defaultPlugins
}

func (d *Debugger) debugOnePlugin(ctx context.Context, name string, num *number.Number, skipAssets bool) (*PluginDebugResult, error) {
	d.mu.RLock()
	creator, ok := d.creators[name]
	d.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin:%s not found", name)
	}
	plg, err := creator(struct{}{})
	if err != nil {
		return nil, err
	}
	searcher, err := NewDefaultSearcher(name, plg, WithHTTPClient(d.cli), WithStorage(d.storage), WithSearchCache(false))
	if err != nil {
		return nil, err
	}
	def, ok := searcher.(*DefaultSearcher)
	if !ok {
		return nil, fmt.Errorf("searcher %s is not default searcher", name)
	}
	return def.debugSearch(ctx, num, skipAssets), nil
}

func cloneCreators(in map[string]factory.CreatorFunc) map[string]factory.CreatorFunc {
	out := make(map[string]factory.CreatorFunc, len(in))
	for name, creator := range in {
		out[name] = creator
	}
	return out
}

func (p *DefaultSearcher) debugSearch(ctx context.Context, num *number.Number, skipAssets bool) *PluginDebugResult {
	trace := &PluginDebugResult{Plugin: p.name}
	ctx = pluginapi.InitContainer(ctx)
	ctx = meta.SetNumberId(ctx, num.GetNumberID())

	ok, err := p.plg.OnPrecheckRequest(ctx, num.GetNumberID())
	if err != nil {
		trace.Error = err.Error()
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "precheck", OK: false, Message: err.Error()})
		return trace
	}
	trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "precheck", OK: ok, Message: boolMessage(ok, "precheck passed", "precheck skipped current plugin")})
	if !ok {
		return trace
	}

	req, err := p.plg.OnMakeHTTPRequest(ctx, num.GetNumberID())
	if err != nil {
		trace.Error = err.Error()
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "make_request", OK: false, Message: err.Error()})
		return trace
	}
	trace.Steps = append(trace.Steps, PluginDebugStep{
		Stage:   "make_request",
		OK:      true,
		Message: "request created",
		URL:     requestURL(req),
	})

	invoker := func(callCtx context.Context, req *http.Request) (*http.Response, error) {
		start := time.Now()
		rsp, invokeErr := p.invokeHTTPRequest(callCtx, req)
		step := PluginDebugStep{
			Stage:      "request",
			OK:         invokeErr == nil,
			URL:        requestURL(req),
			DurationMS: time.Since(start).Milliseconds(),
		}
		if rsp != nil {
			step.StatusCode = rsp.StatusCode
		}
		if invokeErr != nil {
			step.Message = invokeErr.Error()
		} else {
			step.Message = "request finished"
		}
		trace.Steps = append(trace.Steps, step)
		return rsp, invokeErr
	}

	rsp, err := p.plg.OnHandleHTTPRequest(ctx, invoker, req)
	if err != nil {
		trace.Error = err.Error()
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "handle_http", OK: false, Message: err.Error()})
		return trace
	}
	if rsp == nil {
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "handle_http", OK: false, Message: "plugin returned nil response"})
		return trace
	}

	ok, err = p.plg.OnPrecheckResponse(ctx, req, rsp)
	if err != nil {
		trace.Error = err.Error()
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "precheck_response", OK: false, Message: err.Error(), StatusCode: rsp.StatusCode})
		return trace
	}
	trace.Steps = append(trace.Steps, PluginDebugStep{
		Stage:      "precheck_response",
		OK:         ok,
		Message:    boolMessage(ok, "response accepted", "response rejected"),
		StatusCode: rsp.StatusCode,
		URL:        requestURL(req),
	})
	if !ok {
		_ = rsp.Body.Close()
		return trace
	}
	if rsp.StatusCode != http.StatusOK {
		trace.Steps = append(trace.Steps, PluginDebugStep{
			Stage:      "response_status",
			OK:         false,
			Message:    fmt.Sprintf("invalid http status code:%d", rsp.StatusCode),
			StatusCode: rsp.StatusCode,
		})
		_ = rsp.Body.Close()
		return trace
	}

	data, err := client.ReadHTTPData(rsp)
	_ = rsp.Body.Close()
	if err != nil {
		trace.Error = err.Error()
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "read_body", OK: false, Message: err.Error()})
		return trace
	}
	trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "read_body", OK: true, Message: fmt.Sprintf("response bytes: %d", len(data))})

	metaInfo, decodeSucc, err := p.plg.OnDecodeHTTPData(ctx, data)
	if err != nil {
		trace.Error = err.Error()
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "decode", OK: false, Message: err.Error()})
		return trace
	}
	trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "decode", OK: decodeSucc, Message: boolMessage(decodeSucc, "decode success", "decode returned no result")})
	if !decodeSucc || metaInfo == nil {
		return trace
	}

	p.fixMeta(ctx, req, metaInfo)
	trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "fix_meta", OK: true, Message: "meta normalized"})

	if skipAssets {
		trace.Steps = append(trace.Steps, PluginDebugStep{
			Stage:   "store_assets",
			OK:      true,
			Message: "asset fetch skipped",
		})
	} else {
		p.storeImageData(ctx, metaInfo)
		trace.Steps = append(trace.Steps, PluginDebugStep{
			Stage:   "store_assets",
			OK:      metaHasAssets(metaInfo),
			Message: fmt.Sprintf("cover=%t poster=%t sample_images=%d", hasFileKey(metaInfo.Cover), hasFileKey(metaInfo.Poster), countSampleKeys(metaInfo.SampleImages)),
		})
	}

	if err := p.verifyMeta(metaInfo); err != nil {
		trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "verify_meta", OK: false, Message: err.Error()})
		return trace
	}
	trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "verify_meta", OK: true, Message: "meta verified"})

	metaInfo.ExtInfo.ScrapeInfo.Source = p.name
	metaInfo.ExtInfo.ScrapeInfo.DateTs = time.Now().UnixMilli()
	trace.Steps = append(trace.Steps, PluginDebugStep{Stage: "result", OK: true, Message: "plugin returned meta"})
	trace.Meta = metaInfo
	trace.Found = true
	return trace
}

func normalizePluginList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		for _, part := range strings.Split(item, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

func cloneStringMap(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return map[string][]string{}
	}
	dst := make(map[string][]string, len(src))
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := append([]string(nil), src[key]...)
		sort.Strings(values)
		dst[key] = values
	}
	return dst
}

func requestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return req.URL.String()
}

func hasFileKey(file *model.File) bool {
	return file != nil && strings.TrimSpace(file.Key) != ""
}

func countSampleKeys(items []*model.File) int {
	if len(items) == 0 {
		return 0
	}
	total := 0
	for _, item := range items {
		if hasFileKey(item) {
			total++
		}
	}
	return total
}

func metaHasAssets(meta *model.MovieMeta) bool {
	if meta == nil {
		return false
	}
	return hasFileKey(meta.Cover) || hasFileKey(meta.Poster) || countSampleKeys(meta.SampleImages) > 0
}

func boolMessage(ok bool, trueText string, falseText string) string {
	if ok {
		return trueText
	}
	return falseText
}
