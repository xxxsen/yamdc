package number

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	errEmptyNumberStr  = errors.New("empty number str")
	errContainsExtName = errors.New("should not contain extname")
)

type suffixInfoResolveFunc func(info *Number, normalizedSuffix string) bool

var defaultSuffixResolverList = []suffixInfoResolveFunc{
	resolveIsChineseSubTitle,
	resolveCDInfo,
	resolve4K,
	resolve8K,
	resolveVr,
	resolveSpecialEdition,
	resolveRestored,
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

func resolveSpecialEdition(info *Number, str string) bool {
	if str != defaultSuffixSpecialEdition {
		return false
	}
	info.isSpecialEdition = true
	return true
}

func resolveRestored(info *Number, str string) bool {
	if str != defaultSuffixRestored1 && str != defaultSuffixRestored2 {
		return false
	}
	info.isRestored = true
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
		return nil, errEmptyNumberStr
	}
	if strings.Contains(str, ".") {
		return nil, fmt.Errorf("should not contain extname, str:%s: %w", str, errContainsExtName)
	}
	number := strings.ToUpper(str) // 默认所有的影片 ID 都转为大写
	rs := &Number{
		numberID:          "",
		isChineseSubtitle: false,
		isMultiCD:         false,
		multiCDIndex:      0,
	}
	// 部分影片 ID 需要进行改写, 改写逻辑提到外面去, number 只做解析用

	// 提取后缀信息并对影片 ID 进行裁剪
	number = resolveSuffixInfo(rs, number)
	rs.numberID = number
	return rs, nil
}

// GetCleanID 将影片 ID 中 `-`, `_` 进行移除。
//
// 注意: 按字节遍历而非 for-range (rune 级) 遍历, 刻意的选择:
// 老实现 "for _, c := range str { sb.WriteRune(c) }" 会把非法 UTF-8 字节
// 悄悄替换成 U+FFFD (3 字节), 导致 "剥掉分隔符" 这个纯 byte-level 的语义
// 附带一层 UTF-8 normalization; 这会让像 media title 这样的可能含非法字节
// 的调用方输出膨胀甚至损坏。`-` / `_` 都是 ASCII, 不存在和 multi-byte rune
// 碰撞的可能, 直接按字节处理最简单也最安全 (已有 FuzzGetCleanID 守护不变量)。
func GetCleanID(str string) string {
	sb := strings.Builder{}
	sb.Grow(len(str))
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c == '-' || c == '_' {
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String()
}
