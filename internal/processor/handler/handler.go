package handler

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
	"sort"
)

type IHandler interface {
	Handle(ctx context.Context, fc *model.FileContext) error
}

var mp = make(map[string]CreatorFunc)

type CreatorFunc func(args interface{}, deps appdeps.Runtime) (IHandler, error)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func CreateHandler(name string, args interface{}, deps appdeps.Runtime) (IHandler, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("handler:%s not found", name)
	}
	return cr(args, deps)
}

func HandlerToCreator(h IHandler) CreatorFunc {
	return func(args interface{}, deps appdeps.Runtime) (IHandler, error) {
		return h, nil
	}
}

func Handlers() []string {
	rs := make([]string, 0, len(mp))
	for k := range mp {
		rs = append(rs, k)
	}
	return sort.StringSlice(rs)
}
