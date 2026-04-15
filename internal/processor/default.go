package processor

import (
	"context"
	"fmt"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/processor/handler"
)

var DefaultProcessor IProcessor = &defaultProcessor{}

type defaultProcessor struct{}

func (p *defaultProcessor) Name() string {
	return "default"
}

func (p *defaultProcessor) Process(_ context.Context, _ *model.FileContext) error {
	return nil
}

type processorImpl struct {
	name string
	h    handler.IHandler
}

func NewProcessor(name string, h handler.IHandler) IProcessor {
	return &processorImpl{name: name, h: h}
}

func (p *processorImpl) Name() string {
	return p.name
}

func (p *processorImpl) Process(ctx context.Context, meta *model.FileContext) error {
	if err := p.h.Handle(ctx, meta); err != nil {
		return fmt.Errorf("handler %s failed: %w", p.name, err)
	}
	return nil
}
