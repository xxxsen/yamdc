package parser

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"yamdc/searcher/decoder"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var (
	defaultDurationRegexp = regexp.MustCompile(`\s*(\d+)\s*.+`)
)

func DefaultHHMMSSDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		res := strings.Split(v, ":")
		if len(res) > 3 {
			logutil.GetLogger(ctx).Error("invalid time format", zap.String("data", v))
			return 0
		}
		var sec int64
		for i := 0; i < len(res); i++ {
			item := strings.TrimSpace(res[len(res)-i-1])
			val, err := strconv.ParseInt(item, 10, 60)
			if err != nil {
				logutil.GetLogger(ctx).Error("invalid time format", zap.String("data", v))
				return 0
			}
			sec += val * int64(math.Pow(60, float64(i)))
		}
		return sec
	}
}

func DefaultDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		val, err := toDuration(v)
		if err != nil {
			logutil.GetLogger(ctx).Error("decode duration failed", zap.Error(err), zap.String("data", v))
			return 0
		}
		return val
	}
}

func toDuration(timeStr string) (int64, error) {
	re := defaultDurationRegexp
	matches := re.FindStringSubmatch(timeStr)
	if len(matches) <= 1 {
		return 0, errors.New("invalid time format")
	}

	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	seconds := number * 60

	return int64(seconds), nil
}
