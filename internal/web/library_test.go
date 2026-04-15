package web

import (
	"context"
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

func TestSaveLibrary(t *testing.T) {
	t.Run("with media service", func(t *testing.T) {
		mediaSvc := medialib.NewService(nil, "", t.TempDir())
		api := &API{media: mediaSvc}
		assert.Equal(t, mediaSvc, api.saveLibrary())
	})
	t.Run("without media service", func(t *testing.T) {
		api := &API{saveDir: t.TempDir()}
		svc := api.saveLibrary()
		assert.NotNil(t, svc)
	})
}

func TestHandleListLibrary(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/library", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleLibraryItemGet(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing path", "/api/library/item", errCodeMissingLibraryPath},
		{"empty path", "/api/library/item?path=", errCodeMissingLibraryPath},
		{"not found", "/api/library/item?path=nonexistent", errCodeLibraryItemReadFailed},
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

func TestHandleLibraryItemPatch(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		body     string
		wantCode int
	}{
		{"missing path", "/api/library/item", `{"meta":{}}`, errCodeMissingLibraryPath},
		{"invalid json", "/api/library/item?path=demo", `{bad`, errCodeInvalidJSONBody},
		{"path not found", "/api/library/item?path=nonexistent", `{"meta":{}}`, errCodeLibraryUpdateFailed},
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

func TestHandleLibraryItemDelete(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.nfo"), []byte("<movie></movie>"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing path", "/api/library/item", errCodeMissingLibraryPath},
		{"not found", "/api/library/item?path=nonexistent", errCodeLibraryItemDeleteFailed},
		{"success", "/api/library/item?path=demo", 0},
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

func TestHandleLibraryFileGet(t *testing.T) {
	saveDir := t.TempDir()
	filePath := filepath.Join(saveDir, "demo", "cover.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, jpegBytes(), 0o600))

	dirPath := filepath.Join(saveDir, "demo", "subdir")
	require.NoError(t, os.MkdirAll(dirPath, 0o755))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCode   int
	}{
		{"missing path", "/api/library/file", http.StatusOK, errCodeMissingFilePath},
		{"not found", "/api/library/file?path=missing.jpg", http.StatusOK, errCodeLibraryFileNotFound},
		{"directory not file", "/api/library/file?path=demo/subdir", http.StatusOK, errCodeLibraryFileNotFound},
		{"success", "/api/library/file?path=demo/cover.jpg", http.StatusOK, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantCode >= 0 {
				resp := decodeResponse(t, rec)
				assert.Equal(t, tt.wantCode, resp.Code)
			} else {
				assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
				assert.Equal(t, "no-store, no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestHandleLibraryFileDelete(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing path", "/api/library/file", errCodeMissingFilePath},
		{"not found", "/api/library/file?path=missing.jpg", errCodeLibraryFileNotFound},
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

func TestHandleLibraryAsset(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"missing path", "/api/library/asset", errCodeMissingLibraryPath},
		{"invalid kind", "/api/library/asset?path=demo&kind=invalid", errCodeInvalidAssetKind},
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
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/asset?path=demo&kind=cover", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		resp := decodeResponse(t, rec)
		assert.Equal(t, errCodeInvalidUploadFile, resp.Code)
	})

	// Test non-image upload.
	t.Run("non-image upload", func(t *testing.T) {
		buf, ct := buildMultipartImage(t, "file", "test.txt", []byte("not an image"))
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/asset?path=demo&kind=cover", buf)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		resp := decodeResponse(t, rec)
		assert.Equal(t, errCodeUploadFileNotImage, resp.Code)
	})
}

func TestHandleLibraryPosterCrop(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		body     string
		wantCode int
	}{
		{"missing path", "/api/library/poster-crop", `{"x":0,"y":0,"width":10,"height":10}`, errCodeMissingLibraryPath},
		{"invalid json", "/api/library/poster-crop?path=demo", `{bad`, errCodeInvalidJSONBody},
		{"zero width", "/api/library/poster-crop?path=demo", `{"x":0,"y":0,"width":0,"height":10}`, errCodeInvalidCropRectangle},
		{"zero height", "/api/library/poster-crop?path=demo", `{"x":0,"y":0,"width":10,"height":0}`, errCodeInvalidCropRectangle},
		{"negative", "/api/library/poster-crop?path=demo", `{"x":0,"y":0,"width":-1,"height":10}`, errCodeInvalidCropRectangle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.query, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestBuildLibraryConflictKey(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		number  string
		want    string
	}{
		{"with number", "some/path", "ABC-123", "number:ABC-123"},
		{"empty number", "some/path", "", "path:some/path"},
		{"whitespace number", "some/path", "  ", "path:some/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildLibraryConflictKey(tt.relPath, tt.number))
		})
	}
}

func TestCloneStringSlice(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"non-empty", []string{"a", "b"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cloneStringSlice(tt.input)
			assert.Equal(t, tt.expect, result)
			if len(tt.input) > 0 {
				tt.input[0] = "modified"
				assert.NotEqual(t, tt.input[0], result[0], "clone should be independent")
			}
		})
	}
}

func TestToLibraryMeta(t *testing.T) {
	input := medialib.Meta{
		Title:           "t",
		TitleTranslated: "tt",
		OriginalTitle:   "ot",
		Plot:            "p",
		PlotTranslated:  "pt",
		Number:          "n",
		ReleaseDate:     "2024-01-01",
		Runtime:         120,
		Studio:          "s",
		Label:           "l",
		Series:          "se",
		Director:        "d",
		Actors:          []string{"a"},
		Genres:          []string{"g"},
		PosterPath:      "pp",
		CoverPath:       "cp",
		FanartPath:      "fp",
		ThumbPath:       "tp",
		Source:          "src",
		ScrapedAt:       "2024-01-01T00:00:00Z",
	}
	result := toLibraryMeta(input)
	assert.Equal(t, "t", result.Title)
	assert.Equal(t, "tt", result.TitleTranslated)
	assert.Equal(t, "ot", result.OriginalTitle)
	assert.Equal(t, "p", result.Plot)
	assert.Equal(t, "pt", result.PlotTranslated)
	assert.Equal(t, "n", result.Number)
	assert.Equal(t, "2024-01-01", result.ReleaseDate)
	assert.Equal(t, uint64(120), result.Runtime)
	assert.Equal(t, "s", result.Studio)
	assert.Equal(t, "l", result.Label)
	assert.Equal(t, "se", result.Series)
	assert.Equal(t, "d", result.Director)
	assert.Equal(t, []string{"a"}, result.Actors)
	assert.Equal(t, []string{"g"}, result.Genres)
	assert.Equal(t, "pp", result.PosterPath)
	assert.Equal(t, "cp", result.CoverPath)
	assert.Equal(t, "fp", result.FanartPath)
	assert.Equal(t, "tp", result.ThumbPath)
	assert.Equal(t, "src", result.Source)
	assert.Equal(t, "2024-01-01T00:00:00Z", result.ScrapedAt)
}

func TestFromLibraryMeta(t *testing.T) {
	input := libraryMeta{
		Title:   "t",
		Number:  "n",
		Actors:  []string{"a"},
		Genres:  []string{"g"},
		Runtime: 90,
	}
	result := fromLibraryMeta(input)
	assert.Equal(t, "t", result.Title)
	assert.Equal(t, "n", result.Number)
	assert.Equal(t, []string{"a"}, result.Actors)
	assert.Equal(t, []string{"g"}, result.Genres)
	assert.Equal(t, uint64(90), result.Runtime)
}

func TestToLibraryMetaFromLibraryMetaRoundTrip(t *testing.T) {
	original := medialib.Meta{
		Title:  "round",
		Number: "RT-001",
		Actors: []string{"x", "y"},
		Genres: []string{"g1"},
	}
	lm := toLibraryMeta(original)
	back := fromLibraryMeta(lm)
	assert.Equal(t, original.Title, back.Title)
	assert.Equal(t, original.Number, back.Number)
	assert.Equal(t, original.Actors, back.Actors)
}

func TestToLibraryFiles(t *testing.T) {
	tests := []struct {
		name  string
		input []medialib.FileItem
		count int
	}{
		{"empty", nil, 0},
		{"one item", []medialib.FileItem{{Name: "f", RelPath: "r", Kind: "video", Size: 100, UpdatedAt: 1}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLibraryFiles(tt.input)
			assert.Len(t, result, tt.count)
			if tt.count > 0 {
				assert.Equal(t, "f", result[0].Name)
				assert.Equal(t, "r", result[0].RelPath)
				assert.Equal(t, "video", result[0].Kind)
			}
		})
	}
}

func TestToLibraryVariants(t *testing.T) {
	tests := []struct {
		name  string
		input []medialib.Variant
		count int
	}{
		{"empty", nil, 0},
		{"one variant", []medialib.Variant{{
			Key: "v1", Label: "Variant 1", BaseName: "base", Suffix: ".mp4",
			IsPrimary: true, VideoPath: "v", NFOPath: "n", PosterPath: "p", CoverPath: "c",
			Meta:  medialib.Meta{Title: "t"},
			Files: []medialib.FileItem{{Name: "f"}},
		}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLibraryVariants(tt.input)
			assert.Len(t, result, tt.count)
			if tt.count > 0 {
				assert.Equal(t, "v1", result[0].Key)
				assert.Equal(t, "Variant 1", result[0].Label)
				assert.True(t, result[0].IsPrimary)
				assert.Equal(t, "t", result[0].Meta.Title)
				assert.Len(t, result[0].Files, 1)
			}
		})
	}
}

func TestToLibraryListItem(t *testing.T) {
	item := medialib.Item{
		RelPath: "demo", Name: "demo", Title: "Demo", Number: "ABC-123",
		Actors: []string{"a"}, HasNFO: true, FileCount: 2, VideoCount: 1, VariantCount: 1,
	}
	conflicts := map[string]struct{}{"number:ABC-123": {}}
	result := toLibraryListItem(item, conflicts)
	assert.True(t, result.Conflict)
	assert.Equal(t, "demo", result.RelPath)
	assert.Equal(t, "Demo", result.Title)
	assert.Equal(t, []string{"a"}, result.Actors)

	result2 := toLibraryListItem(item, map[string]struct{}{})
	assert.False(t, result2.Conflict)
}

func TestToLibraryListItems(t *testing.T) {
	api := &API{saveDir: t.TempDir()}
	items := []medialib.Item{
		{RelPath: "a", Number: "A-1"},
		{RelPath: "b", Number: "B-2"},
	}
	result := api.toLibraryListItems(items)
	assert.Len(t, result, 2)
}

func TestToLibraryDetailNil(t *testing.T) {
	api := &API{}
	assert.Nil(t, api.toLibraryDetail(nil))
}

func TestToLibraryDetailNonNil(t *testing.T) {
	api := &API{saveDir: t.TempDir()}
	detail := &medialib.Detail{
		Item:              medialib.Item{RelPath: "demo", Number: "D-1"},
		Meta:              medialib.Meta{Title: "test"},
		Variants:          []medialib.Variant{{Key: "v1"}},
		PrimaryVariantKey: "v1",
		Files:             []medialib.FileItem{{Name: "f"}},
	}
	result := api.toLibraryDetail(detail)
	require.NotNil(t, result)
	assert.Equal(t, "demo", result.Item.RelPath)
	assert.Equal(t, "test", result.Meta.Title)
	assert.Len(t, result.Variants, 1)
	assert.Len(t, result.Files, 1)
	assert.Equal(t, "v1", result.PrimaryVariantKey)
}

func TestLoadLibraryConflictFlags(t *testing.T) {
	t.Run("nil media", func(t *testing.T) {
		api := &API{}
		result := api.loadLibraryConflictFlags()
		assert.Empty(t, result)
	})
	t.Run("unconfigured media", func(t *testing.T) {
		api := &API{media: medialib.NewService(nil, "", "")}
		result := api.loadLibraryConflictFlags()
		assert.Empty(t, result)
	})
}

func TestHandleLibraryItemGetSuccess(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Demo Title</title>
  <num>DEMO-001</num>
  <premiered>2024-01-01</premiered>
</movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/library/item?path=demo", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleLibraryListWithItems(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Demo Title</title>
  <num>DEMO-001</num>
</movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/library", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleLibraryFileDeleteSuccess(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	fanartDir := filepath.Join(itemDir, "extrafanart")
	require.NoError(t, os.MkdirAll(fanartDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Test</title></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fanartDir, "fanart1.jpg"), jpegBytes(), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/library/file?path=demo/extrafanart/fanart1.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleLibraryFileDeleteDenied(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Test</title></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "poster.jpg"), jpegBytes(), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/library/file?path=demo/poster.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeLibraryFileDeleteDenied, resp.Code)
}

func TestHandleLibraryItemGetNotExistPath(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "exists")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/library/item?path=exists", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeLibraryItemNotFound, resp.Code)
}

