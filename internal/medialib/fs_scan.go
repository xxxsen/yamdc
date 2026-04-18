package medialib

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxxsen/yamdc/internal/nfo"
)

// 媒体库目录扫描相关逻辑。
//
// 本文件负责"从磁盘读什么" —— 把 library root 下的目录扫成 Item / Detail,
// 不涉及写操作、也不涉及 variant 的完整组装 (那部分在 fs_variant.go)。

// dirScanResult 汇总一次 ReadDir 的扫描结果,用于避免重复 stat/遍历。
type dirScanResult struct {
	updatedAt  int64
	hasNFO     bool
	nfoPath    string
	videoCount int
	fileCount  int
	totalSize  int64
	imageNames []string
}

func (s *Service) listRootItemDirs(root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return []string{}, nil
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("stat library root: %w", err)
	}
	dirs := make([]string, 0, 32)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), "extrafanart") {
			return filepath.SkipDir
		}
		_, ok, err := s.inspectRootDir(root, path)
		if err != nil {
			return err
		}
		if ok {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk library root: %w", err)
	}
	sort.Strings(dirs)
	return dirs, nil
}

func scanDirEntries(absPath string, entries []os.DirEntry, baseUpdatedAt int64) dirScanResult {
	r := dirScanResult{
		updatedAt:  baseUpdatedAt,
		imageNames: make([]string, 0, 6),
	}
	for _, entry := range entries {
		entryInfo, err := entry.Info()
		if err == nil && entryInfo.ModTime().UnixMilli() > r.updatedAt {
			r.updatedAt = entryInfo.ModTime().UnixMilli()
		}
		if entry.IsDir() {
			continue
		}
		r.fileCount++
		if entryInfo != nil {
			r.totalSize += entryInfo.Size()
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".nfo" && !r.hasNFO {
			r.hasNFO = true
			r.nfoPath = filepath.Join(absPath, name)
		}
		if _, ok := videoExts[ext]; ok {
			r.videoCount++
		}
		if _, ok := imageExts[ext]; ok {
			r.imageNames = append(r.imageNames, name)
		}
	}
	return r
}

// applyVariantMetaToItem 把主 variant 的 meta (或 fallback 到 NFO 解析结果)
// 回填到 Item 上,用于列表页展示。
func applyVariantMetaToItem(item *Item, variants []Variant, primaryKey, root, relPath string, scan dirScanResult) {
	if primary, ok := findVariant(variants, primaryKey); ok {
		item.Title = firstNonEmpty(primary.Meta.TitleTranslated, primary.Meta.Title, primary.Meta.OriginalTitle, item.Title)
		item.Number = firstNonEmpty(primary.Meta.Number, item.Number)
		item.ReleaseDate = firstNonEmpty(primary.Meta.ReleaseDate, item.ReleaseDate)
		if len(primary.Meta.Actors) > 0 {
			item.Actors = append([]string(nil), primary.Meta.Actors...)
		}
		item.PosterPath = firstNonEmpty(primary.PosterPath, primary.Meta.PosterPath, item.PosterPath)
		item.CoverPath = firstNonEmpty(
			primary.CoverPath, primary.Meta.CoverPath, primary.Meta.FanartPath, primary.Meta.ThumbPath, item.CoverPath)
		item.HasNFO = item.HasNFO || primary.NFOPath != ""
	} else if scan.hasNFO {
		mov, err := nfo.ParseMovie(scan.nfoPath)
		if err == nil {
			meta := libraryMetaFromMovie(root, relPath, mov)
			item.Title = firstNonEmpty(meta.TitleTranslated, meta.Title, meta.OriginalTitle, item.Title)
			item.Number = meta.Number
			item.ReleaseDate = meta.ReleaseDate
			item.Actors = append([]string(nil), meta.Actors...)
			item.PosterPath = firstNonEmpty(meta.PosterPath, item.PosterPath)
			item.CoverPath = firstNonEmpty(meta.CoverPath, meta.FanartPath, meta.ThumbPath, item.CoverPath)
		}
	}
	if item.PosterPath == "" {
		item.PosterPath = detectArtworkPath(relPath, scan.imageNames, "poster")
	}
	if item.CoverPath == "" {
		item.CoverPath = detectArtworkPath(relPath, scan.imageNames, "fanart")
	}
}

func (s *Service) inspectRootDir(root, absPath string) (Item, bool, error) {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return Item{}, false, fmt.Errorf("read dir %s: %w", absPath, err)
	}
	relPath, err := filepath.Rel(root, absPath)
	if err != nil {
		return Item{}, false, fmt.Errorf("compute relative path: %w", err)
	}
	relPath = filepath.ToSlash(relPath)
	info, err := os.Stat(absPath)
	if err != nil {
		return Item{}, false, fmt.Errorf("stat dir %s: %w", absPath, err)
	}
	scan := scanDirEntries(absPath, entries, info.ModTime().UnixMilli())
	if !scan.hasNFO && scan.videoCount == 0 {
		return Item{}, false, nil
	}
	item := Item{
		RelPath:    relPath,
		Name:       filepath.Base(absPath),
		Title:      filepath.Base(absPath),
		UpdatedAt:  scan.updatedAt,
		HasNFO:     scan.hasNFO,
		TotalSize:  scan.totalSize,
		FileCount:  scan.fileCount,
		VideoCount: scan.videoCount,
	}
	variants, primaryKey, err := s.scanRootVariants(root, relPath, absPath)
	if err != nil {
		return Item{}, false, err
	}
	item.VariantCount = len(variants)
	applyVariantMetaToItem(&item, variants, primaryKey, root, relPath, scan)
	return item, true, nil
}

