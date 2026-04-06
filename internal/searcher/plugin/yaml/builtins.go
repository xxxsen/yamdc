package yaml

import (
	"sort"
	"sync"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
)

var bundleMu sync.Mutex

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
	bundleMu.Lock()
	ctx := factory.NewRegisterContext()
	for _, name := range names {
		registerBytes(ctx, name, plugins[name])
	}
	factory.Swap(ctx)
	bundleMu.Unlock()
}

func registerBytes(ctx *factory.RegisterContext, name string, data []byte) {
	cc := &cachedCreator{data: data}
	ctx.Register(name, cc.create)
}
