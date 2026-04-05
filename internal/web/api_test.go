package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

type testHandlerFn func(ctx context.Context, fc *model.FileContext) error

func (fn testHandlerFn) Handle(ctx context.Context, fc *model.FileContext) error {
	return fn(ctx, fc)
}

func phandlerDebugRuntime() appdeps.Runtime {
	return appdeps.Runtime{
		Storage: store.NewMemStorage(),
	}
}

func TestParseStatusesDefault(t *testing.T) {
	items := parseStatuses("")
	require.Equal(t, []jobdef.Status{
		jobdef.StatusInit,
		jobdef.StatusProcessing,
		jobdef.StatusFailed,
		jobdef.StatusReviewing,
	}, items)
}

func TestParseStatusesCustom(t *testing.T) {
	items := parseStatuses("init, failed ,reviewing")
	require.Equal(t, []jobdef.Status{
		jobdef.StatusInit,
		jobdef.StatusFailed,
		jobdef.StatusReviewing,
	}, items)
}

func TestHandleAssetDetectContentType(t *testing.T) {
	memStore := store.NewMemStorage()
	require.NoError(t, store.PutDataTo(context.Background(), memStore, "img-key", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}))

	req := httptest.NewRequest(http.MethodGet, "/api/assets/img-key", nil)
	rec := httptest.NewRecorder()

	api := &API{store: memStore}
	api.handleAsset(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"))
}

func TestHandleNumberCleanerExplain(t *testing.T) {
	rs, err := numbercleaner.LoadRuleSetFromPath("../../rules/ruleset")
	require.NoError(t, err)
	cl, err := numbercleaner.NewCleaner(rs)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/debug/number-cleaner/explain", strings.NewReader(`{"input":"fc2ppv12345 中文字幕"}`))
	rec := httptest.NewRecorder()

	api := &API{cleaner: cl}
	api.handleNumberCleanerExplain(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Code int                         `json:"code"`
		Data numbercleaner.ExplainResult `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "FC2-PPV-12345-C", payload.Data.Final.Normalized)
	require.NotEmpty(t, payload.Data.Steps)
}

func TestHandleNumberCleanerExplainInvalidInputReturnsBizError(t *testing.T) {
	rs, err := numbercleaner.LoadRuleSetFromPath("../../rules/ruleset")
	require.NoError(t, err)
	cl, err := numbercleaner.NewCleaner(rs)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/debug/number-cleaner/explain", strings.NewReader(`{"input":""}`))
	rec := httptest.NewRecorder()

	api := &API{cleaner: cl}
	api.handleNumberCleanerExplain(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, errCodeInputRequired, payload.Code)
	require.Equal(t, "input is required", payload.Message)
}

func TestHandleHandlerDebugRun(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/debug/handler/run", strings.NewReader(`{
		"mode":"single",
		"handler_id":"number_title",
		"meta":{"number":"ABC-123","title":"sample title"}
	}`))
	rec := httptest.NewRecorder()

	api := &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), numbercleaner.NewPassthroughCleaner(), []string{"number_title"}, map[string]phandler.DebugHandlerOption{})}
	api.handleHandlerDebugRun(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Code int                  `json:"code"`
		Data phandler.DebugResult `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "ABC-123 sample title", payload.Data.AfterMeta.Title)
}

func TestHandleHandlerDebugRunChain(t *testing.T) {
	phandler.Register("test_chain_fail", func(args interface{}, deps appdeps.Runtime) (phandler.IHandler, error) {
		return testHandlerFn(func(ctx context.Context, fc *model.FileContext) error {
			fc.Meta.Title = fc.Meta.Title + "-failed"
			return fmt.Errorf("boom")
		}), nil
	})
	phandler.Register("test_chain_ok", func(args interface{}, deps appdeps.Runtime) (phandler.IHandler, error) {
		return testHandlerFn(func(ctx context.Context, fc *model.FileContext) error {
			fc.Meta.Title = fc.Meta.Title + "-ok"
			return nil
		}), nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/debug/handler/run", strings.NewReader(`{
		"mode":"chain",
		"handler_ids":["test_chain_fail","test_chain_ok"],
		"meta":{"number":"ABC-123","title":"sample title"}
	}`))
	rec := httptest.NewRecorder()

	api := &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), numbercleaner.NewPassthroughCleaner(), []string{"test_chain_fail", "test_chain_ok"}, map[string]phandler.DebugHandlerOption{})}
	api.handleHandlerDebugRun(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Code int                  `json:"code"`
		Data phandler.DebugResult `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "chain", payload.Data.Mode)
	require.Len(t, payload.Data.Steps, 2)
	require.Equal(t, "1 handlers failed", payload.Data.Error)
	require.Equal(t, "sample title-failed-ok", payload.Data.AfterMeta.Title)
	require.Equal(t, "boom", payload.Data.Steps[0].Error)
	require.Empty(t, payload.Data.Steps[1].Error)
}

func TestEngineBuildDoesNotPanic(t *testing.T) {
	api := &API{}
	require.NotPanics(t, func() {
		engine, err := api.Engine(":0")
		require.NoError(t, err)
		require.NotNil(t, engine)
	})
}

func TestEngineHealthzRoute(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "ok", payload.Data.Status)
}

