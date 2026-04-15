package medialib

import (
	"errors"
	"fmt"
	stdimage "image"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	imgutil "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/nfo"
)

var (
	errLibraryItemNotDir            = errors.New("library item is not a directory")
	errLibraryDetailRequired        = errors.New("library detail is required")
	errLibraryVariantNotFound       = errors.New("library variant not found")
	errCoverNotFound                = errors.New("cover not found")
	errCropRectOutOfBounds          = errors.New("crop rectangle out of bounds")
	errOnlyExtrafanartDeletable     = errors.New("only extrafanart files can be deleted")
	errInvalidLibraryPath           = errors.New("invalid library path")
	errExtrafanartFilenameExhausted = errors.New("unable to allocate extrafanart filename")
)

var videoExts = map[string]struct{}{
	".avi": {}, ".flv": {}, ".m2ts": {}, ".m4v": {}, ".mkv": {}, ".mov": {}, ".mp4": {}, ".mpe": {},
	".mpeg": {}, ".mpg": {}, ".mts": {}, ".rm": {}, ".rmvb": {}, ".strm": {}, ".ts": {}, ".wmv": {},
}

var imageExts = map[string]struct{}{
	".avif": {}, ".bmp": {}, ".gif": {}, ".jpeg": {}, ".jpg": {}, ".png": {}, ".webp": {},
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

type dirScanResult struct {
	updatedAt  int64
	hasNFO     bool
	nfoPath    string
	videoCount int
	fileCount  int
	totalSize  int64
	imageNames []string
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

func trimMetaFields(meta *Meta) {
	meta.Title = strings.TrimSpace(meta.Title)
	meta.TitleTranslated = strings.TrimSpace(meta.TitleTranslated)
	meta.OriginalTitle = strings.TrimSpace(meta.OriginalTitle)
	meta.Plot = strings.TrimSpace(meta.Plot)
	meta.PlotTranslated = strings.TrimSpace(meta.PlotTranslated)
	meta.Number = strings.TrimSpace(meta.Number)
	meta.ReleaseDate = strings.TrimSpace(meta.ReleaseDate)
	meta.Studio = strings.TrimSpace(meta.Studio)
	meta.Label = strings.TrimSpace(meta.Label)
	meta.Series = strings.TrimSpace(meta.Series)
	meta.Director = strings.TrimSpace(meta.Director)
	meta.Source = strings.TrimSpace(meta.Source)
	meta.ScrapedAt = strings.TrimSpace(meta.ScrapedAt)
}

func writeVariantNFO(absPath, relPath string, variant Variant, primaryKey string, meta Meta) error {
	mov := &nfo.Movie{}
	nfoPath := selectVariantNFOPath(absPath, variant, primaryKey)
	if variant.NFOAbsPath != "" {
		nfoPath = variant.NFOAbsPath
	}
	if existing, parseErr := nfo.ParseMovie(nfoPath); parseErr == nil {
		mov = existing
	}
	applyMetaToMovie(meta, mov)
	posterValue := firstNonEmpty(
		strings.TrimSpace(mov.Poster),
		preserveAssetValue("", firstNonEmpty(variant.PosterPath, variant.Meta.PosterPath), relPath),
	)
	coverValue := firstNonEmpty(
		strings.TrimSpace(mov.Cover),
		strings.TrimSpace(mov.Fanart),
		strings.TrimSpace(mov.Thumb),
		preserveAssetValue(
			"",
			firstNonEmpty(variant.CoverPath, variant.Meta.CoverPath, variant.Meta.FanartPath, variant.Meta.ThumbPath),
			relPath,
		),
	)
	if posterValue != "" {
		mov.Poster = posterValue
		mov.Art.Poster = posterValue
	}
	if coverValue != "" {
		mov.Cover = coverValue
		mov.Fanart = coverValue
		mov.Thumb = coverValue
	}
	if err := nfo.WriteMovieToFile(nfoPath, mov); err != nil {
		return fmt.Errorf("write nfo file: %w", err)
	}
	return nil
}

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

func (s *Service) replaceRootArtwork(
	root string, detail *Detail, absPath, variantKey, kind, originalName string, data []byte,
) (*Detail, error) {
	if err := validateDirDetail(absPath, detail); err != nil {
		return nil, err
	}
	relPath := detail.Item.RelPath
	if kind == "fanart" {
		return s.writeFanart(root, relPath, absPath, originalName, data)
	}
	variant, ok := pickVariant(detail, variantKey)
	if !ok {
		return nil, errLibraryVariantNotFound
	}
	if err := writeVariantArtwork(absPath, detail, variant, kind, originalName, data); err != nil {
		return nil, err
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

func (s *Service) writeFanart(root, relPath, absPath, originalName string, data []byte) (*Detail, error) {
	targetName, err := pickFanartTargetName(absPath, originalName)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(absPath, filepath.FromSlash(targetName))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, fmt.Errorf("create fanart dir: %w", err)
	}
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write fanart file: %w", err)
	}
	return s.readRootDetail(root, relPath, absPath)
}

func writeVariantArtwork(
	absPath string, detail *Detail, variant Variant, kind, originalName string, data []byte,
) error {
	mov := &nfo.Movie{}
	nfoPath := selectVariantNFOPath(absPath, variant, detail.PrimaryVariantKey)
	if variant.NFOAbsPath != "" {
		nfoPath = variant.NFOAbsPath
	}
	if existing, parseErr := nfo.ParseMovie(nfoPath); parseErr == nil {
		mov = existing
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if _, ok := imageExts[ext]; !ok {
		ext = ".jpg"
	}
	targetName := pickArtworkTargetName(detail, variant, kind, ext)
	targetPath := filepath.Join(absPath, filepath.FromSlash(targetName))
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return fmt.Errorf("write artwork file: %w", err)
	}
	switch kind {
	case "poster":
		mov.Poster = targetName
		mov.Art.Poster = targetName
	case "cover":
		mov.Cover = targetName
		mov.Fanart = targetName
		mov.Thumb = targetName
	}
	if err := nfo.WriteMovieToFile(nfoPath, mov); err != nil {
		return fmt.Errorf("write variant nfo: %w", err)
	}
	return nil
}

func resolveCoverForCrop(detail *Detail, variant Variant, absPath string) ([]byte, string, error) {
	coverPath := firstNonEmpty(
		variant.CoverPath,
		variant.Meta.CoverPath,
		variant.Meta.FanartPath,
		variant.Meta.ThumbPath,
		detail.Meta.CoverPath,
		detail.Meta.FanartPath,
		detail.Meta.ThumbPath,
	)
	if coverPath == "" {
		return nil, "", errCoverNotFound
	}
	coverRelPath := strings.TrimPrefix(filepath.ToSlash(coverPath), detail.Item.RelPath+"/")
	coverAbsPath := filepath.Join(absPath, filepath.FromSlash(coverRelPath))
	raw, err := os.ReadFile(coverAbsPath)
	if err != nil {
		return nil, "", fmt.Errorf("read cover failed: %w", err)
	}
	return raw, coverRelPath, nil
}

func cropImageRect(raw []byte, x, y, width, height int) ([]byte, error) {
	img, err := imgutil.LoadImage(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cover failed: %w", err)
	}
	rect := stdimage.Rect(x, y, x+width, y+height)
	bounds := img.Bounds()
	if rect.Min.X < bounds.Min.X || rect.Min.Y < bounds.Min.Y || rect.Max.X > bounds.Max.X || rect.Max.Y > bounds.Max.Y {
		return nil, errCropRectOutOfBounds
	}
	cropped, err := imgutil.CutImageViaRectangle(img, rect)
	if err != nil {
		return nil, fmt.Errorf("crop poster failed: %w", err)
	}
	result, err := imgutil.WriteImageToBytes(cropped)
	if err != nil {
		return nil, fmt.Errorf("encode cropped image: %w", err)
	}
	return result, nil
}

func (s *Service) cropRootPosterFromCover(
	root string, detail *Detail, absPath, variantKey string, x, y, width, height int,
) (*Detail, error) {
	if err := validateDirDetail(absPath, detail); err != nil {
		return nil, err
	}
	variant, ok := pickVariant(detail, variantKey)
	if !ok {
		return nil, errLibraryVariantNotFound
	}
	raw, coverRelPath, err := resolveCoverForCrop(detail, variant, absPath)
	if err != nil {
		return nil, err
	}
	croppedRaw, err := cropImageRect(raw, x, y, width, height)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(coverRelPath))
	if _, ok := imageExts[ext]; !ok {
		ext = ".jpg"
	}
	targetName := pickArtworkTargetName(detail, variant, "poster", ext)
	if strings.EqualFold(filepath.ToSlash(targetName), coverRelPath) {
		targetName = fmt.Sprintf("%s-poster%s", firstNonEmpty(variant.BaseName, detail.Item.Number, detail.Item.Name), ext)
	}
	targetPath := filepath.Join(absPath, filepath.FromSlash(targetName))

	if err := os.WriteFile(targetPath, croppedRaw, 0o600); err != nil {
		return nil, fmt.Errorf("write poster failed: %w", err)
	}
	if err := updatePosterInNFO(absPath, variant, detail.PrimaryVariantKey, targetName); err != nil {
		return nil, err
	}
	return s.readRootDetail(root, detail.Item.RelPath, absPath)
}

func updatePosterInNFO(absPath string, variant Variant, primaryKey, targetName string) error {
	mov := &nfo.Movie{}
	nfoPath := selectVariantNFOPath(absPath, variant, primaryKey)
	if variant.NFOAbsPath != "" {
		nfoPath = variant.NFOAbsPath
	}
	if existing, parseErr := nfo.ParseMovie(nfoPath); parseErr == nil {
		mov = existing
	}
	mov.Poster = targetName
	mov.Art.Poster = targetName
	if err := nfo.WriteMovieToFile(nfoPath, mov); err != nil {
		return fmt.Errorf("write nfo after poster crop: %w", err)
	}
	return nil
}

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
	sort.Slice(matchKeys, func(i, j int) bool {
		if len(matchKeys[i]) == len(matchKeys[j]) {
			return matchKeys[i] < matchKeys[j]
		}
		return len(matchKeys[i]) > len(matchKeys[j])
	})
	matchImageFilesToVariants(topFiles, matchKeys, variantsByKey)
	return finalizeVariants(variantsByKey, keys, primaryKey, root, relPath, filepath.Base(absPath)), primaryKey, nil
}

