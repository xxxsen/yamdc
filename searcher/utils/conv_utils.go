package utils

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

var (
	defaultDurationRegexp = regexp.MustCompile(`\s*(\d+)\s*.+`)
)

func ToTimestamp(date string) (int64, error) {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func ToTimestampOrDefault(date string, def int64) int64 {
	if v, err := ToTimestamp(date); err == nil {
		return v
	}
	return def
}

func ToDuration(timeStr string) (int64, error) {
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
