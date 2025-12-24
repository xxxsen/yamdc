package number

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type suffixInfoResolveFunc func(info *Number, normalizedSuffix string) bool

var defaultSuffixResolverList = []suffixInfoResolveFunc{
	resolveIsChineseSubTitle,
	resolveCDInfo,
	resolve4K,
	resolve8K,
	resolveVr,
	resolveLeak,
	resolveHack,
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
	if !strings.HasPrefix(str, defaultSuffixMultiCD) {
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

func resolveLeak(info *Number, str string) bool {
	if str != defaultSuffixLeak {
		return false
	}
	info.isLeak = true
	return true
}

func resolveHack(info *Number, str string) bool {
	if str != defaultSuffixHack1 && str != defaultSuffixHack2 {
		return false
	}
	info.isHack = true
	return true
}

func resolve4K(info *Number, str string) bool {
	if str != defaultSuffix4K && str != defaultSuffix4KV2 {
		return false
	}
	info.is4k = true
	return true
}

func resolve8K(info *Number, str string) bool {
	if str != defaultSuffix8K {
		return false
	}
	info.is8k = true
	return true
}

func resolveVr(info *Number, str string) bool {
	if str != defaultSuffixVR {
		return false
	}
	info.isVr = true
	return true
}

func resolveIsChineseSubTitle(info *Number, str string) bool {
	if str != defaultSuffixChineseSubtitle {
		return false
	}
	info.isChineseSubtitle = true
	return true
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
	number := strings.ToUpper(str) //默认所有的番号都是大写的
	rs := &Number{
		numberId:          "",
		isChineseSubtitle: false,
		isMultiCD:         false,
		multiCDIndex:      0,
	}
	//部分番号需要进行改写, 改写逻辑提到外面去, number只做解析用

	//提取后缀信息并对番号进行裁剪
	number = resolveSuffixInfo(rs, number)
	rs.numberId = number
	return rs, nil
}

// GetCleanID 将番号中`-`, `_` 进行移除
func GetCleanID(str string) string {
	sb := strings.Builder{}
	for _, c := range str {
		if c == '-' || c == '_' {
			continue
		}
		sb.WriteRune(c)
	}
	return sb.String()
}
