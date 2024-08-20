package parser

import (
	"context"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

func DefaultReleaseDateParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		val, err := utils.ToTimestamp(v)
		if err != nil {
			logutil.GetLogger(ctx).Error("decode release date failed", zap.Error(err), zap.String("data", v))
			return 0
		}
		return val
	}
}