type variantTopFile struct {
	name    string
	stem    string
	ext     string
	relPath string
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

func cloneMeta(meta Meta) Meta {
	return Meta{
		Title:           meta.Title,
		TitleTranslated: meta.TitleTranslated,
		OriginalTitle:   meta.OriginalTitle,
		Plot:            meta.Plot,
		PlotTranslated:  meta.PlotTranslated,
		Number:          meta.Number,
		ReleaseDate:     meta.ReleaseDate,
		Runtime:         meta.Runtime,
		Studio:          meta.Studio,
		Label:           meta.Label,
		Series:          meta.Series,
		Director:        meta.Director,
		Actors:          append([]string(nil), meta.Actors...),
		Genres:          append([]string(nil), meta.Genres...),
		PosterPath:      meta.PosterPath,
		CoverPath:       meta.CoverPath,
		FanartPath:      meta.FanartPath,
		ThumbPath:       meta.ThumbPath,
		Source:          meta.Source,
		ScrapedAt:       meta.ScrapedAt,
	}
}

func libraryMetaFromMovie(root, relPath string, mov *nfo.Movie) Meta {
	coverRaw := firstNonEmpty(strings.TrimSpace(mov.Cover), strings.TrimSpace(mov.Fanart), strings.TrimSpace(mov.Thumb))
	if coverRaw == "" && len(mov.Art.Fanart) > 0 {
		coverRaw = strings.TrimSpace(mov.Art.Fanart[0])
	}
	originalTitle := firstNonEmpty(strings.TrimSpace(mov.OriginalTitle), strings.TrimSpace(mov.Title))
	titleTranslated := strings.TrimSpace(mov.TitleTranslated)
	if titleTranslated == "" &&
		strings.TrimSpace(mov.OriginalTitle) != "" &&
		strings.TrimSpace(mov.Title) != "" &&
		strings.TrimSpace(mov.Title) != strings.TrimSpace(mov.OriginalTitle) {
		titleTranslated = strings.TrimSpace(mov.Title)
	}
	plot, plotTranslated := splitPlot(strings.TrimSpace(mov.Plot), strings.TrimSpace(mov.PlotTranslated))
	return Meta{
		Title:           originalTitle,
		TitleTranslated: titleTranslated,
		OriginalTitle:   originalTitle,
		Plot:            plot,
		PlotTranslated:  plotTranslated,
		Number:          strings.TrimSpace(mov.ID),
		ReleaseDate:     strings.TrimSpace(firstNonEmpty(mov.ReleaseDate, mov.Premiered, mov.Release)),
		Runtime:         mov.Runtime,
		Studio:          strings.TrimSpace(mov.Studio),
		Label:           strings.TrimSpace(mov.Label),
		Series:          strings.TrimSpace(mov.Set),
		Director:        strings.TrimSpace(mov.Director),
		Actors:          actorNames(mov.Actors),
		Genres:          trimStrings(mov.Genres),
		PosterPath: firstNonEmpty(
			resolveMovieAssetPath(root, relPath, mov.Poster),
			resolveMovieAssetPath(root, relPath, mov.Art.Poster),
		),
		CoverPath:  resolveMovieAssetPath(root, relPath, coverRaw),
		FanartPath: resolveMovieAssetPath(root, relPath, firstNonEmpty(strings.TrimSpace(mov.Fanart), coverRaw)),
		ThumbPath:  resolveMovieAssetPath(root, relPath, firstNonEmpty(strings.TrimSpace(mov.Thumb), coverRaw)),
		Source:     strings.TrimSpace(mov.ScrapeInfo.Source),
		ScrapedAt:  strings.TrimSpace(mov.ScrapeInfo.Date),
	}
}

func applyMetaToMovie(meta Meta, mov *nfo.Movie) {
	baseTitle := firstNonEmpty(meta.Title, meta.OriginalTitle)
	mov.Title = firstNonEmpty(meta.TitleTranslated, baseTitle)
	mov.OriginalTitle = baseTitle
	mov.TitleTranslated = meta.TitleTranslated
	mov.SortTitle = firstNonEmpty(mov.SortTitle, mov.OriginalTitle, mov.Title)
	mov.Plot = meta.Plot
	mov.PlotTranslated = meta.PlotTranslated
	mov.ID = meta.Number
	mov.ReleaseDate = meta.ReleaseDate
	mov.Premiered = meta.ReleaseDate
	mov.Release = meta.ReleaseDate
	mov.Runtime = meta.Runtime
	mov.Studio = meta.Studio
	mov.Maker = firstNonEmpty(meta.Studio, mov.Maker)
	mov.Label = meta.Label
	mov.Set = meta.Series
	mov.Director = meta.Director
	mov.Genres = trimStrings(meta.Genres)
	mov.Tags = trimStrings(meta.Genres)
	mov.Actors = makeActors(meta.Actors)
	mov.ScrapeInfo.Source = firstNonEmpty(meta.Source, mov.ScrapeInfo.Source)
	mov.ScrapeInfo.Date = firstNonEmpty(meta.ScrapedAt, mov.ScrapeInfo.Date, time.Now().Format(time.DateTime))
	if meta.ReleaseDate != "" {
		if year, err := strconv.Atoi(meta.ReleaseDate[:min(4, len(meta.ReleaseDate))]); err == nil {
			mov.Year = year
		}
	}
}

func splitPlot(plot, plotTranslated string) (string, string) {
	if strings.TrimSpace(plotTranslated) != "" {
		return strings.TrimSpace(plot), strings.TrimSpace(plotTranslated)
	}
	const marker = " [翻译:"
	idx := strings.LastIndex(plot, marker)
	if idx < 0 || !strings.HasSuffix(plot, "]") {
		return strings.TrimSpace(plot), ""
	}
	base := strings.TrimSpace(plot[:idx])
	translated := strings.TrimSuffix(strings.TrimSpace(plot[idx+len(marker):]), "]")
	return base, strings.TrimSpace(translated)
}

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

func actorNames(items []nfo.Actor) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func makeActors(names []string) []nfo.Actor {
	actors := make([]nfo.Actor, 0, len(names))
	for _, name := range trimStrings(names) {
		actors = append(actors, nfo.Actor{Name: name})
	}
	return actors
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

func pickArtworkTargetName(detail *Detail, variant Variant, kind, ext string) string {
	currentPath := ""
	if kind == "poster" {
		currentPath = firstNonEmpty(variant.PosterPath, variant.Meta.PosterPath)
	} else {
		currentPath = firstNonEmpty(
			variant.CoverPath,
			variant.Meta.CoverPath,
			variant.Meta.FanartPath,
			variant.Meta.ThumbPath,
		)
	}
	if currentPath == "" && detail != nil {
		if kind == "poster" {
			currentPath = detail.Meta.PosterPath
		} else {
			currentPath = firstNonEmpty(detail.Meta.CoverPath, detail.Meta.FanartPath, detail.Meta.ThumbPath)
		}
	}
	if currentPath != "" {
		prefix := detail.Item.RelPath + "/"
		if strings.HasPrefix(currentPath, prefix) {
			return strings.TrimPrefix(currentPath, prefix)
		}
		return filepath.Base(currentPath)
	}
	base := variant.BaseName
	if detail != nil {
		base = firstNonEmpty(base, detail.Item.Number, detail.Item.Name)
	}
	if kind == "poster" {
		return fmt.Sprintf("%s-poster%s", base, ext)
	}
	return fmt.Sprintf("%s-fanart%s", base, ext)
}

func pickFanartTargetName(absPath, originalName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if _, ok := imageExts[ext]; !ok {
		ext = ".jpg"
	}
	base := strings.TrimSpace(strings.TrimSuffix(filepath.Base(originalName), filepath.Ext(originalName)))
	base = strings.ReplaceAll(base, "/", "-")
	base = strings.ReplaceAll(base, "\\", "-")
	if base == "" || base == "." {
		base = "fanart"
	}
	dirRelPath := "extrafanart"
	for index := 0; index < 1000; index++ {
		name := base
		if index > 0 {
			name = fmt.Sprintf("%s-%d", base, index+1)
		}
		relPath := filepath.ToSlash(filepath.Join(dirRelPath, name+ext))
		if _, err := os.Stat(filepath.Join(absPath, filepath.FromSlash(relPath))); os.IsNotExist(err) {
			return relPath, nil
		} else if err != nil {
			return "", fmt.Errorf("stat fanart candidate: %w", err)
		}
	}
	return "", errExtrafanartFilenameExhausted
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
