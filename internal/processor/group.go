package processor

import (
	"context"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type group struct {
	ps []IProcessor
}

func NewGroup(ps []IProcessor) IProcessor {
	return &group{ps: ps}
}

func (g *group) Name() string {
	return "group"
}

func (g *group) Process(ctx context.Context, fc *model.FileContext) error {
	var lastErr error
	for _, p := range g.ps {
		err := p.Process(ctx, fc)
		if err == nil {
			continue
		}
		logutil.GetLogger(ctx).Error("process failed", zap.Error(err), zap.String("name", p.Name()))
		lastErr = err
	}
	return lastErr
}
