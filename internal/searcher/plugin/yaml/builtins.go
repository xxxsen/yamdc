package yaml

import (
	"sort"
	"sync"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
)

var (
	bundleMu            sync.Mutex
	currentBundlePlugin = make(map[string]struct{})
)

type cachedCreator struct {
	data   []byte
	once   sync.Once
	plugin api.IPlugin
	err    error
}

func (c *cachedCreator) create(args interface{}) (api.IPlugin, error) {
	c.once.Do(func() {
		c.plugin, c.err = NewFromBytes(c.data)
	})
	if c.err != nil {
		return nil, c.err
	}
	return c.plugin, nil
}

func SyncBundle(plugins map[string][]byte) {
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	next := make(map[string]struct{}, len(names))
	bundleMu.Lock()
	for _, name := range names {
		next[name] = struct{}{}
	}
	for name := range currentBundlePlugin {
		if _, ok := next[name]; ok {
			continue
		}
		factory.Unregister(name)
	}
	for _, name := range names {
		registerBytes(name, plugins[name])
	}
	currentBundlePlugin = next
	bundleMu.Unlock()
}

func registerBytes(name string, data []byte) {
	cc := &cachedCreator{data: data}
	factory.Register(name, cc.create)
}
