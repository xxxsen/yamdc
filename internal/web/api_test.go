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
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	plugineditor "github.com/xxxsen/yamdc/internal/searcher/plugin/editor"
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
		AbsPath:       filepath.Join(t.TempDir(), "abc.mp4"),
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

	api := NewAPI(jobRepo, nil, jobSvc, "", nil, store.NewMemStorage(), nil, nil, nil, nil)
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
		AbsPath:       filepath.Join(t.TempDir(), "review.mp4"),
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

	api := NewAPI(jobRepo, nil, jobSvc, "", nil, store.NewMemStorage(), nil, nil, nil, nil)
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

func TestHandlePluginEditorCompile(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}

	req := httptest.NewRequest(http.MethodPost, "/api/debug/plugin-editor/compile", strings.NewReader(`{
		"draft":{
			"version":1,
			"name":"fixture",
			"type":"one-step",
			"hosts":["https://fixture.example"],
			"request":{"method":"GET","path":"/search/${number}"},
			"scrape":{
				"format":"html",
				"fields":{
					"title":{
						"selector":{"kind":"xpath","expr":"//title/text()"},
						"parser":"string",
						"required":true
					}
				}
			}
		}
	}`))
	rec := httptest.NewRecorder()
	api.handlePluginEditorCompile(rec, req)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			OK   bool `json:"ok"`
			Data struct {
				YAML string `json:"yaml"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.True(t, payload.Data.OK)
	require.Contains(t, payload.Data.Data.YAML, "version: 1")
}

func TestHandlePluginEditorScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1 class="title"> Sample Title </h1></body></html>`))
	}))
	defer srv.Close()

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}

	body := fmt.Sprintf(`{
		"number":"ABC-123",
		"draft":{
			"version":1,
			"name":"fixture",
			"type":"one-step",
			"hosts":["%s"],
			"request":{"method":"GET","path":"/search/${number}"},
			"scrape":{
				"format":"html",
				"fields":{
					"title":{
						"selector":{"kind":"xpath","expr":"//h1[@class='title']/text()"},
						"transforms":[{"kind":"trim"}],
						"parser":"string",
						"required":true
					}
				}
			}
		}
	}`, srv.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/debug/plugin-editor/scrape", strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.handlePluginEditorScrape(rec, req)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			OK   bool `json:"ok"`
			Data struct {
				Meta struct {
					Title string `json:"title"`
				} `json:"meta"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.True(t, payload.Data.OK)
	require.Equal(t, "Sample Title", payload.Data.Data.Meta.Title)
}

func TestHandlePluginEditorWorkflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`<html><body><a class="item" href="/detail/1">ABC-123 title</a></body></html>`))
		case "/detail/1":
			_, _ = w.Write([]byte(`<html><body><h1 class="title">Target</h1></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}

	body := fmt.Sprintf(`{
		"number":"ABC-123",
		"draft":{
			"version":1,
			"name":"fixture-two-step",
			"type":"two-step",
			"hosts":["%s"],
			"request":{"method":"GET","path":"/search"},
			"workflow":{
				"search_select":{
					"selectors":[
						{"name":"read_link","kind":"xpath","expr":"//a[@class='item']/@href"},
						{"name":"read_title","kind":"xpath","expr":"//a[@class='item']/text()"}
					],
					"match":{
						"mode":"and",
						"conditions":["contains(\"${item.read_title}\", \"${number}\")"],
						"expect_count":1
					},
					"return":"${item.read_link}",
					"next_request":{"method":"GET","path":"${build_url(${host}, ${value})}"}
				}
			},
			"scrape":{
				"format":"html",
				"fields":{"title":{"selector":{"kind":"xpath","expr":"//h1/text()"},"parser":"string","required":true}}
			}
		}
	}`, srv.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/debug/plugin-editor/workflow", strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.handlePluginEditorWorkflow(rec, req)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			OK   bool `json:"ok"`
			Data struct {
				Steps []struct {
					Stage string `json:"stage"`
				} `json:"steps"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.True(t, payload.Data.OK)
	require.Len(t, payload.Data.Data.Steps, 3)
}

func TestHandlePluginEditorCase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1 class="title"> Sample Title </h1><div class="actors"><span>Alice</span></div></body></html>`))
	}))
	defer srv.Close()

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}

	body := fmt.Sprintf(`{
		"draft":{
			"version":1,
			"name":"fixture",
			"type":"one-step",
			"hosts":["%s"],
			"request":{"method":"GET","path":"/search/${number}"},
			"scrape":{
				"format":"html",
				"fields":{
					"title":{"selector":{"kind":"xpath","expr":"//h1[@class='title']/text()"},"transforms":[{"kind":"trim"}],"parser":"string","required":true},
					"actors":{"selector":{"kind":"xpath","expr":"//div[@class='actors']/span/text()","multi":true},"parser":"string_list","required":false}
				}
			}
		},
		"case":{
			"name":"case-1",
			"input":"ABC-123",
			"output":{"title":"Sample Title","actor_set":["Alice"],"status":"success"}
		}
	}`, srv.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/debug/plugin-editor/case", strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.handlePluginEditorCase(rec, req)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			OK   bool `json:"ok"`
			Data struct {
				Result struct {
					Pass bool `json:"pass"`
				} `json:"result"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.True(t, payload.Data.OK)
	require.True(t, payload.Data.Data.Result.Pass)
}

func TestHandlePluginEditorImport(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}

	req := httptest.NewRequest(http.MethodPost, "/api/debug/plugin-editor/import", strings.NewReader(`{
		"yaml":"version: 1\nname: fixture\ntype: one-step\nhosts:\n  - https://fixture.example\nrequest:\n  method: GET\n  path: /search/${number}\nscrape:\n  format: html\n  fields:\n    title:\n      selector:\n        kind: xpath\n        expr: //title/text()\n      parser: string\n      required: true\n"
	}`))
	rec := httptest.NewRecorder()
	api.handlePluginEditorImport(rec, req)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			OK   bool `json:"ok"`
			Data struct {
				Draft struct {
					Name string `json:"name"`
				} `json:"draft"`
			} `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))
	require.Equal(t, 0, payload.Code)
	require.True(t, payload.Data.OK)
	require.Equal(t, "fixture", payload.Data.Data.Draft.Name)
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
