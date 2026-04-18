package medialib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxxsen/yamdc/internal/nfo"
)

// Variant 组装与选择:
//   - 根据目录下的 video/nfo/image 文件拼装 variant 列表
//   - 判定主 variant、生成后缀与展示 label
//   - 把 FileItem 列表挂回对应 variant
//
// 一个"主 variant"对应目录与其同名 NFO/poster/fanart,
// 其他派生 variant (如多文件分版本) 通过 BaseName 前缀做归属。

// variantTopFile 记录目录顶层文件的关键字段,避免在不同函数间重复切分扩展名。
type variantTopFile struct {
	name    string
	stem    string
	ext     string
	relPath string
}

func (s *Service) scanRootVariants(root, relPath, absPath string) ([]Variant, string, error) {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("read dir for variants: %w", err)
	}
	variantsByKey, keys, topFiles := collectVariantEntries(entries, relPath, absPath)
	if len(keys) == 0 {
		return nil, "", nil
	}
	sort.Strings(keys)
	primaryKey := selectPrimaryVariant(keys, filepath.Base(absPath))
	matchKeys := append([]string(nil), keys...)
	// 图片匹配时, 前缀越长越具体, 所以按长度倒序,
	// 避免 "NUMBER" 的 poster 被 "NUMBER-cd1" 错误吞并。
	sort.Slice(matchKeys, func(i, j int) bool {
		if len(matchKeys[i]) == len(matchKeys[j]) {
			return matchKeys[i] < matchKeys[j]
		}
		return len(matchKeys[i]) > len(matchKeys[j])
	})
	matchImageFilesToVariants(topFiles, matchKeys, variantsByKey)
	return finalizeVariants(variantsByKey, keys, primaryKey, root, relPath, filepath.Base(absPath)), primaryKey, nil
}

func collectVariantEntries(
	entries []os.DirEntry,
	relPath, absPath string,
) (map[string]*Variant, []string, []variantTopFile) {
	variantsByKey := make(map[string]*Variant)
	keys := make([]string, 0, 8)
	topFiles := make([]variantTopFile, 0, len(entries))
	ensureVariant := func(key string) *Variant {
		if current, ok := variantsByKey[key]; ok {
			return current
		}
		current := &Variant{Key: key, BaseName: key, Meta: Meta{Number: key}}
		variantsByKey[key] = current
		keys = append(keys, key)
		return current
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		topFiles = append(topFiles,
			variantTopFile{name: name, stem: stem, ext: ext, relPath: filepath.ToSlash(filepath.Join(relPath, name))})
		if _, ok := videoExts[ext]; ok {
			ensureVariant(stem).VideoPath = filepath.ToSlash(filepath.Join(relPath, name))
			continue
		}
		if ext == ".nfo" {
			v := ensureVariant(stem)
			v.NFOPath = filepath.ToSlash(filepath.Join(relPath, name))
			v.NFOAbsPath = filepath.Join(absPath, name)
		}
	}
	return variantsByKey, keys, topFiles
}

func finalizeVariants(
	variantsByKey map[string]*Variant, keys []string, primaryKey, root, relPath, dirBase string,
) []Variant {
	variants := make([]Variant, 0, len(keys))
	for _, key := range keys {
		variant := cloneVariant(variantsByKey[key])
		variant.IsPrimary = key == primaryKey
		variant.Suffix = variantSuffix(key, primaryKey, dirBase)
		variant.Label = variantLabel(variant.Suffix, variant.IsPrimary)
		populateVariantFromNFO(&variant, root, relPath)
		variant.Meta.Number = firstNonEmpty(variant.Meta.Number, variant.BaseName)
		variant.Meta.PosterPath = firstNonEmpty(variant.Meta.PosterPath, variant.PosterPath)
		variant.Meta.CoverPath = firstNonEmpty(
			variant.Meta.CoverPath, variant.CoverPath, variant.Meta.FanartPath, variant.Meta.ThumbPath)
		variant.Meta.FanartPath = firstNonEmpty(variant.Meta.FanartPath, variant.CoverPath, variant.Meta.CoverPath)
		variant.Meta.ThumbPath = firstNonEmpty(variant.Meta.ThumbPath, variant.Meta.CoverPath, variant.CoverPath)
		variants = append(variants, variant)
	}
	sort.Slice(variants, func(i, j int) bool {
		if variants[i].IsPrimary != variants[j].IsPrimary {
			return variants[i].IsPrimary
		}
		if len(variants[i].Suffix) == len(variants[j].Suffix) {
			return variants[i].BaseName < variants[j].BaseName
		}
		return len(variants[i].Suffix) < len(variants[j].Suffix)
	})
	return variants
}

func matchImageFilesToVariants(topFiles []variantTopFile, matchKeys []string, variantsByKey map[string]*Variant) {
	for _, file := range topFiles {
		if _, ok := imageExts[file.ext]; !ok {
			continue
		}
		assignImageToVariant(file.stem, file.relPath, matchKeys, variantsByKey)
	}
}

