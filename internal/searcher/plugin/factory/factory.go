package factory

import (
	"fmt"
	"sort"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

type CreatorFunc func(args interface{}) (api.IPlugin, error)

var mp = make(map[string]CreatorFunc)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func CreatePlugin(name string, args interface{}) (api.IPlugin, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("plugin:%s not found", name)
	}
	return cr(args)
}

func PluginToCreator(plg api.IPlugin) CreatorFunc {
	return func(args interface{}) (api.IPlugin, error) {
		return plg, nil
	}
}

func Plugins() []string {
	rs := make([]string, 0, len(mp))
	for k := range mp {
		rs = append(rs, k)
	}
	return sort.StringSlice(rs)
}