func TestEngineCORSPreflight(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodOptions, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "OPTIONS")
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestEngineJobRunRoute(t *testing.T) {
	dbPath := t.TempDir() + "/app.db"
	sqlite, err := repository.NewSQLite(dbPath)
	require.NoError(t, err)
	defer func() { _ = sqlite.Close() }()

	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage())

	err = jobRepo.UpsertScannedJob(context.Background(), repository.UpsertJobInput{
		FileName:      "abc",
		FileExt:       ".mp4",
		RelPath:       "abc.mp4",
		AbsPath:       "/tmp/abc.mp4",
		Number:        "ABC-123",
		RawNumber:     "ABC-123",
		CleanedNumber: "ABC-123",
		NumberSource:  "raw",
		FileSize:      1,
	})
	require.NoError(t, err)
	items, err := jobRepo.ListJobs(context.Background(), []jobdef.Status{jobdef.StatusInit}, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, items.Items, 1)
	item := items.Items[0]

	api := NewAPI(jobRepo, nil, jobSvc, "", nil, store.NewMemStorage(), nil, nil, nil)
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/jobs/%d/run", item.ID), nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "job started", payload.Message)

	// Run 路由会异步启动任务，等待后台 goroutine 收尾，避免 TempDir 清理时目录仍被占用导致 CI 偶发失败。
	require.Eventually(t, func() bool {
		current, getErr := jobRepo.GetByID(context.Background(), item.ID)
		require.NoError(t, getErr)
		if current == nil {
			return false
		}
		return current.Status != jobdef.StatusProcessing
	}, 3*time.Second, 50*time.Millisecond)
}

func TestEngineReviewSaveRoute(t *testing.T) {
	dbPath := t.TempDir() + "/app.db"
	sqlite, err := repository.NewSQLite(dbPath)
	require.NoError(t, err)
	defer func() { _ = sqlite.Close() }()

	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage())

	err = jobRepo.UpsertScannedJob(context.Background(), repository.UpsertJobInput{
		FileName:      "review",
		FileExt:       ".mp4",
		RelPath:       "review.mp4",
		AbsPath:       "/tmp/review.mp4",
		Number:        "ABC-456",
		RawNumber:     "ABC-456",
		CleanedNumber: "ABC-456",
		NumberSource:  "raw",
		FileSize:      1,
	})
	require.NoError(t, err)
	items, err := jobRepo.ListJobs(context.Background(), []jobdef.Status{jobdef.StatusInit}, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, items.Items, 1)
	item := items.Items[0]
	updated, err := jobRepo.UpdateStatus(context.Background(), item.ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, updated)
	err = scrapeRepo.UpsertRawData(context.Background(), item.ID, "test", `{"number":"ABC-456","title":"raw"}`)
	require.NoError(t, err)
	err = scrapeRepo.SaveReviewData(context.Background(), item.ID, `{"number":"ABC-456","title":"old"}`)
	require.NoError(t, err)

	api := NewAPI(jobRepo, nil, jobSvc, "", nil, store.NewMemStorage(), nil, nil, nil)
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/review/jobs/%d", item.ID), strings.NewReader(`{"review_data":"{\"number\":\"ABC-456\",\"title\":\"new\"}"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "review data saved", payload.Message)

	saved, err := scrapeRepo.GetByJobID(context.Background(), item.ID)
	require.NoError(t, err)
	require.Contains(t, saved.ReviewData, `"title":"new"`)
}

func TestEngineAssetRouteGet(t *testing.T) {
	memStore := store.NewMemStorage()
	require.NoError(t, store.PutDataTo(context.Background(), memStore, "img-key", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}))

	api := &API{store: memStore}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/img-key", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"))
}

func TestEngineLibraryFileRouteGet(t *testing.T) {
	saveDir := t.TempDir()
	filePath := filepath.Join(saveDir, "demo", "cover.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte{0xff, 0xd8, 0xff, 0xdb}, 0o644))

	api := &API{saveDir: saveDir}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/library/file?path=demo/cover.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"))
	require.Equal(t, "no-store, no-cache, must-revalidate", resp.Header.Get("Cache-Control"))
	require.Equal(t, "no-cache", resp.Header.Get("Pragma"))
	require.Equal(t, "0", resp.Header.Get("Expires"))
}

func TestEngineLibraryFileRouteGetNotFound(t *testing.T) {
	api := &API{saveDir: t.TempDir()}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/library/file?path=missing.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, errCodeLibraryFileNotFound, payload.Code)
	require.NotEmpty(t, payload.Message)
}

func TestEngineMediaLibraryFileRouteGet(t *testing.T) {
	libraryDir := t.TempDir()
	filePath := filepath.Join(libraryDir, "movie", "poster.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte{0xff, 0xd8, 0xff, 0xdb}, 0o644))

	api := &API{media: medialib.NewService(nil, libraryDir, "")}
	engine, err := api.Engine(":0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/media-library/file?path=movie/poster.jpg", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"))
	require.Equal(t, "no-store, no-cache, must-revalidate", resp.Header.Get("Cache-Control"))
	require.Equal(t, "no-cache", resp.Header.Get("Pragma"))
	require.Equal(t, "0", resp.Header.Get("Expires"))
}
