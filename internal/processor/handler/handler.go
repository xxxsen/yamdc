package handler

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
)

type IHandler interface {
	Handle(ctx context.Context, fc *model.FileContext) error
}

var errHandlerNotFound = errors.New("handler not found")

var mp = make(map[string]CreatorFunc)

type CreatorFunc func(args any, deps appdeps.Runtime) (IHandler, error)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func CreateHandler(name string, args any, deps appdeps.Runtime) (IHandler, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("handler:%s: %w", name, errHandlerNotFound)
	}
	return cr(args, deps)
}

func ToCreator(h IHandler) CreatorFunc {
	return func(_ any, _ appdeps.Runtime) (IHandler, error) {
		return h, nil
	}
}

func Handlers() []string {
	rs := make([]string, 0, len(mp))
	for k := range mp {
		rs = append(rs, k)
	}
	sort.Strings(rs)
	return rs
}
