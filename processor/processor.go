package processor

import (
	"av-capture/model"
	"context"
	"fmt"
)

type IProcessor interface {
	Name() string
	Process(ctx context.Context, meta *model.FileContext) error
	IsOptional() bool
}

type CreatorFunc func(args interface{}) (IProcessor, error)

var mp = make(map[string]CreatorFunc)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func MakeProcessor(name string, args interface{}) (IProcessor, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("processor:%s not found", name)
	}
	return cr(args)
}

func ProcessorToCreator(p IProcessor) CreatorFunc {
	return func(args interface{}) (IProcessor, error) {
		return p, nil
	}
}
