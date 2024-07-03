package number

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	defaultCDNumberParserRegexp = regexp.MustCompile(`^(.*)[-_][cC][dD](\d+)`)
)

var defaultNumberResolveList = []numberResolveFunc{
	resolveIsChineseSubTitle,
	resolveCDInfo,
	resolveIsUncensorMovie,
}

func resolveCDInfo(info *Number, str string) string {
	matches := defaultCDNumberParserRegexp.FindStringSubmatch(str)
	if len(matches) <= 2 {
		return str
	}
	cdidx, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return str
	}
	base := matches[1]
	info.isMultiCD = true
	info.multiCDIndex = int(cdidx)
	return base
}

func resolveIsChineseSubTitle(info *Number, str string) string {
	tmp := strings.ToLower(str)
	if !(strings.HasSuffix(tmp, "_c") || strings.HasSuffix(tmp, "-c")) {
		return str
	}
	info.isChineseSubtitle = true
	base := str[:len(str)-2]
	return base
}

func resolveIsUncensorMovie(info *Number, str string) string {
	if IsUncensorMovie(str) {
		info.isUncensorMovie = true
	}
	return str
}

type numberResolveFunc func(info *Number, str string) string

func ParseWithFileName(f string) (*Number, error) {
	filename := filepath.Base(f)
	fileext := filepath.Ext(f)
	filenoext := filename[:len(filename)-len(fileext)]
	return Parse(filenoext)
}

func Parse(str string) (*Number, error) {
	if len(str) == 0 {
		return nil, fmt.Errorf("empty number str")
	}
	rs := &Number{
		number:            "",
		isChineseSubtitle: false,
		isMultiCD:         false,
		multiCDIndex:      0,
		isUncensorMovie:   false,
	}
	number := str
	steps := defaultNumberResolveList
	for _, step := range steps {
		number = step(rs, number)
	}
	rs.number = strings.ToUpper(number)
	return rs, nil
}
