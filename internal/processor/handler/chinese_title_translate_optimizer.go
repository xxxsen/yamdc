package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/resource"
)

type chineseTitleTranslateOptimizer struct {
	once sync.Once
	m    map[string]string
}

func (c *chineseTitleTranslateOptimizer) tryInitCNumber(ctx context.Context) {
	c.once.Do(func() {
		start := time.Now()
		r, err := gzip.NewReader(bytes.NewReader(resource.ResCNumber))
		if err != nil {
			logutil.GetLogger(ctx).Error("failed to read cnumber gzip data from res", zap.Error(err))
			return
		}
		m := make(map[string]string)
		err = json.NewDecoder(r).Decode(&m)
		if err != nil {
			logutil.GetLogger(ctx).Error("failed to decode cnumber json", zap.Error(err))
			return
		}
		c.m = m
		logutil.GetLogger(ctx).Info("cnumber init succ",
			zap.Int("count", len(c.m)), zap.Duration("cost", time.Since(start)))
	})
}

func (c *chineseTitleTranslateOptimizer) readTitleFromCNumber(ctx context.Context, numberid string) (
	string,
	bool,
	error,
) {
	c.tryInitCNumber(ctx)
	title, ok := c.m[numberid]
	if !ok {
		return "", false, nil
	}
	return title, true, nil
}

func (c *chineseTitleTranslateOptimizer) Handle(ctx context.Context, fc *model.FileContext) error {
	hlist := []struct {
		name    string
		handler func(ctx context.Context, numberid string) (string, bool, error)
	}{
		{"c_number", c.readTitleFromCNumber},
	}
	for _, h := range hlist {
		newTitle, ok, err := h.handler(ctx, fc.Number.GetNumberID())
		if err != nil {
			logutil.GetLogger(ctx).Error("call sub handler for optimized title failed, skip",
				zap.Error(err),
				zap.String("title_searcher", h.name),
				zap.String("numberid", fc.Number.GetNumberID()),
			)
			continue
		}
		if ok {
			logutil.GetLogger(ctx).Info("optimized chinese title found",
				zap.String("numberid", fc.Number.GetNumberID()),
				zap.String("title_searcher", h.name),
				zap.String("title", newTitle),
			)
			fc.Meta.TitleTranslated = newTitle
			return nil
		}
	}
	logutil.GetLogger(ctx).Debug("no optimized chinese title found, skip")
	return nil
}

func init() {
	Register(HChineseTitleTranslateOptimizer, func(_ any, _ appdeps.Runtime) (IHandler, error) {
		return &chineseTitleTranslateOptimizer{}, nil
	})
}
