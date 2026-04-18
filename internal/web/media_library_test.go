package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/repository"
)

func TestHandleMediaLibraryList(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		query    string
		wantCode int
	}{
		{"nil media", &API{}, "/api/media-library", 0},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, "/api/media-library", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryListConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name  string
		query string
	}{
		{"default", "/api/media-library"},
		{"with keyword", "/api/media-library?keyword=test"},
		{"with year", "/api/media-library?year=2024"},
		{"with size", "/api/media-library?size=lt-5"},
		{"with sort", "/api/media-library?sort=title&order=asc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, 0, resp.Code)
		})
	}
}

func TestHandleMediaLibraryItemGet(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		query    string
		wantCode int
	}{
		{"nil media", &API{}, "/api/media-library/item?id=1", errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, "/api/media-library/item?id=1", errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryItemGetConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing id", "/api/media-library/item", errCodeMissingMediaLibraryID},
		{"invalid id", "/api/media-library/item?id=abc", errCodeMissingMediaLibraryID},
		{"zero id", "/api/media-library/item?id=0", errCodeMissingMediaLibraryID},
		{"negative id", "/api/media-library/item?id=-1", errCodeMissingMediaLibraryID},
		{"not found", "/api/media-library/item?id=9999", errCodeMediaLibraryDetailNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryItemPatch(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		query    string
		body     string
		wantCode int
	}{
		{"nil media", &API{}, "/api/media-library/item?id=1", `{"meta":{}}`, errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, "/api/media-library/item?id=1", `{"meta":{}}`, errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, tt.query, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryItemPatchConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		body     string
		wantCode int
	}{
		{"missing id", "/api/media-library/item", `{"meta":{}}`, errCodeMissingMediaLibraryID},
		{"invalid json", "/api/media-library/item?id=1", `{bad`, errCodeInvalidJSONBody},
		{"not found", "/api/media-library/item?id=9999", `{"meta":{}}`, errCodeMediaLibraryUpdateFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, tt.query, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryFileGet(t *testing.T) {
	libraryDir := t.TempDir()
	filePath := filepath.Join(libraryDir, "movie", "poster.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, jpegBytes(), 0o600))

	dirPath := filepath.Join(libraryDir, "movie", "subdir")
	require.NoError(t, os.MkdirAll(dirPath, 0o755))

	svc := medialib.NewService(nil, libraryDir, "")
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCode   int
	}{
		{"nil media", "/api/media-library/file?path=x", http.StatusOK, -2},
		{"missing path", "/api/media-library/file", http.StatusOK, errCodeMissingFilePath},
		{"not found", "/api/media-library/file?path=missing.jpg", http.StatusOK, errCodeMediaLibraryFileNotFound},
		{"directory", "/api/media-library/file?path=movie/subdir", http.StatusOK, errCodeMediaLibraryFileNotFound},
		{"success", "/api/media-library/file?path=movie/poster.jpg", http.StatusOK, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantCode == -2 {
				nilAPI := &API{}
				nilEngine, err := nilAPI.Engine(":0")
				require.NoError(t, err)
				req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
				rec := httptest.NewRecorder()
				nilEngine.ServeHTTP(rec, req)
				resp := decodeResponse(t, rec)
				assert.Equal(t, errCodeLibraryNotConfigured, resp.Code)
				return
			}
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantCode >= 0 {
				resp := decodeResponse(t, rec)
				assert.Equal(t, tt.wantCode, resp.Code)
			} else if tt.wantCode == -1 {
				assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestHandleMediaLibraryFileDelete(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		query    string
		wantCode int
	}{
		{"nil media", &API{}, "/api/media-library/file?path=x&id=1", errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, "/api/media-library/file?path=x&id=1", errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryFileDeleteConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing path", "/api/media-library/file?id=1", errCodeMissingFilePath},
		{"missing id", "/api/media-library/file?path=x", errCodeMissingMediaLibraryID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryAsset(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		query    string
		wantCode int
	}{
		{"nil media", &API{}, "/api/media-library/asset?id=1&kind=cover", errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, "/api/media-library/asset?id=1&kind=cover", errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryAssetConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing id", "/api/media-library/asset?kind=cover", errCodeMissingMediaLibraryID},
		{"invalid kind", "/api/media-library/asset?id=1&kind=invalid", errCodeInvalidAssetKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}

	// Test no file upload.
	t.Run("no file upload", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/asset?id=1&kind=cover", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		resp := decodeResponse(t, rec)
		assert.Equal(t, errCodeInvalidUploadFile, resp.Code)
	})

	// Test non-image upload.
	t.Run("non-image upload", func(t *testing.T) {
		buf, ct := buildMultipartImage(t, "file", "test.txt", []byte("not an image"))
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/asset?id=1&kind=cover", buf)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		resp := decodeResponse(t, rec)
		assert.Equal(t, errCodeUploadFileNotImage, resp.Code)
	})
}

func TestHandleMediaLibrarySyncGet(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"nil media", &API{}, errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/sync", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibrarySyncGetConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/sync", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibrarySyncPost(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"nil media", &API{}, errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/sync", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibrarySyncPostConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/sync", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryMoveGet(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"nil media", &API{}, errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/move", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryMoveGetConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/move", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryMovePost(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"nil media", &API{}, errCodeLibraryNotConfigured},
		{"unconfigured", &API{media: medialib.NewService(nil, "", "")}, errCodeLibraryNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/move", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryMovePostConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/move", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryMoveTriggerFailed, resp.Code)
}

func TestHandleMediaLibraryStatus(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"nil media", &API{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := tt.api.Engine(":0")
			require.NoError(t, err)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/status", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleMediaLibraryStatusConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/status", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestParseInt64Query(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		wantID int64
		wantOK bool
	}{
		{"valid", "id=42", 42, true},
		{"empty", "", 0, false},
		{"missing id", "other=1", 0, false},
		{"invalid", "id=abc", 0, false},
		{"zero", "id=0", 0, false},
		{"negative", "id=-1", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/test"
			if tt.query != "" {
				url += "?" + tt.query
			}
			c, _ := newGinContext(http.MethodGet, url, nil)
			id, ok := parseInt64Query(c)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestReadJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"valid", `{"key":"value"}`, false},
		{"invalid", `{bad`, true},
		{"empty", ``, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test", strings.NewReader(tt.body))
			var out map[string]string
			err := readJSON(req, &out)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleMediaLibraryFileGetNotFound(t *testing.T) {
	libraryDir := t.TempDir()
	svc := medialib.NewService(nil, libraryDir, "")
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/media-library/file?path=nonexistent.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryFileNotFound, resp.Code)
}

func TestHandleMediaLibraryFileDeleteError(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/media-library/file?path=missing.jpg&id=9999", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryFileDeleteFailed, resp.Code)
}

func TestHandleMediaLibraryAssetReplaceError(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/media-library/asset?id=9999&kind=cover", buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryAssetReplaceFailed, resp.Code)
}

func TestHandleMediaLibraryMovePostSaveNotConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPost, "/api/media-library/move", nil)
	api.handleMediaLibraryMovePost(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryMoveTriggerFailed, resp.Code)
}

func TestHandleMediaLibraryItemGetNotFoundVsError(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContextWithParams(http.MethodGet, "/api/media-library/item?id=9999", nil, nil)
	c.Request.URL.RawQuery = "id=9999"
	api.handleMediaLibraryItemGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryDetailNotFound, resp.Code)
}

func TestHandleMediaLibraryItemPatchError(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPatch, "/api/media-library/item?id=9999", strings.NewReader(`{"meta":{"title":"test"}}`))
	c.Request.URL.RawQuery = "id=9999"
	api.handleMediaLibraryItemPatch(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryUpdateFailed, resp.Code)
}

func TestHandleMediaLibraryFileGetOpenError(t *testing.T) {
	libraryDir := t.TempDir()
	filePath := filepath.Join(libraryDir, "demo", "unreadable.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, jpegBytes(), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filePath, 0o644) })

	svc := medialib.NewService(nil, libraryDir, "")
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/file?path=demo/unreadable.jpg", nil)
	c.Request.URL.RawQuery = "path=demo/unreadable.jpg"
	api.handleMediaLibraryFileGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryFileOpenFailed, resp.Code)
}

func TestHandleMediaLibraryAssetImageUploadSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	c, rec := newGinContext(http.MethodPost, "/api/media-library/asset?id=9999&kind=cover", buf)
	c.Request.URL.RawQuery = "id=9999&kind=cover"
	c.Request.Header.Set("Content-Type", ct)
	api.handleMediaLibraryAsset(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryAssetReplaceFailed, resp.Code)
}

func TestHandleMediaLibrarySyncGetSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/sync", nil)
	api.handleMediaLibrarySyncGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibrarySyncPostSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPost, "/api/media-library/sync", nil)
	api.handleMediaLibrarySyncPost(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryMoveGetSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/move", nil)
	api.handleMediaLibraryMoveGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryStatusSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/status", nil)
	api.handleMediaLibraryStatus(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryListWithKeyword(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library?keyword=test&year=2024&size=lt-5&sort=title&order=asc", nil)
	c.Request.URL.RawQuery = "keyword=test&year=2024&size=lt-5&sort=title&order=asc"
	api.handleMediaLibraryList(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryItemPatchSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPatch, "/api/media-library/item?id=99999", strings.NewReader(`{"meta":{"title":"updated"}}`))
	c.Request.URL.RawQuery = "id=99999"
	api.handleMediaLibraryItemPatch(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryUpdateFailed, resp.Code)
}

func TestHandleMediaLibrarySyncGetDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/sync", nil)
	api.handleMediaLibrarySyncGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibrarySyncStatusFailed, resp.Code)
}

func TestHandleMediaLibrarySyncPostDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPost, "/api/media-library/sync", nil)
	api.handleMediaLibrarySyncPost(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryMoveGetDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/move", nil)
	api.handleMediaLibraryMoveGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryMoveStatusFailed, resp.Code)
}

func TestHandleMediaLibraryMovePostDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPost, "/api/media-library/move", nil)
	api.handleMediaLibraryMovePost(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryStatusDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/status", nil)
	api.handleMediaLibraryStatus(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryStatusFailed, resp.Code)
}

// TestHandleMediaLibrarySyncLogsNotConfigured 覆盖边缘 case: media service
// 未注入时前端依然能拿到合法的空列表响应, 不会跑到 500。
func TestHandleMediaLibrarySyncLogsNotConfigured(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/sync/logs", nil)
	api.handleMediaLibrarySyncLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

// TestHandleMediaLibrarySyncLogsSuccess 覆盖正常 case: 先触发一次 full sync
// (libraryDir 是空 TempDir, 日志表里会留下 "同步开始 / 同步完成" 两条),
// 再请求日志接口, 断言 code=0 + 返回的 list 非空。
func TestHandleMediaLibrarySyncLogsSuccess(t *testing.T) {
	svc := setupMediaLibDB(t)
	require.NoError(t, svc.TriggerFullSync(context.Background()))
	svc.WaitBackground()

	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/sync/logs?limit=10", nil)
	c.Request.URL.RawQuery = "limit=10"
	api.handleMediaLibrarySyncLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)

	// resp.Data 是 []interface{} 形式, 至少有 start + end 两条记录。
	items, ok := resp.Data.([]interface{})
	require.True(t, ok, "response data must be array")
	assert.GreaterOrEqual(t, len(items), 2)
}

// TestHandleMediaLibrarySyncLogsDBError 覆盖异常 case: db 已关闭时 List 返回
// 错误, handler 必须走 errCodeMediaLibrarySyncLogsFailed 分支而不是 500。
func TestHandleMediaLibrarySyncLogsDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/sync/logs", nil)
	api.handleMediaLibrarySyncLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibrarySyncLogsFailed, resp.Code)
}

// TestHandleMediaLibrarySyncLogsBadLimit 覆盖边缘 case: limit 参数不是合法
// 整数时 handler 不应该 400, 而是退化到 service 层的默认 limit (ListSyncLogs
// 自己把 <=0 兜回默认值)。
func TestHandleMediaLibrarySyncLogsBadLimit(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/sync/logs?limit=abc", nil)
	c.Request.URL.RawQuery = "limit=abc"
	api.handleMediaLibrarySyncLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryListDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library", nil)
	api.handleMediaLibraryList(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeListMediaLibraryFailed, resp.Code)
}

func TestHandleMediaLibraryItemGetDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/item?id=1", nil)
	c.Request.URL.RawQuery = "id=1"
	api.handleMediaLibraryItemGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryDetailReadFailed, resp.Code)
}

func TestHandleMediaLibraryItemPatchDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPatch, "/api/media-library/item?id=1", strings.NewReader(`{"meta":{}}`))
	c.Request.URL.RawQuery = "id=1"
	api.handleMediaLibraryItemPatch(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryUpdateFailed, resp.Code)
}

func TestHandleMediaLibraryFileDeleteDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodDelete, "/api/media-library/file?path=x&id=1", nil)
	c.Request.URL.RawQuery = "path=x&id=1"
	api.handleMediaLibraryFileDelete(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryFileDeleteFailed, resp.Code)
}

func TestHandleMediaLibraryAssetDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	c, rec := newGinContext(http.MethodPost, "/api/media-library/asset?id=1&kind=cover", buf)
	c.Request.URL.RawQuery = "id=1&kind=cover"
	c.Request.Header.Set("Content-Type", ct)
	api.handleMediaLibraryAsset(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryAssetReplaceFailed, resp.Code)
}

