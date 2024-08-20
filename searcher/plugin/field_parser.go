package plugin

import (
	"context"
	"strconv"
	"strings"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

func DefaultHHMMSSDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		res := strings.Split(v, ":")
		if len(res) != 3 {
			logutil.GetLogger(ctx).Error("invalid time format", zap.String("data", v))
			return 0
		}
		h, _ := strconv.ParseInt(strings.TrimSpace(res[0]), 10, 64)
		m, _ := strconv.ParseInt(strings.TrimSpace(res[1]), 10, 64)
		s, _ := strconv.ParseInt(strings.TrimSpace(res[2]), 10, 64)
		return h*3600 + m*60 + s
	}
}

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
