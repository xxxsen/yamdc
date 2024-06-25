package utils

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

var (
	defaultDurationRegexp = regexp.MustCompile(`^(\d+)(分鐘)$`)
)

func ToTimestamp(date string) (int64, error) {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func ToDuration(timeStr string) (int64, error) {
	// Define a regular expression to match the time format
	re := defaultDurationRegexp
	matches := re.FindStringSubmatch(timeStr)

	// If the input string doesn't match the expected format, return an error
	if len(matches) != 3 {
		return 0, errors.New("invalid time format")
	}

	// Extract the number part and convert it to an integer
	numberStr := matches[1]
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0, err
	}

	// Convert the number of minutes to seconds
	seconds := number * 60

	return int64(seconds), nil
}