func (s *Service) readRootDetail(root, relPath, absPath string) (*Detail, error) {
	item, ok, err := s.inspectRootDir(root, absPath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, os.ErrNotExist
	}
	variants, primaryKey, err := s.scanRootVariants(root, relPath, absPath)
	if err != nil {
		return nil, err
	}
	files, err := s.listRootFiles(root, absPath)
	if err != nil {
		return nil, err
	}
	variants, files = attachFilesToVariants(variants, files)
	meta := Meta{
		Title:       item.Title,
		Number:      item.Number,
		ReleaseDate: item.ReleaseDate,
		Actors:      append([]string(nil), item.Actors...),
		PosterPath:  item.PosterPath,
		CoverPath:   item.CoverPath,
	}
	if primary, ok := findVariant(variants, primaryKey); ok {
		meta = cloneMeta(primary.Meta)
		meta.PosterPath = firstNonEmpty(primary.PosterPath, meta.PosterPath, item.PosterPath)
		meta.CoverPath = firstNonEmpty(primary.CoverPath, meta.CoverPath, meta.FanartPath, meta.ThumbPath, item.CoverPath)
		meta.FanartPath = firstNonEmpty(meta.FanartPath, primary.CoverPath, item.CoverPath)
		meta.ThumbPath = firstNonEmpty(meta.ThumbPath, meta.CoverPath, primary.CoverPath)
	}
	return &Detail{
		Item:              item,
		Meta:              meta,
		Variants:          variants,
		PrimaryVariantKey: primaryKey,
		Files:             files,
	}, nil
}

func (s *Service) listRootFiles(root, absPath string) ([]FileItem, error) {
	files := make([]FileItem, 0, 16)
	err := filepath.WalkDir(absPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get file info: %w", err)
		}
		fileRelPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("compute file relative path: %w", err)
		}
		files = append(files, FileItem{
			Name:      filepath.ToSlash(strings.TrimPrefix(path, absPath+string(filepath.Separator))),
			RelPath:   filepath.ToSlash(fileRelPath),
			Kind:      detectFileKind(entry.Name()),
			Size:      info.Size(),
			UpdatedAt: info.ModTime().UnixMilli(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk root files: %w", err)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Kind == files[j].Kind {
			return files[i].Name < files[j].Name
		}
		return files[i].Kind < files[j].Kind
	})
	return files, nil
}

// detectArtworkPath 根据文件名启发式挑一张 poster/fanart,用于目录没带 NFO 时的兜底展示。
func detectArtworkPath(relDir string, imageNames []string, kind string) string {
	for _, name := range imageNames {
		lower := strings.ToLower(name)
		if kind == "poster" && strings.Contains(lower, "poster") {
			return filepath.ToSlash(filepath.Join(relDir, name))
		}
		if kind == "fanart" && (strings.Contains(lower, "fanart") || strings.Contains(lower, "cover")) {
			return filepath.ToSlash(filepath.Join(relDir, name))
		}
	}
	if len(imageNames) == 0 {
		return ""
	}
	return filepath.ToSlash(filepath.Join(relDir, imageNames[0]))
}

func detectFileKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".nfo"):
		return "nfo"
	case strings.Contains(lower, "poster"):
		return "poster"
	case strings.Contains(lower, "fanart") || strings.Contains(lower, "cover") || strings.Contains(lower, "thumb"):
		return "cover"
	}
	ext := strings.ToLower(filepath.Ext(lower))
	if _, ok := videoExts[ext]; ok {
		return "video"
	}
	if _, ok := imageExts[ext]; ok {
		return "image"
	}
	return "file"
}
