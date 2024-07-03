package number

import "strings"

func IsFc2(number string) bool {
	number = strings.ToLower(number)
	return strings.HasPrefix(number, "fc2")
}
