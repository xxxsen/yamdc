package processor

import (
	"av-capture/model"
	"context"
)

var DefaultProcessor IProcessor = &defaultProcessor{}

type defaultProcessor struct {
}

func (p *defaultProcessor) Name() string {
	return "default"
}

func (p *defaultProcessor) Process(ctx context.Context, fc *model.FileContext) error {
	return nil
}

func (p *defaultProcessor) IsOptional() bool {
	return true
}
