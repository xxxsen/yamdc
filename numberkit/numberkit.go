package numberkit

import "strings"

func IsFc2(number string) bool {
	number = strings.ToUpper(number)
	return strings.HasPrefix(number, "FC2")
}

func DecodeFc2ValID(n string) (string, bool) {
	if !IsFc2(n) {
		return "", false
	}
	idx := strings.LastIndex(n, "-")
	if idx < 0 {
		return "", false
	}
	return n[idx+1:], true
}
