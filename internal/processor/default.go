package processor

import (
	"context"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/processor/handler"
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
	return p.h.Handle(ctx, meta)
}
