package utils

import "strings"

func NormalizeNumber(num string) string {
	num = strings.ReplaceAll(num, "-", "")
	num = strings.ReplaceAll(num, "_", "")
	return num
}