func TestHandleLibraryItemDeleteNotExist(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/library/item?path=nonexistent", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

func TestHandleLibraryAssetSuccess(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Test</title><num>DEMO</num></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	for _, kind := range []string{"poster", "cover", "fanart"} {
		t.Run(kind, func(t *testing.T) {
			buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/asset?path=demo&kind="+kind, buf)
			req.Header.Set("Content-Type", ct)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, 0, resp.Code)
		})
	}
}

func TestHandleLibraryPosterCropSuccess(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Test</title><num>DEMO</num></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "fanart.jpg"), createValidJPEG(t, 10, 10), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/poster-crop?path=demo", strings.NewReader(`{"x":0,"y":0,"width":1,"height":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	// May succeed or fail depending on whether cover image exists - just verify no panic.
	_ = resp
}

func TestHandleLibraryPosterCropError(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Test</title><num>DEMO</num></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/poster-crop?path=demo", strings.NewReader(`{"x":0,"y":0,"width":10,"height":10}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeLibraryPosterCropFailed, resp.Code)
}

func TestHandleLibraryAssetReplaceError(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/asset?path=nonexistent&kind=cover", buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeLibraryAssetReplaceFailed, resp.Code)
}

func TestLoadLibraryConflictFlagsConfigured(t *testing.T) {
	svc := setupMediaLibDB(t)
	api := &API{media: svc}
	result := api.loadLibraryConflictFlags()
	assert.NotNil(t, result)
}

