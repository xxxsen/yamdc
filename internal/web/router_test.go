package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestEngineCORSPreflight(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestCORSMiddlewareNonOptions(t *testing.T) {
	api := &API{}
	engine, err := api.Engine(":0")
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}
