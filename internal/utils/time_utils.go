package utils

import "time"

func FormatTimeToDate(ts int64) string {
	t := time.UnixMilli(ts)
	return t.Format(time.DateOnly)
}
