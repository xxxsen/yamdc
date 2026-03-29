package web

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/medialib"
	"go.uber.org/zap"
)

func (a *API) handleMediaLibraryList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.media == nil || !a.media.IsConfigured() {
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": []medialib.Item{}})
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	year := strings.TrimSpace(r.URL.Query().Get("year"))
	sizeFilter := strings.TrimSpace(r.URL.Query().Get("size"))
	sortMode := strings.TrimSpace(r.URL.Query().Get("sort"))
	order := strings.TrimSpace(r.URL.Query().Get("order"))
	items, err := a.media.ListItems(r.Context(), medialib.ListItemsOptions{
		Keyword:    keyword,
		Year:       year,
		SizeFilter: sizeFilter,
		Sort:       sortMode,
		Order:      order,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": items})
}

func (a *API) handleMediaLibraryItem(w http.ResponseWriter, r *http.Request) {
	if a.media == nil || !a.media.IsConfigured() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "library dir is not configured"})
		return
	}
	id, ok := parseInt64Query(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "missing media library id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		detail, err := a.media.GetDetail(r.Context(), id)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "file does not exist") {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": detail})
	case http.MethodPatch:
		var req struct {
			Meta medialib.Meta `json:"meta"`
		}
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid json body"})
			return
		}
		detail, err := a.media.UpdateItem(r.Context(), id, req.Meta)
		if err != nil {
			logutil.GetLogger(r.Context()).Warn("media library item update failed", zap.Int64("media_library_id", id), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("media library item updated", zap.Int64("media_library_id", id), zap.String("rel_path", detail.Item.RelPath))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "media library item updated", "data": detail})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *API) handleMediaLibraryFile(w http.ResponseWriter, r *http.Request) {
	if a.media == nil || !a.media.IsConfigured() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "library dir is not configured"})
		return
	}
	pathValue := strings.TrimSpace(r.URL.Query().Get("path"))
	if pathValue == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "missing file path"})
		return
	}
	_, absPath, err := a.media.ResolveLibraryPath(pathValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	switch r.Method {
	case http.MethodGet:
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"code": 1, "message": "media library file not found"})
			return
		}
		file, err := os.Open(absPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": "open media library file failed"})
			return
		}
		defer file.Close()
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeContent(w, r, info.Name(), time.Time{}, file)
	case http.MethodDelete:
		id, ok := parseInt64Query(r, "id")
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "missing media library id"})
			return
		}
		detail, err := a.media.DeleteFile(r.Context(), id, pathValue)
		if err != nil {
			logutil.GetLogger(r.Context()).Warn("media library file delete failed", zap.Int64("media_library_id", id), zap.String("path", pathValue), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("media library file deleted", zap.Int64("media_library_id", id), zap.String("path", pathValue), zap.String("rel_path", detail.Item.RelPath))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "media library file deleted", "data": detail})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *API) handleMediaLibraryAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.media == nil || !a.media.IsConfigured() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "library dir is not configured"})
		return
	}
	id, ok := parseInt64Query(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "missing media library id"})
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	variantKey := strings.TrimSpace(r.URL.Query().Get("variant"))
	if kind != "poster" && kind != "cover" && kind != "fanart" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid asset kind"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid upload file"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "read upload file failed"})
		return
	}
	if !strings.HasPrefix(http.DetectContentType(data), "image/") {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "upload file is not an image"})
		return
	}
	detail, err := a.media.ReplaceAsset(r.Context(), id, variantKey, kind, header.Filename, data)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("media library asset replace failed",
			zap.Int64("media_library_id", id),
			zap.String("variant", variantKey),
			zap.String("kind", kind),
			zap.String("file_name", header.Filename),
			zap.Error(err),
		)
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	logutil.GetLogger(r.Context()).Info("media library asset replaced",
		zap.Int64("media_library_id", id),
		zap.String("variant", variantKey),
		zap.String("kind", kind),
		zap.String("file_name", header.Filename),
		zap.String("rel_path", detail.Item.RelPath),
	)
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "media library asset replaced", "data": detail})
}

func (a *API) handleMediaLibrarySync(w http.ResponseWriter, r *http.Request) {
	if a.media == nil || !a.media.IsConfigured() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "library dir is not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		state, err := a.media.GetStatusSnapshot(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": state.Sync})
	case http.MethodPost:
		if err := a.media.TriggerFullSync(r.Context()); err != nil {
			logutil.GetLogger(r.Context()).Warn("media library sync trigger failed", zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("media library sync triggered")
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "media library sync started"})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *API) handleMediaLibraryMove(w http.ResponseWriter, r *http.Request) {
	if a.media == nil || !a.media.IsConfigured() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "library dir is not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		state, err := a.media.GetStatusSnapshot(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": state.Move})
	case http.MethodPost:
		if err := a.media.TriggerMove(r.Context()); err != nil {
			logutil.GetLogger(r.Context()).Warn("move to media library trigger failed", zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("move to media library triggered")
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "move to media library started"})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *API) handleMediaLibraryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.media == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": medialib.StatusSnapshot{Configured: false}})
		return
	}
	status, err := a.media.GetStatusSnapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": status})
}

func parseInt64Query(r *http.Request, key string) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
