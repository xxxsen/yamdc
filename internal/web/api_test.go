package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/store"
)

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

func TestParseJobRoute(t *testing.T) {
	id, action, err := parseJobRoute("/api/jobs/42/run", "/api/jobs/")
	require.NoError(t, err)
	require.EqualValues(t, 42, id)
	require.Equal(t, "run", action)
}

func TestParseJobRouteNoAction(t *testing.T) {
	id, action, err := parseJobRoute("/api/review/jobs/7", "/api/review/jobs/")
	require.NoError(t, err)
	require.EqualValues(t, 7, id)
	require.Equal(t, "", action)
}

func TestParseJobRouteInvalid(t *testing.T) {
	_, _, err := parseJobRoute("/api/jobs/abc/run", "/api/jobs/")
	require.Error(t, err)
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

func TestHandleHandlerDebugRun(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/debug/handler/run", strings.NewReader(`{
		"handler_id":"number_title",
		"meta":{"number":"ABC-123","title":"sample title"}
	}`))
	rec := httptest.NewRecorder()

	api := &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), numbercleaner.NewPassthroughCleaner(), []string{"number_title"}, map[string]config.HandlerConfig{})}
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