func assignImageToVariant(stem, relPath string, matchKeys []string, variantsByKey map[string]*Variant) {
	for _, key := range matchKeys {
		variant := variantsByKey[key]
		switch stem {
		case key + "-poster":
			variant.PosterPath = relPath
		case key + "-fanart", key + "-cover", key + "-thumb":
			if variant.CoverPath == "" || strings.HasSuffix(stem, "-fanart") || strings.HasSuffix(stem, "-cover") {
				variant.CoverPath = relPath
			}
		default:
			continue
		}
		return
	}
}

func populateVariantFromNFO(variant *Variant, root, relPath string) {
	if variant.NFOAbsPath == "" {
		return
	}
	mov, err := nfo.ParseMovie(variant.NFOAbsPath)
	if err != nil {
		return
	}
	variant.Meta = libraryMetaFromMovie(root, relPath, mov)
	if variant.PosterPath == "" {
		variant.PosterPath = firstNonEmpty(variant.Meta.PosterPath)
	}
	if variant.CoverPath == "" {
		variant.CoverPath = firstNonEmpty(variant.Meta.CoverPath, variant.Meta.FanartPath, variant.Meta.ThumbPath)
	}
}

// attachFilesToVariants 把扫到的 FileItem 列表挂到对应 variant 上,
// 同时给每个 file 标记 variant key/label,便于前端展示。
func attachFilesToVariants(variants []Variant, files []FileItem) ([]Variant, []FileItem) {
	pathToVariant := make(map[string]struct {
		key   string
		label string
	})
	for _, variant := range variants {
		for _, relPath := range []string{variant.VideoPath, variant.NFOPath, variant.PosterPath, variant.CoverPath} {
			if relPath == "" {
				continue
			}
			pathToVariant[relPath] = struct {
				key   string
				label string
			}{key: variant.Key, label: variant.Label}
		}
	}
	variantFiles := make(map[string][]FileItem, len(variants))
	for index := range files {
		if mapping, ok := pathToVariant[files[index].RelPath]; ok {
			files[index].VariantKey = mapping.key
			files[index].VariantLabel = mapping.label
			variantFiles[mapping.key] = append(variantFiles[mapping.key], files[index])
		}
	}
	for index := range variants {
		variants[index].Files = append([]FileItem(nil), variantFiles[variants[index].Key]...)
		variants[index].FileCount = len(variants[index].Files)
	}
	return variants, files
}

func findVariant(variants []Variant, key string) (Variant, bool) {
	for _, variant := range variants {
		if variant.Key == key {
			return variant, true
		}
	}
	return Variant{}, false
}

// pickVariant 选择要操作的 variant: 优先用显式 key, 其次主 variant, 最后兜底第一个。
func pickVariant(detail *Detail, key string) (Variant, bool) {
	if detail == nil {
		return Variant{}, false
	}
	if strings.TrimSpace(key) != "" {
		if variant, ok := findVariant(detail.Variants, key); ok {
			return variant, true
		}
	}
	if variant, ok := findVariant(detail.Variants, detail.PrimaryVariantKey); ok {
		return variant, true
	}
	if len(detail.Variants) == 0 {
		return Variant{}, false
	}
	return detail.Variants[0], true
}

func cloneVariant(src *Variant) Variant {
	if src == nil {
		return Variant{}
	}
	out := *src
	out.Meta = cloneMeta(src.Meta)
	out.Files = append([]FileItem(nil), src.Files...)
	return out
}

// selectPrimaryVariant 挑选主 variant:
//  1. 与目录同名 → 主
//  2. 否则按 (key 长度短, 字典序小) 作为主
func selectPrimaryVariant(keys []string, dirBase string) string {
	if len(keys) == 0 {
		return ""
	}
	for _, key := range keys {
		if strings.EqualFold(key, dirBase) {
			return key
		}
	}
	primary := keys[0]
	for _, key := range keys[1:] {
		if len(key) < len(primary) || (len(key) == len(primary) && key < primary) {
			primary = key
		}
	}
	return primary
}

func variantSuffix(key, primaryKey, dirBase string) string {
	switch {
	case primaryKey != "" && strings.HasPrefix(key, primaryKey+"-"):
		return strings.TrimPrefix(key, primaryKey+"-")
	case dirBase != "" && strings.HasPrefix(key, dirBase+"-"):
		return strings.TrimPrefix(key, dirBase+"-")
	case key == primaryKey || key == dirBase:
		return ""
	default:
		return key
	}
}

func variantLabel(suffix string, isPrimary bool) string {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" && isPrimary {
		return "原始文件"
	}
	if suffix == "" {
		return "实例"
	}
	return strings.ToUpper(suffix)
}

func selectVariantNFOPath(absPath string, variant Variant, primaryKey string) string {
	if variant.NFOAbsPath != "" {
		return variant.NFOAbsPath
	}
	if strings.TrimSpace(variant.BaseName) != "" {
		return filepath.Join(absPath, variant.BaseName+".nfo")
	}
	if strings.TrimSpace(primaryKey) != "" {
		return filepath.Join(absPath, primaryKey+".nfo")
	}
	return filepath.Join(absPath, filepath.Base(absPath)+".nfo")
}
