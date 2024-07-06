package number

import "strings"

func IsFc2(number string) bool {
	number = strings.ToUpper(number)
	return strings.HasPrefix(number, "FC2")
}
