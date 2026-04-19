package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/store"
)

func TestHandleAssetGet(t *testing.T) {
	memStore := store.NewMemStorage()
	require.NoError(t, store.PutDataTo(context.Background(), memStore, "img-key", pngBytes()))
	api := &API{store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   int
	}{
		{"success", "/api/assets/img-key", http.StatusOK, -1},
		{"empty key", "/api/assets/%20", http.StatusOK, errCodeInvalidAssetKey},
		{"not found", "/api/assets/missing", http.StatusOK, errCodeAssetNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantCode >= 0 {
				resp := decodeResponse(t, rec)
				assert.Equal(t, tt.wantCode, resp.Code)
			} else {
				assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
				assert.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestHandleAssetPost(t *testing.T) {
	tests := []struct {
		name     string
		file     []byte
		wantCode int
	}{
		{"success png", pngBytes(), 0},
		{"not image", []byte("hello world, this is plain text"), errCodeUploadFileNotImage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memStore := store.NewMemStorage()
			api := &API{store: memStore}
			engine, err := api.Engine(":0")
			require.NoError(t, err)
			buf, ct := buildMultipartImage(t, "file", "test.png", tt.file)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/assets/test.png", buf)
			req.Header.Set("Content-Type", ct)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleAssetPostNoFile(t *testing.T) {
	api := &API{store: store.NewMemStorage()}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/assets/test.png", strings.NewReader("no file"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeInvalidUploadFile, resp.Code)
}

func TestHandleAssetPostStoreError(t *testing.T) {
	api := &API{store: &failingStore{}}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	buf, ct := buildMultipartImage(t, "file", "poster.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/assets/poster.png", buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeDebugAssetStoreFailed, resp.Code)
}
