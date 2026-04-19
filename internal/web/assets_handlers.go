package web

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/store"
)

func (a *API) handleAssetPost(c *gin.Context) {
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
	key, err := store.AnonymousPutDataTo(c.Request.Context(), a.store, data)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Error("debug asset upload failed",
			zap.String("file_name", header.Filename),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeDebugAssetStoreFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("debug asset uploaded",
		zap.String("file_name", header.Filename),
		zap.String("asset_key", key),
	)
	writeSuccess(c.Writer, "asset uploaded", map[string]string{
		"name": filepath.Base(header.Filename),
		"key":  key,
	})
}

func (a *API) handleAssetGet(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("path"), "/")
	key = strings.TrimSpace(key)
	if key == "" {
		writeFail(c.Writer, errCodeInvalidAssetKey, "invalid asset key")
		return
	}
	data, err := store.GetDataFrom(c.Request.Context(), a.store, key)
	if err != nil {
		writeFail(c.Writer, errCodeAssetNotFound, "asset not found")
		return
	}
	c.Writer.Header().Set("Content-Type", http.DetectContentType(data))
	c.Writer.Header().Set("Cache-Control", "public, max-age=300")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(data)
}
