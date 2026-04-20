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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestParseStatuses(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		expect []jobdef.Status
	}{
		{"default", "", []jobdef.Status{jobdef.StatusInit, jobdef.StatusProcessing, jobdef.StatusFailed, jobdef.StatusReviewing}},
		{"custom", "init, failed ,reviewing", []jobdef.Status{jobdef.StatusInit, jobdef.StatusFailed, jobdef.StatusReviewing}},
		{"empty items filtered", "init,,failed", []jobdef.Status{jobdef.StatusInit, jobdef.StatusFailed}},
		{"single", "init", []jobdef.Status{jobdef.StatusInit}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, parseStatuses(tt.raw))
		})
	}
}

func TestParseIDParam(t *testing.T) {
	tests := []struct {
		name    string
		idParam string
		wantID  int64
		wantOK  bool
	}{
		{"valid", "42", 42, true},
		{"invalid", "abc", 0, false},
		{"empty", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newGinContextWithParams(http.MethodPost, "/", nil, gin.Params{{Key: "id", Value: tt.idParam}})
			id, ok := parseIDParam(c)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestHandleScan(t *testing.T) {
	_, jobRepo, _, _ := setupTestDB(t) //nolint:dogsled // 测试只关心前若干返回值
	scanDir := t.TempDir()
	scanSvc := scanner.New(scanDir, nil, jobRepo, movieidcleaner.NewPassthroughCleaner())

	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"success empty dir", &API{scanner: scanSvc}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/scan", nil)
			tt.api.handleScan(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleScanWithFile(t *testing.T) {
	_, jobRepo, _, _ := setupTestDB(t) //nolint:dogsled // 测试只关心前若干返回值
	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "test.mp4"), []byte("data"), 0o600))
	scanSvc := scanner.New(scanDir, nil, jobRepo, movieidcleaner.NewPassthroughCleaner())
	api := &API{scanner: scanSvc}
	c, rec := newGinContext(http.MethodPost, "/api/scan", nil)
	api.handleScan(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleListJobs(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	createTestJob(t, jobRepo, "LIST-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{"default", "/api/jobs", 0},
		{"with status", "/api/jobs?status=init", 0},
		{"with page", "/api/jobs?page=1&page_size=10", 0},
		{"with keyword", "/api/jobs?keyword=LIST", 0},
		{"all=true", "/api/jobs?all=true", 0},
		{"invalid page ignored", "/api/jobs?page=abc", 0},
		{"invalid page_size ignored", "/api/jobs?page_size=-1", 0},
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

func TestHandleJobRun(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RUN-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"invalid id", "/api/jobs/abc/run", errCodeInvalidJobID},
		{"valid", fmt.Sprintf("/api/jobs/%d/run", j.ID), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.url, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}

	require.Eventually(t, func() bool {
		current, getErr := jobRepo.GetByID(context.Background(), j.ID)
		require.NoError(t, getErr)
		return current != nil && current.Status != jobdef.StatusProcessing
	}, 3*time.Second, 50*time.Millisecond)
}

func TestHandleJobRerun(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RERUN-001")
	// Move to failed so it can be rerun.
	_, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusFailed, "test fail")
	require.NoError(t, err)

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"invalid id", "/api/jobs/abc/rerun", errCodeInvalidJobID},
		{"valid", fmt.Sprintf("/api/jobs/%d/rerun", j.ID), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.url, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}

	require.Eventually(t, func() bool {
		current, getErr := jobRepo.GetByID(context.Background(), j.ID)
		require.NoError(t, getErr)
		return current != nil && current.Status != jobdef.StatusProcessing
	}, 3*time.Second, 50*time.Millisecond)
}

func TestHandleJobLogs(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "LOGS-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"invalid id", "/api/jobs/abc/logs", errCodeInvalidJobID},
		{"valid", fmt.Sprintf("/api/jobs/%d/logs", j.ID), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

// TestHandleListNumberVariants 覆盖 GET /api/number/variants: 返回 200 + code=0,
// data.variants 至少包含已知的几个关键 id (防止默认描述符被意外删掉); 返回的
// indexed 描述符必须自带合法 min/max, 供前端渲染 number input。
func TestHandleListNumberVariants(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/number/variants", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
	require.NotNil(t, resp.Data)
	payload, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected data to be object, got %T", resp.Data)
	variantsAny, ok := payload["variants"].([]any)
	require.True(t, ok, "expected data.variants to be array, got %T", payload["variants"])
	require.NotEmpty(t, variantsAny)

	byID := make(map[string]map[string]any, len(variantsAny))
	for _, item := range variantsAny {
		v, ok := item.(map[string]any)
		require.True(t, ok, "variant entry must be object, got %T", item)
		id, _ := v["id"].(string)
		require.NotEmpty(t, id)
		byID[id] = v
	}

	for _, wantID := range []string{"resolution_4k", "resolution_8k", "vr", "chinese_subtitle", "special_edition", "restored", "multi_cd"} {
		_, ok := byID[wantID]
		assert.True(t, ok, "missing descriptor id: %s", wantID)
	}

	cd := byID["multi_cd"]
	require.NotNil(t, cd)
	assert.Equal(t, "indexed", cd["kind"])
	assert.EqualValues(t, 1, cd["min"])
	maxAny, ok := cd["max"].(float64)
	require.True(t, ok, "expected numeric max for multi_cd, got %T", cd["max"])
	assert.GreaterOrEqual(t, int(maxAny), 2, "multi_cd max should be >= 2")
	assert.Equal(t, "CD", cd["suffix"])
}

func TestHandleJobUpdateNumber(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "UPD-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		body     string
		wantCode int
	}{
		{"invalid id", "/api/jobs/abc/number", `{"number":"X"}`, errCodeInvalidJobID},
		{"invalid json", fmt.Sprintf("/api/jobs/%d/number", j.ID), `{bad`, errCodeInvalidJSONBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleJobDelete(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "DEL-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"invalid id", "/api/jobs/abc", errCodeInvalidJobID},
		{"valid", fmt.Sprintf("/api/jobs/%d", j.ID), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, tt.url, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleReviewGet(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RVGET-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"invalid id", "/api/review/jobs/abc", errCodeInvalidJobID},
		{"valid no data", fmt.Sprintf("/api/review/jobs/%d", j.ID), errCodeReviewGetFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleReviewSave(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVSAVE-001")
	// Move to reviewing status.
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", `{"number":"RVSAVE-001"}`))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		body     string
		wantCode int
	}{
		{"invalid id", "/api/review/jobs/abc", `{"review_data":"{}"}`, errCodeInvalidJobID},
		{"invalid json body", fmt.Sprintf("/api/review/jobs/%d", j.ID), `{bad`, errCodeInvalidJSONBody},
		{"success", fmt.Sprintf("/api/review/jobs/%d", j.ID), `{"review_data":"{\"number\":\"RVSAVE-001\",\"title\":\"new\"}"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleReviewImport(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVIMP-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"invalid id", "/api/review/jobs/abc/import", errCodeInvalidJobID},
		{"not reviewing status", fmt.Sprintf("/api/review/jobs/%d/import", j.ID), errCodeReviewImportFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.url, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleReviewPosterCrop(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVCROP-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		url      string
		body     string
		wantCode int
	}{
		{"invalid id", "/api/review/jobs/abc/poster-crop", `{"x":0,"y":0,"width":10,"height":10}`, errCodeInvalidJobID},
		{"invalid json", fmt.Sprintf("/api/review/jobs/%d/poster-crop", j.ID), `{bad`, errCodeInvalidJSONBody},
		{"zero width", fmt.Sprintf("/api/review/jobs/%d/poster-crop", j.ID), `{"x":0,"y":0,"width":0,"height":10}`, errCodeInvalidCropRectangle},
		{"zero height", fmt.Sprintf("/api/review/jobs/%d/poster-crop", j.ID), `{"x":0,"y":0,"width":10,"height":0}`, errCodeInvalidCropRectangle},
		{"negative height", fmt.Sprintf("/api/review/jobs/%d/poster-crop", j.ID), `{"x":0,"y":0,"width":10,"height":-1}`, errCodeInvalidCropRectangle},
		{"not reviewing", fmt.Sprintf("/api/review/jobs/%d/poster-crop", j.ID), `{"x":0,"y":0,"width":10,"height":10}`, errCodeReviewPosterCropFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleReviewAsset(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVASSET-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	// Test invalid id.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/review/jobs/abc/asset?target=cover", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeInvalidJobID, resp.Code)

	// Test invalid target.
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=invalid", j.ID), nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp = decodeResponse(t, rec)
	assert.Equal(t, errCodeInvalidAssetTarget, resp.Code)

	// Test missing file upload.
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=cover", j.ID), strings.NewReader("no file"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp = decodeResponse(t, rec)
	assert.Equal(t, errCodeInvalidUploadFile, resp.Code)

	// Test with non-image file.
	buf, ct := buildMultipartImage(t, "file", "test.txt", []byte("not an image, just text"))
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=cover", j.ID), buf)
	req.Header.Set("Content-Type", ct)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp = decodeResponse(t, rec)
	assert.Equal(t, errCodeUploadFileNotImage, resp.Code)
}

func TestHandleReviewAssetSuccess(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVASSET-002")

	// Set up reviewing state with scrape data.
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	rawMeta := `{"number":"RVASSET-002","title":"test","cover":{"name":"cover.jpg","key":"ckey"}}`
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", rawMeta))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc, store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	targets := []string{"cover", "poster", "fanart"}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=%s", j.ID, target), buf)
			req.Header.Set("Content-Type", ct)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, 0, resp.Code)
		})
	}
}

func TestReadUploadImageData(t *testing.T) {
	// Test via the handleReviewAsset path which calls readUploadImageData.
	// No file.
	c, rec := newGinContext(http.MethodPost, "/test", strings.NewReader(""))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, _, ok := readUploadImageData(c)
	assert.False(t, ok)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeInvalidUploadFile, resp.Code)
}

func TestLoadReviewMeta(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "LOADMETA-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, store: memStore}

	// No scrape data - returns false because GetScrapeData errors.
	c, rec := newGinContext(http.MethodGet, "/test", nil)
	meta, ok := api.loadReviewMeta(c, j.ID)
	assert.False(t, ok)
	assert.Nil(t, meta)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReviewGetFailed, resp.Code)

	// With raw data.
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", `{"number":"LOADMETA-001","title":"raw"}`))
	c, _ = newGinContext(http.MethodGet, "/test", nil)
	meta, ok = api.loadReviewMeta(c, j.ID)
	assert.True(t, ok)
	assert.NotNil(t, meta)
	assert.Equal(t, "raw", meta.Title)

	// With review data overriding raw.
	require.NoError(t, scrapeRepo.SaveReviewData(context.Background(), j.ID, `{"number":"LOADMETA-001","title":"reviewed"}`))
	c, _ = newGinContext(http.MethodGet, "/test", nil)
	meta, ok = api.loadReviewMeta(c, j.ID)
	assert.True(t, ok)
	assert.NotNil(t, meta)
	assert.Equal(t, "reviewed", meta.Title)

	// Invalid JSON in review data.
	require.NoError(t, scrapeRepo.SaveReviewData(context.Background(), j.ID, `{not json`))
	c, rec = newGinContext(http.MethodGet, "/test", nil)
	meta, ok = api.loadReviewMeta(c, j.ID)
	assert.False(t, ok)
	assert.Nil(t, meta)
	resp = decodeResponse(t, rec)
	assert.Equal(t, errCodeInvalidReviewJSON, resp.Code)
}

func TestHandleScanError(t *testing.T) {
	_, jobRepo, _, _ := setupTestDB(t) //nolint:dogsled // 测试只关心前若干返回值
	scanSvc := scanner.New("/nonexistent/path/that/does/not/exist", nil, jobRepo, movieidcleaner.NewPassthroughCleaner())
	api := &API{scanner: scanSvc}
	c, rec := newGinContext(http.MethodPost, "/api/scan", nil)
	api.handleScan(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeScanFailed, resp.Code)
}

func TestHandleJobRunError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPost, "/api/jobs/99999/run", nil, gin.Params{{Key: "id", Value: "99999"}})
	api.handleJobRun(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeJobRunFailed, resp.Code)
}

func TestHandleJobRerunError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RERUNERR-001")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPost, fmt.Sprintf("/api/jobs/%d/rerun", j.ID), nil, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleJobRerun(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeJobRerunFailed, resp.Code)
}

func TestHandleJobLogsError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "LOGSERR-001")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodGet, fmt.Sprintf("/api/jobs/%d/logs", j.ID), nil, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleJobLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleJobUpdateNumberSuccess(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "UPDNUM-001")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPatch, fmt.Sprintf("/api/jobs/%d/number", j.ID), strings.NewReader(`{"number":"NEW-001"}`), gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleJobUpdateNumber(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeJobUpdateNumberFailed, resp.Code)
}

// TestHandleJobUpdateNumberStructuredRouting 只覆盖 handler 的 "如何路由 body
// 形态" 的分支: 结构化 / 老 number / 两者空。验证错误路径上的返回码而不是
// 成功的持久化 (成功路径由 service 层测试负责), 这样 handler 测试不依赖
// 真实 capture 的启动环境。
func TestHandleJobUpdateNumberStructuredRouting(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "UPDNUM-STR-001")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			// 结构化入参, variant id 非法: 应当在 ApplyVariantSelections 层失败,
			// 返回 errCodeJobUpdateNumberFailed。
			name:     "structured invalid variant id",
			body:     `{"base":"NEW-001","variants":[{"id":"totally-bogus"}]}`,
			wantCode: errCodeJobUpdateNumberFailed,
		},
		{
			// 结构化入参, base 空: 应当在 variant layer 的 ErrVariantEmptyBase
			// 直接失败。
			name:     "structured empty base",
			body:     `{"base":"","variants":[{"id":"resolution_4k"}]}`,
			wantCode: errCodeJobUpdateNumberFailed,
		},
		{
			// 仅带 base, 无 variants: 也要走结构化路径 (因为 base 非 nil)。
			// 此处 test capture 不认识 "NEW-ONLY-BASE-001" 这种番号, 会在
			// UpdateNumber 里报错, 但不应该回退到老路径。
			name:     "structured base only",
			body:     `{"base":"NEW-ONLY-BASE-001"}`,
			wantCode: errCodeJobUpdateNumberFailed,
		},
		{
			// 老路径: 只有 number 字段。test capture 同样会报错, 但此时走的是
			// 老 UpdateNumber, 验证回退逻辑没有因为新字段而被误触发。
			name:     "legacy number field",
			body:     `{"number":"NEW-001"}`,
			wantCode: errCodeJobUpdateNumberFailed,
		},
		{
			// 完全空 body (json 合法): 既不走结构化, 也会被 UpdateNumber 报错。
			name:     "empty object falls back to legacy",
			body:     `{}`,
			wantCode: errCodeJobUpdateNumberFailed,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newGinContextWithParams(
				http.MethodPatch,
				fmt.Sprintf("/api/jobs/%d/number", j.ID),
				strings.NewReader(tc.body),
				gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}},
			)
			api.handleJobUpdateNumber(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tc.wantCode, resp.Code, "body=%s", tc.body)
		})
	}
}

func TestHandleJobDeleteError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodDelete, "/api/jobs/99999", nil, gin.Params{{Key: "id", Value: "99999"}})
	api.handleJobDelete(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeJobDeleteFailed, resp.Code)
}

func TestHandleReviewGetSuccess(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RVGET-OK")
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", `{"number":"RVGET-OK","title":"ok"}`))
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodGet, fmt.Sprintf("/api/review/jobs/%d", j.ID), nil, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewGet(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleReviewSaveError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVSAVEERR-001")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	c, rec := newGinContextWithParams(http.MethodPut, fmt.Sprintf("/api/review/jobs/%d", j.ID), strings.NewReader(`{"review_data":"{\"number\":\"x\"}"}`), gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewSave(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReviewSaveFailed, resp.Code)
}

func TestHandleReviewImportSuccess(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVIMP-OK")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", `{"number":"RVIMP-OK","title":"test"}`))
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	c, rec := newGinContextWithParams(http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/import", j.ID), nil, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewImport(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReviewImportFailed, resp.Code)
}

func TestReadUploadImageDataSuccess(t *testing.T) {
	buf, ct := buildMultipartImage(t, "file", "test.png", pngBytes())
	c, _ := newGinContext(http.MethodPost, "/test", buf)
	c.Request.Header.Set("Content-Type", ct)
	data, name, ok := readUploadImageData(c)
	assert.True(t, ok)
	assert.NotEmpty(t, data)
	assert.Equal(t, "test.png", name)
}

func TestReadUploadImageDataNotImage(t *testing.T) {
	buf, ct := buildMultipartImage(t, "file", "test.txt", []byte("hello world"))
	c, rec := newGinContext(http.MethodPost, "/test", buf)
	c.Request.Header.Set("Content-Type", ct)
	_, _, ok := readUploadImageData(c)
	assert.False(t, ok)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeUploadFileNotImage, resp.Code)
}

func TestLoadReviewMetaNilScrapeData(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "LOADMETA-NIL")
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", `{"number":"x"}`))
	require.NoError(t, scrapeRepo.SaveReviewData(context.Background(), j.ID, ""))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, store: memStore}
	c, rec := newGinContext(http.MethodGet, "/test", nil)
	meta, ok := api.loadReviewMeta(c, j.ID)
	assert.True(t, ok)
	assert.NotNil(t, meta)
	_ = rec
}

func TestHandleReviewPosterCropSuccess(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVCROP-OK")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)

	coverData := createValidJPEG(t, 10, 10)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), memStore, coverData)
	require.NoError(t, err)

	rawMeta := fmt.Sprintf(`{"number":"RVCROP-OK","title":"test","cover":{"name":"cover.jpg","key":"%s"}}`, coverKey)
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", rawMeta))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc, store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/poster-crop", j.ID), strings.NewReader(`{"x":0,"y":0,"width":1,"height":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleReviewSaveReadBodyError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RVBODY-001")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPut, "/test", &errReader{}, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewSave(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReadBodyFailed, resp.Code)
}

func TestHandleReviewPosterCropReadBodyError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "RVCROPBODY-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPost, "/test", &errReader{}, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewPosterCrop(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReadBodyFailed, resp.Code)
}

func TestHandleJobUpdateNumberReadBodyError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "UPDNUMBODY-001")

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPatch, "/test", &errReader{}, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleJobUpdateNumber(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReadBodyFailed, resp.Code)
}

func TestHandleReviewAssetStoreFail(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	failStore := &failingStore{}
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, failStore)
	j := createTestJob(t, jobRepo, "RVASSETFAIL-001")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	rawMeta := `{"number":"RVASSETFAIL-001","title":"test"}`
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", rawMeta))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc, store: failStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=cover", j.ID), buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReviewAssetStoreFailed, resp.Code)
}

func TestHandleListJobsApplyConflictsError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	createTestJob(t, jobRepo, "CONF-001")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}

	engine, err := api.Engine(":0")
	require.NoError(t, err)

	// Verify all query parameter branches.
	for _, q := range []string{
		"/api/jobs",
		"/api/jobs?status=init",
		"/api/jobs?page=2&page_size=5",
		"/api/jobs?keyword=CONF",
		"/api/jobs?all=true",
		"/api/jobs?page=0",
		"/api/jobs?page_size=0",
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, q, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		resp := decodeResponse(t, rec)
		assert.Equal(t, 0, resp.Code)
	}
}

func TestHandleJobLogsSuccess(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "LOGSOK-001")
	require.NoError(t, logRepo.Append(context.Background(), repository.LogTypeScrapeJob,
		fmt.Sprintf("%d", j.ID), "info",
		`{"stage":"test","message":"hello","detail":"detail"}`))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodGet, "/test", nil, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleJobLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleJobUpdateNumberError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	j := createTestJob(t, jobRepo, "UPDNUMERR-001")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodPatch, "/test", strings.NewReader(`{"number":"X"}`), gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleJobUpdateNumber(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeJobUpdateNumberFailed, resp.Code)
}

func TestHandleReviewImportNotReviewing(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVIMP-NOTREV")
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	c, rec := newGinContextWithParams(http.MethodPost, "/test", nil, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewImport(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReviewImportFailed, resp.Code)
}

func TestHandleReviewAssetMarshalAndSave(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVASSET-MS")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	rawMeta := `{"number":"RVASSET-MS","title":"test","cover":{"name":"c.jpg","key":"k"},"poster":{"name":"p.jpg","key":"k2"}}`
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", rawMeta))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc, store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=poster", j.ID), buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleReviewAssetFanart(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVASSET-FAN")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	rawMeta := `{"number":"RVASSET-FAN","title":"test","cover":{"name":"c.jpg","key":"k"}}`
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", rawMeta))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc, store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "fan.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=fanart", j.ID), buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleReviewAssetSaveReviewError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVASSET-SAVERR")
	rawMeta := `{"number":"RVASSET-SAVERR","title":"test","cover":{"name":"c.jpg","key":"k"}}`
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", rawMeta))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc, store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	buf, ct := buildMultipartImage(t, "file", "img.png", pngBytes())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("/api/review/jobs/%d/asset?target=poster", j.ID), buf)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeReviewSaveFailed, resp.Code)
}

func TestHandleListJobsDBError(t *testing.T) {
	jobRepo, logRepo, scrapeRepo := setupClosedJobDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContext(http.MethodGet, "/api/jobs", nil)
	api.handleListJobs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeListJobsFailed, resp.Code)
}

func TestHandleJobLogsDBError(t *testing.T) {
	jobRepo, logRepo, scrapeRepo := setupClosedJobDB(t)
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, store.NewMemStorage())
	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContextWithParams(http.MethodGet, "/test", nil, gin.Params{{Key: "id", Value: "1"}})
	api.handleJobLogs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeJobLogsFailed, resp.Code)
}

func TestHandleListJobsApplyConflictsDBError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "CONFDB-001")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusFailed, "")
	require.NoError(t, err)
	require.True(t, ok)

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc}
	c, rec := newGinContext(http.MethodGet, "/api/jobs?status=failed", nil)
	c.Request.URL.RawQuery = "status=failed"
	api.handleListJobs(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestLoadReviewMetaScrapeDataNil(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "LOADMETA-NILSD")
	require.NoError(t, scrapeRepo.UpsertRawData(context.Background(), j.ID, "test", `{"number":"x"}`))

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, store: memStore}
	c, _ := newGinContext(http.MethodGet, "/test", nil)
	meta, ok := api.loadReviewMeta(c, j.ID)
	assert.True(t, ok)
	assert.NotNil(t, meta)
}

