package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/medialib"
	"go.uber.org/zap"
)

type libraryListItem struct {
	RelPath      string   `json:"rel_path"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Number       string   `json:"number"`
	ReleaseDate  string   `json:"release_date"`
	Actors       []string `json:"actors"`
	UpdatedAt    int64    `json:"updated_at"`
	HasNFO       bool     `json:"has_nfo"`
	PosterPath   string   `json:"poster_path"`
	CoverPath    string   `json:"cover_path"`
	FileCount    int      `json:"file_count"`
	VideoCount   int      `json:"video_count"`
	VariantCount int      `json:"variant_count"`
	Conflict     bool     `json:"conflict"`
}

type libraryMeta struct {
	Title           string   `json:"title"`
	TitleTranslated string   `json:"title_translated"`
	OriginalTitle   string   `json:"original_title"`
	Plot            string   `json:"plot"`
	PlotTranslated  string   `json:"plot_translated"`
	Number          string   `json:"number"`
	ReleaseDate     string   `json:"release_date"`
	Runtime         uint64   `json:"runtime"`
	Studio          string   `json:"studio"`
	Label           string   `json:"label"`
	Series          string   `json:"series"`
	Director        string   `json:"director"`
	Actors          []string `json:"actors"`
	Genres          []string `json:"genres"`
	PosterPath      string   `json:"poster_path"`
	CoverPath       string   `json:"cover_path"`
	FanartPath      string   `json:"fanart_path"`
	ThumbPath       string   `json:"thumb_path"`
	Source          string   `json:"source"`
	ScrapedAt       string   `json:"scraped_at"`
}

type libraryFileItem struct {
	Name         string `json:"name"`
	RelPath      string `json:"rel_path"`
	Kind         string `json:"kind"`
	Size         int64  `json:"size"`
	UpdatedAt    int64  `json:"updated_at"`
	VariantKey   string `json:"variant_key,omitempty"`
	VariantLabel string `json:"variant_label,omitempty"`
}

type libraryVariant struct {
	Key        string            `json:"key"`
	Label      string            `json:"label"`
	BaseName   string            `json:"base_name"`
	Suffix     string            `json:"suffix"`
	IsPrimary  bool              `json:"is_primary"`
	VideoPath  string            `json:"video_path"`
	NFOPath    string            `json:"nfo_path"`
	PosterPath string            `json:"poster_path"`
	CoverPath  string            `json:"cover_path"`
	Meta       libraryMeta       `json:"meta"`
	Files      []libraryFileItem `json:"files"`
	FileCount  int               `json:"file_count"`
}

type libraryDetail struct {
	Item              libraryListItem   `json:"item"`
	Meta              libraryMeta       `json:"meta"`
	Variants          []libraryVariant  `json:"variants"`
	PrimaryVariantKey string            `json:"primary_variant_key"`
	Files             []libraryFileItem `json:"files"`
}

func (a *API) saveLibrary() *medialib.Service {
	if a.media != nil {
		return a.media
	}
	return medialib.NewService(nil, "", a.saveDir)
}

func (a *API) handleListLibrary(c *gin.Context) {
	items, err := a.saveLibrary().ListSaveItems()
	if err != nil {
		writeFail(c.Writer, errCodeListLibraryFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", a.toLibraryListItems(items))
}

func (a *API) handleLibraryItemGet(c *gin.Context) {
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingLibraryPath, "missing library path")
		return
	}
	svc := a.saveLibrary()
	detail, err := svc.GetSaveDetail(pathValue)
	if err != nil {
		if os.IsNotExist(err) {
			writeFail(c.Writer, errCodeLibraryItemNotFound, err.Error())
			return
		}
		writeFail(c.Writer, errCodeLibraryItemReadFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", a.toLibraryDetail(detail))
}

func (a *API) handleLibraryItemPatch(c *gin.Context) {
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingLibraryPath, "missing library path")
		return
	}
	var req struct {
		Meta libraryMeta `json:"meta"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	detail, err := a.saveLibrary().UpdateSaveItem(pathValue, fromLibraryMeta(req.Meta))
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("library item update failed", zap.String("path", pathValue), zap.Error(err))
		writeFail(c.Writer, errCodeLibraryUpdateFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("library item updated", zap.String("path", detail.Item.RelPath))
	writeSuccess(c.Writer, http.StatusOK, "library item updated", a.toLibraryDetail(detail))
}

func (a *API) handleLibraryItemDelete(c *gin.Context) {
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingLibraryPath, "missing library path")
		return
	}
	if err := a.saveLibrary().DeleteSaveItem(pathValue); err != nil {
		if os.IsNotExist(err) {
			writeFail(c.Writer, errCodeLibraryItemNotFound, err.Error())
			return
		}
		logutil.GetLogger(c.Request.Context()).Warn("library item delete failed", zap.String("path", pathValue), zap.Error(err))
		writeFail(c.Writer, errCodeLibraryItemDeleteFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("library item deleted", zap.String("path", pathValue))
	writeSuccess(c.Writer, http.StatusOK, "library item deleted", nil)
}

func (a *API) handleLibraryFileGet(c *gin.Context) {
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingFilePath, "missing file path")
		return
	}
	svc := a.saveLibrary()
	_, absPath, err := svc.ResolveSavePath(pathValue)
	if err != nil {
		writeFail(c.Writer, errCodeResolveLibraryPathFailed, err.Error())
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		writeFail(c.Writer, errCodeLibraryFileNotFound, "library file not found")
		return
	}
	file, err := os.Open(absPath)
	if err != nil {
		writeFail(c.Writer, errCodeLibraryFileOpenFailed, "open library file failed")
		return
	}
	defer func() {
		_ = file.Close()
	}()
	c.Writer.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Writer.Header().Set("Pragma", "no-cache")
	c.Writer.Header().Set("Expires", "0")
	http.ServeContent(c.Writer, c.Request, info.Name(), time.Time{}, file)
}

