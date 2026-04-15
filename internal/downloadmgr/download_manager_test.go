package downloadmgr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- mock HTTP client ----------

type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func defaultMockClient(srv *httptest.Server) *mockHTTPClient {
	return &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return srv.Client().Do(req) //nolint:bodyclose
		},
	}
}

// ---------- NewManager ----------

func TestNewManager(t *testing.T) {
	cli := &mockHTTPClient{}
	m := NewManager(cli)
	require.NotNil(t, m)
	assert.Equal(t, cli, m.cli)
}

// ---------- ensureDir ----------

func TestEnsureDir_CreatesParent(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "sub", "deep", "file.txt")
	m := NewManager(nil)
	require.NoError(t, m.ensureDir(dst))

	_, err := os.Stat(filepath.Join(dir, "sub", "deep"))
	assert.NoError(t, err)
}

func TestEnsureDir_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(nil)
	require.NoError(t, m.ensureDir(filepath.Join(dir, "file.txt")))
}

// ---------- writeToFile ----------

func TestWriteToFile_Success(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.txt")
	m := NewManager(nil)

	f, err := os.CreateTemp(dir, "src")
	require.NoError(t, err)
	_, _ = f.WriteString("hello world")
	_, _ = f.Seek(0, 0)
	defer func() { _ = f.Close() }()

	require.NoError(t, m.writeToFile(f, dst))
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))

	_, err = os.Stat(dst + ".temp")
	assert.True(t, os.IsNotExist(err))
}

// ---------- metaFilePath ----------

func TestMetaFilePath(t *testing.T) {
	assert.Equal(t, "/tmp/file.bin.meta", metaFilePath("/tmp/file.bin"))
}

// ---------- readFileMeta ----------

func TestReadFileMeta_NotExist(t *testing.T) {
	_, err := readFileMeta(filepath.Join(t.TempDir(), "nope.meta"))
	require.Error(t, err)
	assert.ErrorIs(t, err, errMetaFileNotFound)
}

func TestReadFileMeta_InvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.meta")
	require.NoError(t, os.WriteFile(p, []byte("not-json"), 0o644))
	_, err := readFileMeta(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal meta")
}

func TestReadFileMeta_Success(t *testing.T) {
	p := filepath.Join(t.TempDir(), "good.meta")
	raw, _ := json.Marshal(&fileMeta{ETag: `"abc"`, LastModified: "Mon, 01 Jan 2024 00:00:00 GMT"})
	require.NoError(t, os.WriteFile(p, raw, 0o644))

	meta, err := readFileMeta(p)
	require.NoError(t, err)
	assert.Equal(t, `"abc"`, meta.ETag)
	assert.Equal(t, "Mon, 01 Jan 2024 00:00:00 GMT", meta.LastModified)
}

// ---------- writeFileMeta ----------

func TestWriteFileMeta_Success(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.meta")
	m := NewManager(nil)
	require.NoError(t, m.writeFileMeta(p, &fileMeta{ETag: `"e"`, LastModified: "lm"}))

	meta, err := readFileMeta(p)
	require.NoError(t, err)
	assert.Equal(t, `"e"`, meta.ETag)
}

// ---------- attachETag ----------

func TestAttachETag_NoMetaFile(t *testing.T) {
	m := NewManager(nil)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, m.attachETag(req, filepath.Join(t.TempDir(), "nope.meta")))
	assert.Empty(t, req.Header.Get("If-None-Match"))
}

func TestAttachETag_WithMeta(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	raw, _ := json.Marshal(&fileMeta{ETag: `"abc"`, LastModified: "lm-value"})
	require.NoError(t, os.WriteFile(metaPath, raw, 0o644))

	m := NewManager(nil)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, m.attachETag(req, metaPath))
	assert.Equal(t, `"abc"`, req.Header.Get("If-None-Match"))
	assert.Equal(t, "lm-value", req.Header.Get("If-Modified-Since"))
}

func TestAttachETag_EmptyETagAndLastModified(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	raw, _ := json.Marshal(&fileMeta{})
	require.NoError(t, os.WriteFile(metaPath, raw, 0o644))

	m := NewManager(nil)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, m.attachETag(req, metaPath))
	assert.Empty(t, req.Header.Get("If-None-Match"))
	assert.Empty(t, req.Header.Get("If-Modified-Since"))
}

// ---------- isFileExist ----------

func TestIsFileExist(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "exists.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))

	m := NewManager(nil)
	assert.True(t, m.isFileExist(f))
	assert.False(t, m.isFileExist(filepath.Join(dir, "nope")))
}

// ---------- Download ----------

func TestDownload_FileExistsNoSync(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o644))

	m := NewManager(&mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			t.Fatal("should not download")
			return nil, nil
		},
	})
	updated, err := m.Download(context.Background(), "http://example.com/file.bin", dst, false)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestDownload_NewFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"etag-1"`)
		w.Header().Set("Last-Modified", "Tue, 01 Jan 2025 00:00:00 GMT")
		_, _ = w.Write([]byte("new-content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")

	m := NewManager(defaultMockClient(srv))
	updated, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, false)
	require.NoError(t, err)
	assert.True(t, updated)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new-content", string(data))

	meta, err := readFileMeta(metaFilePath(dst))
	require.NoError(t, err)
	assert.Equal(t, `"etag-1"`, meta.ETag)
}

func TestDownload_Sync_NotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"etag-1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte("should-not-reach"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	require.NoError(t, os.WriteFile(dst, []byte("cached"), 0o644))
	metaRaw, _ := json.Marshal(&fileMeta{ETag: `"etag-1"`})
	require.NoError(t, os.WriteFile(metaFilePath(dst), metaRaw, 0o644))

	m := NewManager(defaultMockClient(srv))
	updated, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, true)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestDownload_Sync_Updated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"etag-2"`)
		_, _ = w.Write([]byte("new-version"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o644))
	metaRaw, _ := json.Marshal(&fileMeta{ETag: `"etag-1"`})
	require.NoError(t, os.WriteFile(metaFilePath(dst), metaRaw, 0o644))

	m := NewManager(defaultMockClient(srv))
	updated, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, true)
	require.NoError(t, err)
	assert.True(t, updated)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new-version", string(data))
}

