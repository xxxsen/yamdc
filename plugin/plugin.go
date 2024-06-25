package plugin

import (
	"av-capture/model"
	"fmt"
)

type IPlugin interface {
	Name() string
	Search(number string) (*model.AvMeta, error)
}

type CreatorFunc func(args interface{}) (IPlugin, error)

var mp = make(map[string]CreatorFunc)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func MakePlugin(name string, args interface{}) (IPlugin, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("plugin:%s not found", name)
	}
	return cr(args)
}
