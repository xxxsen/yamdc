package searcher

import (
	"av-capture/model"
	"fmt"
)

type ISearcher interface {
	Name() string
	Search(number string) (*model.AvMeta, error)
}

type CreatorFunc func(args interface{}) (ISearcher, error)

var mp = make(map[string]CreatorFunc)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func MakeSearcher(name string, args interface{}) (ISearcher, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("plugin:%s not found", name)
	}
	return cr(args)
}
