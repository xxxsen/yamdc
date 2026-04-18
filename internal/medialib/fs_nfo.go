package medialib

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/nfo"
)

// NFO 文件读写与 Meta 映射:
//   - Meta ⇄ nfo.Movie 的字段映射
//   - 写入 variant NFO (含封面/海报字段保留逻辑)
//   - 辅助: trim / clone / plot 拆分 / actor 转换

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

// writeVariantNFO 把 meta 写入 variant 对应的 NFO 文件。
// 为避免覆盖已有 poster/cover 字段, 先尝试解析老 NFO 再合并。
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

// libraryMetaFromMovie 把 NFO 里的 Movie 反映射成内部 Meta,
// 尽量保留翻译字段、兼容不同 NFO 写入习惯的 poster/fanart/cover 语义。
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

// splitPlot 兼容旧版 NFO 的两种 plot 存储格式:
//  1. 新: plot + plotTranslated 两个字段
//  2. 旧: 全部塞进 plot, 翻译用 " [翻译:xxx]" 后缀
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
