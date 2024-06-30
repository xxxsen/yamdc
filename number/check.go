package number

import (
	"regexp"
	"strings"
)

var defaultUncensorPrefix = []string{
	"1pon",
	"carib",
	"sm3d2dbd",
	"smdv",
	"sky",
	"hey",
	"fc2",
	"mkd",
	"mkbd",
	"h4610",
	"h0930",
}

var defaultUncensorRegexp = []*regexp.Regexp{
	regexp.MustCompile(`^\d+[-|_]\d+$`),
	regexp.MustCompile(`^n\d+$`),
}

func IsUncensorMovie(str string) bool {
	str = strings.ToLower(str)
	for _, prefix := range defaultUncensorPrefix {
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	for _, regexpr := range defaultUncensorRegexp {
		if regexpr.MatchString(str) {
			return true
		}
	}
	return false
}
