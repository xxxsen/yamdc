package medialib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 写操作 + 路径解析 + 杂项字符串工具:
//   - updateRootItem:      把 Meta 变更落盘到所有 variant 的 NFO
//   - deleteRootFile:      仅允许删除 extrafanart 下的文件
//   - validateDirDetail:   写操作前的基础校验
//   - resolveRootPath:     rel→abs 路径并防越界
//   - resolveMovieAssetPath / preserveAssetValue: NFO 资源路径规范化
//   - trimStrings / firstNonEmpty: 小工具

func (s *Service) updateRootItem(root string, detail *Detail, absPath string, meta Meta) (*Detail, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat item dir: %w", err)
	}
	if !info.IsDir() {
		return nil, errLibraryItemNotDir
	}
	trimMetaFields(&meta)
	if detail == nil {
		return nil, errLibraryDetailRequired
	}
	relPath := detail.Item.RelPath
	variants := detail.Variants
	if len(variants) == 0 {
		variants = []Variant{{
			Key:       detail.PrimaryVariantKey,
			BaseName:  firstNonEmpty(detail.PrimaryVariantKey, detail.Item.Number, detail.Item.Name),
			IsPrimary: true,
		}}
	}
	for _, variant := range variants {
		if err := writeVariantNFO(absPath, relPath, variant, detail.PrimaryVariantKey, meta); err != nil {
			return nil, fmt.Errorf("write nfo file: %w", err)
		}
	}
	return s.readRootDetail(root, relPath, absPath)
}

func validateDirDetail(absPath string, detail *Detail) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat item dir: %w", err)
	}
	if !info.IsDir() {
		return errLibraryItemNotDir
	}
	if detail == nil {
		return errLibraryDetailRequired
	}
	return nil
}

// deleteRootFile 出于安全考虑只允许删除 item 目录下 extrafanart/ 里的文件,
// 避免前端误删主 video / poster / nfo。
func (s *Service) deleteRootFile(root, itemRelPath, fileRelPath string) (*Detail, error) {
	relPath, absPath, err := s.resolveRootPath(root, fileRelPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return nil, os.ErrNotExist
	}
	itemPrefix := strings.TrimSuffix(itemRelPath, "/") + "/extrafanart/"
	if !strings.HasPrefix(relPath, itemPrefix) {
		return nil, errOnlyExtrafanartDeletable
	}
	if err := os.Remove(absPath); err != nil {
		return nil, fmt.Errorf("remove file: %w", err)
	}
	itemAbsPath := filepath.Join(root, filepath.FromSlash(itemRelPath))
	return s.readRootDetail(root, itemRelPath, itemAbsPath)
}

// resolveRootPath 把外部传入的相对路径解析成 (cleanedRel, abs),
// 并防御 "../" 越界访问。返回的 rel 使用 slash 分隔、不含前导 "/"。
func (s *Service) resolveRootPath(root, raw string) (string, string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(raw) == "" {
		return "", "", errInvalidLibraryPath
	}
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(raw)))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", "", errInvalidLibraryPath
	}
	absPath := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", "", fmt.Errorf("compute relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") || rel == ".." {
		return "", "", errInvalidLibraryPath
	}
	return rel, absPath, nil
}

// resolveMovieAssetPath 把 NFO 里写的 poster/cover 路径规范化为 library root 下的相对路径,
// raw 如果跑到 root 外部或非法会返回空串。
func resolveMovieAssetPath(root, relDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(raw, "/")))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") {
		return ""
	}
	absPath := filepath.Join(root, filepath.FromSlash(relDir), filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// preserveAssetValue 为 NFO 字段计算一个"尽量稳定"的写回值:
// 若当前已经有值直接复用; 否则用 relPath 相对 relDir 的部分, 尽量不写绝对路径。
func preserveAssetValue(current, relPath, relDir string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	if relPath == "" {
		return ""
	}
	prefix := relDir + "/"
	if strings.HasPrefix(relPath, prefix) {
		return strings.TrimPrefix(relPath, prefix)
	}
	return filepath.Base(relPath)
}

func trimStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