func TestDownload_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")

	m := NewManager(defaultMockClient(srv))
	_, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, errUnexpectedStatusCode)
}

func TestDownload_DoError(t *testing.T) {
	m := NewManager(&mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	})
	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	_, err := m.Download(context.Background(), "http://example.com/file.bin", dst, false)
	require.Error(t, err)
}

func TestWriteToFile_CopyError(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.txt")
	m := NewManager(nil)

	r := &errReader{err: errors.New("read fail")}
	err := m.writeToFile(r, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transfer data failed")
}

type errReader struct{ err error }

func (r *errReader) Read(_ []byte) (int, error) { return 0, r.err }

func TestEnsureDir_InvalidPath(t *testing.T) {
	m := NewManager(nil)
	err := m.ensureDir("/dev/null/impossible/file.txt")
	require.Error(t, err)
}

func TestWriteFileMeta_BadPath(t *testing.T) {
	m := NewManager(nil)
	err := m.writeFileMeta("/dev/null/impossible/bad.meta", &fileMeta{ETag: "e"})
	require.Error(t, err)
}

func TestAttachETag_InvalidMetaFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.meta")
	require.NoError(t, os.WriteFile(p, []byte("not-json"), 0o644))
	m := NewManager(nil)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	err := m.attachETag(req, p)
	require.Error(t, err)
}

func TestDownload_SyncFileExists_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"new-etag"`)
		w.Header().Set("Last-Modified", "Mon, 01 Jan 2025 00:00:00 GMT")
		_, _ = w.Write([]byte("new-content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o644))

	m := NewManager(defaultMockClient(srv))
	updated, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, true)
	require.NoError(t, err)
	assert.True(t, updated)

	data, _ := os.ReadFile(dst)
	assert.Equal(t, "new-content", string(data))

	meta, err := readFileMeta(metaFilePath(dst))
	require.NoError(t, err)
	assert.Equal(t, `"new-etag"`, meta.ETag)
	assert.Equal(t, "Mon, 01 Jan 2025 00:00:00 GMT", meta.LastModified)
}

func TestReadFileMeta_ReadError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "unreadable.meta")
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	_, err := readFileMeta(p)
	require.Error(t, err)
	assert.NotErrorIs(t, err, errMetaFileNotFound)
}

func TestDownload_EnsureDirError(t *testing.T) {
	m := NewManager(&mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			t.Fatal("should not reach")
			return nil, nil
		},
	})
	_, err := m.Download(context.Background(), "http://example.com/f", "/dev/null/bad/file.bin", false)
	require.Error(t, err)
}

func TestDownload_AttachETagError(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	metaPath := metaFilePath(dst)
	require.NoError(t, os.WriteFile(metaPath, []byte("not-json"), 0o644))

	m := NewManager(&mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			t.Fatal("should not reach")
			return nil, nil
		},
	})
	_, err := m.Download(context.Background(), "http://example.com/f", dst, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attach etag")
}

func TestDownload_BuildReaderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "zstd")
		_, _ = w.Write([]byte("not-zstd-data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")

	m := NewManager(defaultMockClient(srv))
	_, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, false)
	require.Error(t, err)
}

func TestWriteToFile_RenameError(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(dstDir, 0o755))
	tempFile := filepath.Join(dstDir, "out.txt.temp")

	require.NoError(t, os.WriteFile(tempFile, []byte("existing"), 0o644))

	targetDir := filepath.Join(dstDir, "blocked")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	dst := filepath.Join(dstDir, "blocked")
	m := NewManager(nil)

	f, err := os.CreateTemp(dir, "src")
	require.NoError(t, err)
	_, _ = f.WriteString("data")
	_, _ = f.Seek(0, 0)
	defer func() { _ = f.Close() }()

	err = m.writeToFile(f, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to move file")
}

func TestDownload_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")
	m := NewManager(&mockHTTPClient{})
	_, err := m.Download(context.Background(), "://bad-url", dst, false)
	require.Error(t, err)
}

func TestDownload_WriteMetaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"e"`)
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "file.bin")

	m := NewManager(defaultMockClient(srv))
	updated, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, false)
	require.NoError(t, err)
	assert.True(t, updated)

	meta, err := readFileMeta(metaFilePath(dst))
	require.NoError(t, err)
	assert.Equal(t, `"e"`, meta.ETag)
}

func TestDownload_WriteToFileError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	subdir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	dst := filepath.Join(subdir, "file.bin")

	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	require.NoError(t, os.WriteFile(dst+".temp", []byte("x"), 0o644))
	require.NoError(t, os.Chmod(subdir, 0o444))
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })

	m := NewManager(defaultMockClient(srv))
	_, err := m.Download(context.Background(), srv.URL+"/file.bin", dst, false)
	require.Error(t, err)
}
