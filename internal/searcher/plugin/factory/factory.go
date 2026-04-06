package factory

import (
	"fmt"
	"maps"
	"sort"
	"sync/atomic"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

type CreatorFunc func(args interface{}) (api.IPlugin, error)

type RegisterContext struct {
	mp map[string]CreatorFunc
}

var registry atomic.Value

func init() {
	registry.Store(map[string]CreatorFunc{})
}

func NewRegisterContext() *RegisterContext {
	return &RegisterContext{mp: make(map[string]CreatorFunc)}
}

func (r *RegisterContext) Register(name string, fn CreatorFunc) {
	r.mp[name] = fn
}

func (r *RegisterContext) Snapshot() map[string]CreatorFunc {
	out := make(map[string]CreatorFunc, len(r.mp))
	maps.Copy(out, r.mp)
	return out
}

func Swap(ctx *RegisterContext) {
	next := make(map[string]CreatorFunc, len(ctx.mp))
	maps.Copy(next, ctx.mp)
	registry.Store(next)
}

func CreatePlugin(name string, args interface{}) (api.IPlugin, error) {
	cr, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("plugin:%s not found", name)
	}
	return cr(args)
}

func Lookup(name string) (CreatorFunc, bool) {
	cr, ok := currentRegistry()[name]
	return cr, ok
}

func PluginToCreator(plg api.IPlugin) CreatorFunc {
	return func(args interface{}) (api.IPlugin, error) {
		return plg, nil
	}
}

func Plugins() []string {
	current := currentRegistry()
	rs := make([]string, 0, len(current))
	for k := range current {
		rs = append(rs, k)
	}
	sort.Strings(rs)
	return rs
}

func Snapshot() map[string]CreatorFunc {
	current := currentRegistry()
	out := make(map[string]CreatorFunc, len(current))
	maps.Copy(out, current)
	return out
}

func currentRegistry() map[string]CreatorFunc {
	current, ok := registry.Load().(map[string]CreatorFunc)
	if !ok {
		panic("invalid plugin factory registry")
	}
	return current
}