func TestHandleMediaLibraryItemGetFileDoesNotExist(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/item?id=99999", nil)
	c.Request.URL.RawQuery = "id=99999"
	api.handleMediaLibraryItemGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibraryDetailNotFound, resp.Code)
}

func TestHandleMediaLibraryItemGetDBSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ml_get.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	db := sqlite.DB()
	detailJSON := `{"item":{"rel_path":"ok","name":"ok","number":"OK-001"},"meta":{},"variants":[],"files":[]}`
	itemJSON := `{"rel_path":"ok","number":"OK-001"}`
	_, err = db.Exec(
		`INSERT INTO yamdc_media_library_tab (rel_path,item_json,detail_json,created_at) VALUES (?,?,?,?)`,
		"ok", itemJSON, detailJSON, time.Now().UnixMilli(),
	)
	require.NoError(t, err)
	var id int64
	require.NoError(t, db.QueryRow(`SELECT id FROM yamdc_media_library_tab WHERE rel_path=?`, "ok").Scan(&id))

	svc := medialib.NewService(db, t.TempDir(), "")
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, fmt.Sprintf("/api/media-library/item?id=%d", id), nil)
	c.Request.URL.RawQuery = fmt.Sprintf("id=%d", id)
	api.handleMediaLibraryItemGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleMediaLibraryFileGetResolveError(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodGet, "/api/media-library/file?path=../../../etc/passwd", nil)
	c.Request.URL.RawQuery = "path=../../../etc/passwd"
	api.handleMediaLibraryFileGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeResolveMediaLibraryPathFailed, resp.Code)
}

func TestHandleMediaLibrarySyncPostAlreadyRunning(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sync_running.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	db := sqlite.DB()
	_, err = db.Exec(
		`INSERT INTO yamdc_task_state_tab (task_key,status,total,processed,success_count,conflict_count,error_count,current,message,started_at,finished_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		"media_library_sync", "running", 0, 0, 0, 0, 0, "", "", time.Now().UnixMilli(), 0, time.Now().UnixMilli(),
	)
	require.NoError(t, err)

	svc := medialib.NewService(db, t.TempDir(), "")
	api := &API{media: svc}
	c, rec := newGinContext(http.MethodPost, "/api/media-library/sync", nil)
	api.handleMediaLibrarySyncPost(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeMediaLibrarySyncTriggerFailed, resp.Code)
}
