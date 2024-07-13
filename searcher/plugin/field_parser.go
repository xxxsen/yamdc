package plugin

import (
	"context"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

func DefaultDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		val, err := utils.ToDuration(v)
		if err != nil {
			logutil.GetLogger(ctx).Error("decode duration failed", zap.Error(err), zap.String("data", v))
			return 0
		}
		return val
	}
}

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
