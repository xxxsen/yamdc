package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func FormatTimeToDate(ts int64) string {
	t := time.UnixMilli(ts)
	return t.Format(time.DateOnly)
}

func TimeStrToSecond(str string) (int64, error) {
	// 解析时间字符串
	parts := strings.Split(str, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format")
	}
	h, e1 := strconv.ParseInt(parts[0], 10, 64)
	m, e2 := strconv.ParseInt(parts[1], 10, 64)
	s, e3 := strconv.ParseInt(parts[2], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, fmt.Errorf("parse time str failed, e1:%w, e2:%w, e3:%w", e1, e2, e3)
	}
	return h*3600 + m*60 + s, nil
}
