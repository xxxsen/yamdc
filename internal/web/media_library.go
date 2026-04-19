package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/medialib"
)

func (a *API) handleMediaLibraryList(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeSuccess(c.Writer, "ok", []medialib.Item{})
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	year := strings.TrimSpace(c.Query("year"))
	sizeFilter := strings.TrimSpace(c.Query("size"))
	sortMode := strings.TrimSpace(c.Query("sort"))
	order := strings.TrimSpace(c.Query("order"))
	items, err := a.media.ListItems(c.Request.Context(), medialib.ListItemsOptions{
		Keyword:    keyword,
		Year:       year,
		SizeFilter: sizeFilter,
		Sort:       sortMode,
		Order:      order,
	})
	if err != nil {
		writeFail(c.Writer, errCodeListMediaLibraryFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", items)
}

func (a *API) handleMediaLibraryItemGet(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	id, ok := parseInt64Query(c)
	if !ok {
		writeFail(c.Writer, errCodeMissingMediaLibraryID, "missing media library id")
		return
	}
	detail, err := a.media.GetDetail(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "file does not exist") {
			writeFail(c.Writer, errCodeMediaLibraryDetailNotFound, err.Error())
			return
		}
		writeFail(c.Writer, errCodeMediaLibraryDetailReadFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", detail)
}

func (a *API) handleMediaLibraryItemPatch(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	id, ok := parseInt64Query(c)
	if !ok {
		writeFail(c.Writer, errCodeMissingMediaLibraryID, "missing media library id")
		return
	}
	var req struct {
		Meta medialib.Meta `json:"meta"`
	}
	if err := readJSON(c.Request, &req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	detail, err := a.media.UpdateItem(c.Request.Context(), id, req.Meta)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("media library item update failed",
			zap.Int64("media_library_id", id),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeMediaLibraryUpdateFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("media library item updated",
		zap.Int64("media_library_id", id),
		zap.String("rel_path", detail.Item.RelPath),
	)
	writeSuccess(c.Writer, "media library item updated", detail)
}

func (a *API) handleMediaLibraryFileGet(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingFilePath, "missing file path")
		return
	}
	_, absPath, err := a.media.ResolveLibraryPath(pathValue)
	if err != nil {
		writeFail(c.Writer, errCodeResolveMediaLibraryPathFailed, err.Error())
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		writeFail(c.Writer, errCodeMediaLibraryFileNotFound, "media library file not found")
		return
	}
	file, err := os.Open(absPath)
	if err != nil {
		writeFail(c.Writer, errCodeMediaLibraryFileOpenFailed, "open media library file failed")
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

func (a *API) handleMediaLibraryFileDelete(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	pathValue := strings.TrimSpace(c.Query("path"))
	if pathValue == "" {
		writeFail(c.Writer, errCodeMissingFilePath, "missing file path")
		return
	}
	id, ok := parseInt64Query(c)
	if !ok {
		writeFail(c.Writer, errCodeMissingMediaLibraryID, "missing media library id")
		return
	}
	detail, err := a.media.DeleteFile(c.Request.Context(), id, pathValue)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("media library file delete failed",
			zap.Int64("media_library_id", id),
			zap.String("path", pathValue),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeMediaLibraryFileDeleteFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("media library file deleted",
		zap.Int64("media_library_id", id),
		zap.String("path", pathValue),
		zap.String("rel_path", detail.Item.RelPath),
	)
	writeSuccess(c.Writer, "media library file deleted", detail)
}

func (a *API) handleMediaLibraryAsset(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	id, ok := parseInt64Query(c)
	if !ok {
		writeFail(c.Writer, errCodeMissingMediaLibraryID, "missing media library id")
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	variantKey := strings.TrimSpace(c.Query("variant"))
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
	detail, err := a.media.ReplaceAsset(c.Request.Context(), id, variantKey, kind, header.Filename, data)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("media library asset replace failed",
			zap.Int64("media_library_id", id),
			zap.String("variant", variantKey),
			zap.String("kind", kind),
			zap.String("file_name", header.Filename),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeMediaLibraryAssetReplaceFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("media library asset replaced",
		zap.Int64("media_library_id", id),
		zap.String("variant", variantKey),
		zap.String("kind", kind),
		zap.String("file_name", header.Filename),
		zap.String("rel_path", detail.Item.RelPath),
	)
	writeSuccess(c.Writer, "media library asset replaced", detail)
}

func (a *API) handleMediaLibrarySyncGet(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	state, err := a.media.GetStatusSnapshot(c.Request.Context())
	if err != nil {
		writeFail(c.Writer, errCodeMediaLibrarySyncStatusFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", state.Sync)
}

func (a *API) handleMediaLibrarySyncPost(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	if err := a.media.TriggerFullSync(c.Request.Context()); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("media library sync trigger failed", zap.Error(err))
		writeFail(c.Writer, errCodeMediaLibrarySyncTriggerFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("media library sync triggered")
	writeSuccess(c.Writer, "media library sync started", nil)
}

func (a *API) handleMediaLibraryMoveGet(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	state, err := a.media.GetStatusSnapshot(c.Request.Context())
	if err != nil {
		writeFail(c.Writer, errCodeMediaLibraryMoveStatusFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", state.Move)
}

func (a *API) handleMediaLibraryMovePost(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeFail(c.Writer, errCodeLibraryNotConfigured, "library dir is not configured")
		return
	}
	if err := a.media.TriggerMove(c.Request.Context()); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("move to media library trigger failed", zap.Error(err))
		writeFail(c.Writer, errCodeMediaLibraryMoveTriggerFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("move to media library triggered")
	writeSuccess(c.Writer, "move to media library started", nil)
}

// handleMediaLibrarySyncLogs 暴露最近的自动/手动同步事件日志, 支持 limit 参数
// (默认 200, 上限 1000 由 service 层兜底)。前端 '查看同步日志' 弹窗调用此接口。
func (a *API) handleMediaLibrarySyncLogs(c *gin.Context) {
	if a.media == nil || !a.media.IsConfigured() {
		writeSuccess(c.Writer, "ok", []medialib.SyncLogEntry{})
		return
	}
	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	entries, err := a.media.ListSyncLogs(c.Request.Context(), limit)
	if err != nil {
		writeFail(c.Writer, errCodeMediaLibrarySyncLogsFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", entries)
}

func (a *API) handleMediaLibraryStatus(c *gin.Context) {
	if a.media == nil {
		writeSuccess(c.Writer, "ok", medialib.StatusSnapshot{Configured: false})
		return
	}
	status, err := a.media.GetStatusSnapshot(c.Request.Context())
	if err != nil {
		writeFail(c.Writer, errCodeMediaLibraryStatusFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", status)
}

func parseInt64Query(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("id"))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("decode json body failed: %w", err)
	}
	return nil
}
