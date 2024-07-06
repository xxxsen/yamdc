package number

import (
	"regexp"
	"strings"
)

var defaultUncensorPrefix = []string{
	"1PON",
	"CARIB",
	"SM3D2DBD",
	"SMDV",
	"SKY",
	"HEY",
	"FC2",
	"MKD",
	"MKBD",
	"H4610",
	"H0930",
	"MD-",
	"SMD-",
	"SSDV-",
	"CCDV-",
	"LLDV-",
	"DRC-",
	"MXX-",
	"DSAM-",
}

var defaultUncensorRegexp = []*regexp.Regexp{
	regexp.MustCompile(`^\d+[-|_]\d+$`),
	regexp.MustCompile(`^N\d+$`),
	regexp.MustCompile(`^K\d+$`),
	regexp.MustCompile(`^KB\d+$`),
	regexp.MustCompile(`^C\d+-KI\d+$`),
}

func IsUncensorMovie(str string) bool {
	str = strings.ToUpper(str)
	str = strings.ReplaceAll(str, "_", "-")
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