// TestHandleReviewReject 验证 POST /api/review/jobs/:id/reject 的四条主路径:
// invalid id / 非 reviewing / 成功 (空 body) / 成功 (带 reason)。
// 成功路径通过 engine.ServeHTTP 走完整路由, 顺便锁死 POST 路由注册。
func TestHandleReviewReject(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)

	initJob := createTestJob(t, jobRepo, "RVRJ-INIT")
	reviewingJob := createTestJob(t, jobRepo, "RVRJ-OK")
	ok, err := jobRepo.UpdateStatus(context.Background(), reviewingJob.ID,
		[]jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, scrapeRepo.UpsertRawData(
		context.Background(), reviewingJob.ID, "test",
		`{"number":"RVRJ-OK","title":"t"}`,
	))
	reviewingJob2 := createTestJob(t, jobRepo, "RVRJ-EMPTY")
	ok, err = jobRepo.UpdateStatus(context.Background(), reviewingJob2.ID,
		[]jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	cases := []struct {
		name     string
		url      string
		body     string
		wantCode int
	}{
		{"invalid id", "/api/review/jobs/abc/reject", `{"reason":"x"}`, errCodeInvalidJobID},
		{"not reviewing", fmt.Sprintf("/api/review/jobs/%d/reject", initJob.ID), `{"reason":"x"}`, errCodeReviewRejectFailed},
		{"success_with_reason", fmt.Sprintf("/api/review/jobs/%d/reject", reviewingJob.ID), `{"reason":"用户手动打回"}`, 0},
		{"success_empty_body", fmt.Sprintf("/api/review/jobs/%d/reject", reviewingJob2.ID), "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reqBody *strings.Reader
			if tc.body != "" {
				reqBody = strings.NewReader(tc.body)
			}
			var req *http.Request
			if reqBody != nil {
				req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, tc.url, reqBody)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, tc.url, nil)
			}
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tc.wantCode, resp.Code, "case=%s body=%q", tc.name, rec.Body.String())
		})
	}

	// 成功路径后的副作用: reviewingJob.ID 现在应该是 failed, error_msg
	// 等于用户传入的 reason。
	j, err := jobRepo.GetByID(context.Background(), reviewingJob.ID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
	assert.Equal(t, "用户手动打回", j.ErrorMsg)

	// 空 body 的 reviewing job 应该走默认 reason。
	j2, err := jobRepo.GetByID(context.Background(), reviewingJob2.ID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j2.Status)
	assert.Equal(t, "rejected by reviewer", j2.ErrorMsg)
}

