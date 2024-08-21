package parser

import (
	"context"
	"time"
	"yamdc/searcher/decoder"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

func DefaultReleaseDateParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			logutil.GetLogger(ctx).Error("decode release date failed", zap.Error(err), zap.String("data", v))
			return 0
		}
		return t.UnixMilli()
	}
}
