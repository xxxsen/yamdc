package parser

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var (
	defaultDurationRegexp = regexp.MustCompile(`\s*(\d+)\s*.+`)
)

func cleanTimeSequence(res string) []string {
	list := strings.Split(res, ":")
	rs := make([]string, 0, len(list))
	for _, item := range list {
		rs = append(rs, strings.TrimSpace(item))
	}
	return rs
}

func DefaultMMDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		res, _ := strconv.ParseInt(v, 10, 64)
		return res * 60 // convert minutes to seconds
	}
}

func DefaultHHMMSSDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		res := cleanTimeSequence(v)
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

func MinuteOnlyDurationParser(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		intv, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			logutil.GetLogger(ctx).Error("decode minute only duration failed", zap.Error(err), zap.String("data", v))
			return 0
		}
		return intv * 60 // convert minutes to seconds
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

func HumanDurationToSecond(duration string) int64 {
	var totalSeconds int64
	var currentNum int64

	for _, char := range duration {
		switch char {
		case 'h':
			totalSeconds += currentNum * 3600
			currentNum = 0
		case 'm':
			totalSeconds += currentNum * 60
			currentNum = 0
		case 's':
			totalSeconds += currentNum
			currentNum = 0
		default:
			if char >= '0' && char <= '9' {
				currentNum = currentNum*10 + int64(char-'0')
			}
		}
	}
	return totalSeconds
}
