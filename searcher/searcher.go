package searcher

import (
	"av-capture/model"
	"context"
)

type ISearcher interface {
	Name() string
	Search(ctx context.Context, number string) (*model.AvMeta, bool, error)
}

type CreatorFunc func(args interface{}) (ISearcher, error)

var mp = make(map[string]ISearcher)

func Register(ss ISearcher) {
	mp[ss.Name()] = ss
}

func Get(name string) (ISearcher, bool) {
	if v, ok := mp[name]; ok {
		return v, true
	}
	return nil, false
}