func (a *API) handleLibraryFileDelete(c *gin.Context) {
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingFilePath, "missing file path")
		return
	}
	svc := a.saveLibrary()
	relPath, _, err := svc.ResolveSavePath(pathValue)
	if err != nil {
		writeFail(c.Writer, errCodeResolveLibraryPathFailed, err.Error())
		return
	}
	detail, err := svc.DeleteSaveFile(pathValue)
	if err != nil {
		if os.IsNotExist(err) {
			writeFail(c.Writer, errCodeLibraryFileNotFound, "library file not found")
			return
		}
		if err.Error() == "only extrafanart files can be deleted" {
			writeFail(c.Writer, errCodeLibraryFileDeleteDenied, err.Error())
			return
		}
		logutil.GetLogger(c.Request.Context()).Error("library file delete failed", zap.String("path", relPath), zap.Error(err))
		writeFail(c.Writer, errCodeLibraryFileDeleteFailed, "delete library file failed")
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("library file deleted", zap.String("path", relPath), zap.String("item_path", detail.Item.RelPath))
	writeSuccess(c.Writer, http.StatusOK, "library file deleted", a.toLibraryDetail(detail))
}

func (a *API) handleLibraryAsset(c *gin.Context) {
	itemPath := strings.TrimSpace(c.Query("path"))
	kind := strings.TrimSpace(c.Query("kind"))
	variantKey := strings.TrimSpace(c.Query("variant"))
	if itemPath == "" {
		writeFail(c.Writer, errCodeMissingLibraryPath, "missing library path")
		return
	}
	if kind != "poster" && kind != "cover" && kind != "fanart" {
		writeFail(c.Writer, errCodeInvalidAssetKind, "invalid asset kind")
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		writeFail(c.Writer, errCodeInvalidUploadFile, "invalid upload file")
		return
	}
	file, err := header.Open()
	if err != nil {
		writeFail(c.Writer, errCodeInvalidUploadFile, "invalid upload file")
		return
	}
	defer func() {
		_ = file.Close()
	}()
	data, err := io.ReadAll(file)
	if err != nil {
		writeFail(c.Writer, errCodeReadUploadFileFailed, "read upload file failed")
		return
	}
	if !strings.HasPrefix(http.DetectContentType(data), "image/") {
		writeFail(c.Writer, errCodeUploadFileNotImage, "upload file is not an image")
		return
	}
	detail, err := a.saveLibrary().ReplaceSaveAsset(itemPath, variantKey, kind, header.Filename, data)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("library asset replace failed",
			zap.String("path", itemPath),
			zap.String("variant", variantKey),
			zap.String("kind", kind),
			zap.String("file_name", header.Filename),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeLibraryAssetReplaceFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("library asset replaced",
		zap.String("path", detail.Item.RelPath),
		zap.String("variant", variantKey),
		zap.String("kind", kind),
		zap.String("file_name", header.Filename),
	)
	writeSuccess(c.Writer, http.StatusOK, "library asset replaced", a.toLibraryDetail(detail))
}

func (a *API) handleLibraryPosterCrop(c *gin.Context) {
	itemPath := strings.TrimSpace(c.Query("path"))
	variantKey := strings.TrimSpace(c.Query("variant"))
	if itemPath == "" {
		writeFail(c.Writer, errCodeMissingLibraryPath, "missing library path")
		return
	}
	var req struct {
		X      int `json:"x"`
		Y      int `json:"y"`
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Width <= 0 || req.Height <= 0 {
		writeFail(c.Writer, errCodeInvalidCropRectangle, "invalid crop rectangle")
		return
	}
	detail, err := a.saveLibrary().CropSavePoster(itemPath, variantKey, req.X, req.Y, req.Width, req.Height)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("library poster crop failed",
			zap.String("path", itemPath),
			zap.String("variant", variantKey),
			zap.Int("x", req.X),
			zap.Int("y", req.Y),
			zap.Int("width", req.Width),
			zap.Int("height", req.Height),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeLibraryPosterCropFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("library poster cropped",
		zap.String("path", detail.Item.RelPath),
		zap.String("variant", variantKey),
		zap.Int("x", req.X),
		zap.Int("y", req.Y),
		zap.Int("width", req.Width),
		zap.Int("height", req.Height),
	)
	writeSuccess(c.Writer, http.StatusOK, "library poster cropped", a.toLibraryDetail(detail))
}

func (a *API) toLibraryListItems(items []medialib.Item) []libraryListItem {
	conflicts := a.loadLibraryConflictFlags()
	out := make([]libraryListItem, 0, len(items))
	for _, item := range items {
		out = append(out, toLibraryListItem(item, conflicts))
	}
	return out
}

func toLibraryListItem(item medialib.Item, conflicts map[string]struct{}) libraryListItem {
	_, conflict := conflicts[buildLibraryConflictKey(item.RelPath, item.Number)]
	return libraryListItem{
		RelPath:      item.RelPath,
		Name:         item.Name,
		Title:        item.Title,
		Number:       item.Number,
		ReleaseDate:  item.ReleaseDate,
		Actors:       cloneStringSlice(item.Actors),
		UpdatedAt:    item.UpdatedAt,
		HasNFO:       item.HasNFO,
		PosterPath:   item.PosterPath,
		CoverPath:    item.CoverPath,
		FileCount:    item.FileCount,
		VideoCount:   item.VideoCount,
		VariantCount: item.VariantCount,
		Conflict:     conflict,
	}
}

func (a *API) toLibraryDetail(detail *medialib.Detail) *libraryDetail {
	if detail == nil {
		return nil
	}
	conflicts := a.loadLibraryConflictFlags()
	return &libraryDetail{
		Item:              toLibraryListItem(detail.Item, conflicts),
		Meta:              toLibraryMeta(detail.Meta),
		Variants:          toLibraryVariants(detail.Variants),
		PrimaryVariantKey: detail.PrimaryVariantKey,
		Files:             toLibraryFiles(detail.Files),
	}
}

func (a *API) loadLibraryConflictFlags() map[string]struct{} {
	out := map[string]struct{}{}
	if a.media == nil || !a.media.IsConfigured() {
		return out
	}
	items, err := a.media.ListItems(context.Background(), medialib.ListItemsOptions{})
	if err != nil {
		return out
	}
	for _, item := range items {
		out[buildLibraryConflictKey(item.RelPath, item.Number)] = struct{}{}
	}
	return out
}

func buildLibraryConflictKey(relPath string, number string) string {
	relPath = strings.TrimSpace(relPath)
	number = strings.TrimSpace(number)
	if number != "" {
		return "number:" + strings.ToUpper(number)
	}
	return "path:" + relPath
}

func toLibraryMeta(meta medialib.Meta) libraryMeta {
	return libraryMeta{
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
		Actors:          cloneStringSlice(meta.Actors),
		Genres:          cloneStringSlice(meta.Genres),
		PosterPath:      meta.PosterPath,
		CoverPath:       meta.CoverPath,
		FanartPath:      meta.FanartPath,
		ThumbPath:       meta.ThumbPath,
		Source:          meta.Source,
		ScrapedAt:       meta.ScrapedAt,
	}
}

func fromLibraryMeta(meta libraryMeta) medialib.Meta {
	return medialib.Meta{
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
		Actors:          cloneStringSlice(meta.Actors),
		Genres:          cloneStringSlice(meta.Genres),
		PosterPath:      meta.PosterPath,
		CoverPath:       meta.CoverPath,
		FanartPath:      meta.FanartPath,
		ThumbPath:       meta.ThumbPath,
		Source:          meta.Source,
		ScrapedAt:       meta.ScrapedAt,
	}
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func toLibraryFiles(files []medialib.FileItem) []libraryFileItem {
	out := make([]libraryFileItem, 0, len(files))
	for _, file := range files {
		out = append(out, libraryFileItem{
			Name:         file.Name,
			RelPath:      file.RelPath,
			Kind:         file.Kind,
			Size:         file.Size,
			UpdatedAt:    file.UpdatedAt,
			VariantKey:   file.VariantKey,
			VariantLabel: file.VariantLabel,
		})
	}
	return out
}

func toLibraryVariants(variants []medialib.Variant) []libraryVariant {
	out := make([]libraryVariant, 0, len(variants))
	for _, variant := range variants {
		out = append(out, libraryVariant{
			Key:        variant.Key,
			Label:      variant.Label,
			BaseName:   variant.BaseName,
			Suffix:     variant.Suffix,
			IsPrimary:  variant.IsPrimary,
			VideoPath:  variant.VideoPath,
			NFOPath:    variant.NFOPath,
			PosterPath: variant.PosterPath,
			CoverPath:  variant.CoverPath,
			Meta:       toLibraryMeta(variant.Meta),
			Files:      toLibraryFiles(variant.Files),
			FileCount:  variant.FileCount,
		})
	}
	return out
}