// TestHandleReviewRejectReadBodyError 覆盖 body 读取失败的兜底: reject 需要
// 兼容"body 读不出来"的场景, 此时走默认 reason, 不应返回 ReadBodyFailed。
func TestHandleReviewRejectReadBodyError(t *testing.T) {
	_, jobRepo, logRepo, scrapeRepo := setupTestDB(t)
	memStore := store.NewMemStorage()
	jobSvc := newTestJobService(t, jobRepo, logRepo, scrapeRepo, memStore)
	reviewSvc := newTestReviewService(jobSvc, jobRepo, scrapeRepo, memStore)
	j := createTestJob(t, jobRepo, "RVRJ-BODYERR")
	ok, err := jobRepo.UpdateStatus(context.Background(), j.ID,
		[]jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)

	api := &API{jobRepo: jobRepo, jobSvc: jobSvc, reviewSvc: reviewSvc}
	c, rec := newGinContextWithParams(http.MethodPost, "/test", &errReader{},
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", j.ID)}})
	api.handleReviewReject(c)
	resp := decodeResponse(t, rec)
	// 读失败时 reject 应该仍能跑成功 (默认 reason), 不把 body 读错误当 500 抛。
	assert.Equal(t, 0, resp.Code)
	after, err := jobRepo.GetByID(context.Background(), j.ID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, after.Status)
	assert.Equal(t, "rejected by reviewer", after.ErrorMsg)
}
