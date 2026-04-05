package yamlplugin

import (
	"fmt"
	"sync"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	yamlassets "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
)

const legacyPrefix = "legacy:"

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

func RegisterBuiltins() {
	mustRegisterFromFile(constant.SSJav321, "jav321.yaml")
	mustRegisterFromFile(constant.SSJavDB, "javdb.yaml")
	mustRegisterFromFile(constant.SSJavBus, "javbus.yaml")
	mustRegisterFromFile(constant.SSMissav, "missav.yaml")
	mustRegisterFromFile(constant.SSTKTube, "tktube.yaml")
	mustRegisterFromFile(constant.SSJvrPorn, "jvrporn.yaml")
	mustRegisterFromFile(constant.SSJavhoo, "javhoo.yaml")
	mustRegisterFromFile(constant.SSJavLibrary, "javlibrary.yaml")
	mustRegisterFromFile(constant.SSFreeJavBt, "freejavbt.yaml")
	mustRegisterFromFile(constant.SS18AV, "18av.yaml")
	mustRegisterFromFile(constant.SSNJav, "njav.yaml")
	mustRegisterFromFile(constant.SSFc2PPVDB, "fc2ppvdb.yaml")
	mustRegisterFromFile(constant.SSCaribpr, "caribpr.yaml")
	mustRegisterFromFile(constant.SSMadouqu, "madouqu.yaml")
	mustRegisterFromFile(constant.SSAvsox, "avsox.yaml")
	mustRegisterFromFile(constant.SSManyVids, "manyvids.yaml")
	mustRegisterFromFile(constant.SSFc2, "fc2.yaml")
	mustRegisterFromFile(constant.SSAirav, "airav.yaml")
	mustRegisterFromFile(constant.SSCospuri, "cospuri.yaml")
}

func LegacyPluginName(name string) string {
	return legacyPrefix + name
}

func mustRegisterFromFile(name, file string) {
	if legacy, ok := factory.Lookup(name); ok {
		factory.Register(LegacyPluginName(name), legacy)
	}
	data, err := yamlassets.ReadFile(file)
	if err != nil {
		panic(fmt.Errorf("read yaml plugin %s failed, err:%w", file, err))
	}
	cc := &cachedCreator{data: data}
	factory.Register(name, cc.create)
}

func LoadBuiltinYAML(name string) ([]byte, error) {
	return yamlassets.ReadFile(name + ".yaml")
}
