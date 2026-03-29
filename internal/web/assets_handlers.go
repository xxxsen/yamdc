package web

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

func (a *API) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		file, header, err := r.FormFile("file")
		if err != nil {
			writeFail(w, errCodeInvalidUploadFile, "invalid upload file")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			writeFail(w, errCodeReadUploadFileFailed, "read upload file failed")
			return
		}
		if !strings.HasPrefix(http.DetectContentType(data), "image/") {
			writeFail(w, errCodeUploadFileNotImage, "upload file is not an image")
			return
		}
		key, err := store.AnonymousPutDataTo(r.Context(), a.store, data)
		if err != nil {
			logutil.GetLogger(r.Context()).Error("debug asset upload failed", zap.String("file_name", header.Filename), zap.Error(err))
			writeFail(w, errCodeDebugAssetStoreFailed, err.Error())
			return
		}
		logutil.GetLogger(r.Context()).Info("debug asset uploaded", zap.String("file_name", header.Filename), zap.String("asset_key", key))
		writeSuccess(w, http.StatusOK, "asset uploaded", map[string]string{
			"name": filepath.Base(header.Filename),
			"key":  key,
		})
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/api/assets/")
	key = strings.TrimSpace(key)
	if key == "" {
		writeFail(w, errCodeInvalidAssetKey, "invalid asset key")
		return
	}
	data, err := store.GetDataFrom(r.Context(), a.store, key)
	if err != nil {
		writeFail(w, errCodeAssetNotFound, "asset not found")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
