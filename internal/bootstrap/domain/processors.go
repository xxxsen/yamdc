package domain

import (
	"context"
	"fmt"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/processor/handler"
)

func BuildProcessor(
	ctx context.Context,
	deps appdeps.Runtime,
	hs []string,
	m map[string]HandlerOption,
) ([]processor.IProcessor, error) {
	rs := make([]processor.IProcessor, 0, len(hs))
	for _, name := range hs {
		opt := m[name]
		if opt.Disable {
			logutil.GetLogger(ctx).Info("handler is disabled, skip create", zap.String("handler", name))
			continue
		}
		h, err := handler.CreateHandler(name, opt.Args, deps)
		if err != nil {
			return nil, fmt.Errorf("create handler failed, name:%s, err:%w", name, err)
		}
		p := processor.NewProcessor(name, h)
		logutil.GetLogger(ctx).Info("create processor succ", zap.String("handler", name))
		rs = append(rs, p)
	}
	return rs, nil
}
