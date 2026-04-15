package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPlugin_OnGetHosts(t *testing.T) {
	var p DefaultPlugin
	assert.Empty(t, p.OnGetHosts(context.Background()))
}

func TestDefaultPlugin_OnPrecheckRequest(t *testing.T) {
	var p DefaultPlugin
	ok, err := p.OnPrecheckRequest(context.Background(), "ANY-123")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDefaultPlugin_OnMakeHTTPRequest(t *testing.T) {
	var p DefaultPlugin
	req, err := p.OnMakeHTTPRequest(context.Background(), "ANY-123")
	assert.Nil(t, req)
	assert.ErrorIs(t, err, errNoImpl)
}

func TestDefaultPlugin_OnDecorateRequest(t *testing.T) {
	var p DefaultPlugin
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, p.OnDecorateRequest(context.Background(), req))
}

func TestDefaultPlugin_OnPrecheckResponse(t *testing.T) {
	var p DefaultPlugin
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	rsp404 := &http.Response{StatusCode: http.StatusNotFound, Request: req}
	ok, err := p.OnPrecheckResponse(context.Background(), req, rsp404)
	require.NoError(t, err)
	assert.False(t, ok)

	rsp200 := &http.Response{StatusCode: http.StatusOK, Request: req}
	ok, err = p.OnPrecheckResponse(context.Background(), req, rsp200)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDefaultPlugin_OnHandleHTTPRequest(t *testing.T) {
	var p DefaultPlugin
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	want := &http.Response{StatusCode: http.StatusTeapot, Body: io.NopCloser(nil)}
	invoker := func(ctx context.Context, r *http.Request) (*http.Response, error) {
		assert.Equal(t, req, r)
		return want, nil
	}
	got, err := p.OnHandleHTTPRequest(context.Background(), invoker, req)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDefaultPlugin_OnDecodeHTTPData(t *testing.T) {
	var p DefaultPlugin
	meta, ok, err := p.OnDecodeHTTPData(context.Background(), []byte("<html/>"))
	assert.Nil(t, meta)
	assert.False(t, ok)
	assert.ErrorIs(t, err, errNoImpl)
}

func TestDefaultPlugin_OnDecorateMediaRequest(t *testing.T) {
	var p DefaultPlugin
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, p.OnDecorateMediaRequest(context.Background(), req))
}
