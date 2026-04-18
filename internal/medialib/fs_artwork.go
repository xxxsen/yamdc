package medialib

import (
	"fmt"
	stdimage "image"
	"os"
	"path/filepath"
	"strings"

	imgutil "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/nfo"
)

// 海报 / fanart / cover 等 artwork 相关的读写:
//   - 替换 variant artwork (poster / cover)
//   - 追加 extrafanart
//   - 从 cover 裁剪 poster
//   - artwork 目标文件名生成

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
	// 防止 poster 目标文件名与 cover 完全一致导致原 cover 被覆盖。
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

// pickArtworkTargetName 为新上传/裁剪出的 artwork 选目标文件名,
// 尽量沿用当前 variant 已用过的文件名,以避免产生 orphan 旧图。
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

// pickFanartTargetName 为 extrafanart 目录下新上传的 fanart 找到空闲的文件名,
// 在最坏情况下会尝试 "base"、"base-2" … "base-1000", 超出上限视为分配失败。
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
