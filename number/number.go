package number

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type suffixInfoResolveFunc func(info *Number, normalizedSuffix string) bool
type numberInfoResolveFunc func(info *Number, number string)

var defaultSuffixResolverList = []suffixInfoResolveFunc{
	resolveIsChineseSubTitle,
	resolveCDInfo,
	resolve4K,
}

var defaultNumberInfoResolverList = []numberInfoResolveFunc{
	resolveIsUncensorMovie,
}

func extractSuffix(str string) (string, bool) {
	for i := len(str) - 1; i >= 0; i-- {
		if str[i] == '_' || str[i] == '-' {
			return str[i:], true
		}
	}
	return "", false
}

func tryResolveSuffix(info *Number, suffix string) bool {
	normalizedSuffix := strings.ToUpper(suffix[1:])
	for _, resolver := range defaultSuffixResolverList {
		if resolver(info, normalizedSuffix) {
			return true
		}
	}
	return false
}

func resolveSuffixInfo(info *Number, str string) string {
	for {
		suffix, ok := extractSuffix(str)
		if !ok {
			return str
		}
		if !tryResolveSuffix(info, suffix) {
			return str
		}
		str = str[:len(str)-len(suffix)]
	}
}

func resolveCDInfo(info *Number, str string) bool {
	if !strings.HasPrefix(str, "CD") {
		return false
	}
	strNum := str[2:]
	num, err := strconv.ParseInt(strNum, 10, 64)
	if err != nil {
		return false
	}
	info.isMultiCD = true
	info.multiCDIndex = int(num)
	return true
}

func resolve4K(info *Number, str string) bool {
	if str != "4K" {
		return false
	}
	info.is4k = true
	return true
}

func resolveIsChineseSubTitle(info *Number, str string) bool {
	if str != "C" {
		return false
	}
	info.isChineseSubtitle = true
	return true
}

func resolveNumberInfo(info *Number, number string) {
	for _, resolver := range defaultNumberInfoResolverList {
		resolver(info, number)
	}
}

func resolveIsUncensorMovie(info *Number, str string) {
	if IsUncensorMovie(str) {
		info.isUncensorMovie = true
	}
}

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
	if strings.Contains(str, ".") {
		return nil, fmt.Errorf("should not contain extname, str:%s", str)
	}
	rs := &Number{
		number:            "",
		isChineseSubtitle: false,
		isMultiCD:         false,
		multiCDIndex:      0,
		isUncensorMovie:   false,
	}
	//提取后缀信息并对番号进行裁剪
	number := resolveSuffixInfo(rs, str)
	rs.number = strings.ToUpper(number)
	//通过番号直接填充信息(不进行裁剪)
	resolveNumberInfo(rs, number)
	return rs, nil
}
