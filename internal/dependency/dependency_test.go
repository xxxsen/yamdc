package dependency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestResolve_EmptyDeps(t *testing.T) {
	err := Resolve(context.Background(), &mockHTTPClient{}, nil)
	require.NoError(t, err)
}

func TestResolve_SingleDep_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("model-data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "model.bin")

	cli := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}
	deps := []*Dependency{
		{URL: srv.URL + "/model.bin", Target: target, Refresh: false},
	}
	err := Resolve(context.Background(), cli, deps)
	require.NoError(t, err)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "model-data", string(data))
}

func TestResolve_MultipleDeps_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content-" + r.URL.Path))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cli := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}
	deps := []*Dependency{
		{URL: srv.URL + "/a.bin", Target: filepath.Join(dir, "a.bin"), Refresh: false},
		{URL: srv.URL + "/b.bin", Target: filepath.Join(dir, "b.bin"), Refresh: false},
	}
	err := Resolve(context.Background(), cli, deps)
	require.NoError(t, err)

	for _, dep := range deps {
		_, err := os.Stat(dep.Target)
		assert.NoError(t, err)
	}
}

func TestResolve_DownloadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cli := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}
	deps := []*Dependency{
		{URL: srv.URL + "/bad", Target: filepath.Join(dir, "bad.bin"), Refresh: false},
	}
	err := Resolve(context.Background(), cli, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download")
}

func TestResolve_Refresh_AlreadyExists(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "file.bin")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))

	cli := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}

	deps := []*Dependency{
		{URL: srv.URL + "/file.bin", Target: target, Refresh: true},
	}
	err := Resolve(context.Background(), cli, deps)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestResolve_NoRefresh_AlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not download when file exists and refresh=false")
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "file.bin")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))

	cli := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}
	deps := []*Dependency{
		{URL: srv.URL + "/file.bin", Target: target, Refresh: false},
	}
	err := Resolve(context.Background(), cli, deps)
	require.NoError(t, err)
}

func TestResolve_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cli := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}
	deps := []*Dependency{
		{URL: srv.URL + "/good", Target: filepath.Join(dir, "good.bin"), Refresh: false},
		{URL: srv.URL + "/bad", Target: filepath.Join(dir, "bad.bin"), Refresh: false},
	}
	err := Resolve(context.Background(), cli, deps)
	require.Error(t, err)
}