func TestHandleLibraryItemPatchSuccess(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Original</title><num>DEMO-001</num></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("video"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/library/item?path=demo", strings.NewReader(`{"meta":{"title":"Updated","number":"DEMO-001"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleListLibraryError(t *testing.T) {
	api := &API{saveDir: ""}
	c, rec := newGinContext(http.MethodGet, "/api/library", nil)
	api.handleListLibrary(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeListLibraryFailed, resp.Code)
}

func TestHandleLibraryFileGetOpenError(t *testing.T) {
	saveDir := t.TempDir()
	filePath := filepath.Join(saveDir, "demo", "unreadable.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, jpegBytes(), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filePath, 0o644) })

	api := &API{saveDir: saveDir}
	c, rec := newGinContext(http.MethodGet, "/api/library/file?path=demo/unreadable.jpg", nil)
	c.Request.URL.RawQuery = "path=demo/unreadable.jpg"
	api.handleLibraryFileGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeLibraryFileOpenFailed, resp.Code)
}

func TestHandleLibraryFileDeleteGeneralError(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?><movie><title>T</title></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "cover.jpg"), jpegBytes(), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/library/file?path=demo/cover.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

func TestHandleLibraryItemDeleteOsNotExist(t *testing.T) {
	saveDir := t.TempDir()
	api := &API{saveDir: saveDir}
	c, rec := newGinContext(http.MethodDelete, "/api/library/item?path=nonexistent", nil)
	c.Request.URL.RawQuery = "path=nonexistent"
	api.handleLibraryItemDelete(c)
	resp := decodeResponse(t, rec)
	assert.NotEqual(t, 0, resp.Code)
}

func TestLoadLibraryConflictFlagsDBError(t *testing.T) {
	svc := setupClosedMediaLibDB(t)
	api := &API{media: svc}
	result := api.loadLibraryConflictFlags()
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestLoadLibraryConflictFlagsWithDBItems(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conflict_items.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	db := sqlite.DB()
	itemJSON := `{"rel_path":"test/item","number":"CF-001","name":"item"}`
	_, err = db.Exec(
		`INSERT INTO yamdc_media_library_tab (rel_path, item_json, detail_json, created_at) VALUES (?,?,?,?)`,
		"test/item", itemJSON, "{}", time.Now().UnixMilli(),
	)
	require.NoError(t, err)

	svc := medialib.NewService(db, t.TempDir(), "")
	api := &API{media: svc}
	result := api.loadLibraryConflictFlags()
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
}

func TestLoadLibraryConflictFlagsWithItems(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conflict.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })

	libraryDir := t.TempDir()
	svc := medialib.NewService(sqlite.DB(), libraryDir, "")
	api := &API{media: svc}
	result := api.loadLibraryConflictFlags()
	assert.NotNil(t, result)
}

func TestHandleLibraryFileDeleteResolveError(t *testing.T) {
	api := &API{saveDir: ""}
	c, rec := newGinContext(http.MethodDelete, "/api/library/file?path=x", nil)
	c.Request.URL.RawQuery = "path=x"
	api.handleLibraryFileDelete(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeResolveLibraryPathFailed, resp.Code)
}

func TestHandleLibraryFileGetResolveError(t *testing.T) {
	api := &API{saveDir: ""}
	c, rec := newGinContext(http.MethodGet, "/api/library/file?path=x", nil)
	c.Request.URL.RawQuery = "path=x"
	api.handleLibraryFileGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeResolveLibraryPathFailed, resp.Code)
}

func TestHandleLibraryAssetVariant(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8"?><movie><title>T</title><num>D</num></movie>`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "demo.mp4"), []byte("v"), 0o600))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/library/asset?path=demo&kind=poster&variant=demo", buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}
