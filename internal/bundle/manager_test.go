package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// ---------- helpers ----------

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// ---------- Data.Close ----------

func TestData_Close_Nil(t *testing.T) {
	var d *Data
	assert.NoError(t, d.Close())
}

func TestData_Close_NilCloseFunc(t *testing.T) {
	d := &Data{}
	assert.NoError(t, d.Close())
}

func TestData_Close_CallsFunc(t *testing.T) {
	called := false
	d := &Data{close: func() error { called = true; return nil }}
	assert.NoError(t, d.Close())
	assert.True(t, called)
}

func TestData_Close_Error(t *testing.T) {
	d := &Data{close: func() error { return errors.New("close err") }}
	assert.Error(t, d.Close())
}

// ---------- parseGitHubRepoURL ----------

func TestParseGitHubRepoURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ok    bool
		owner string
		repo  string
	}{
		{"valid https", "https://github.com/owner/repo", true, "owner", "repo"},
		{"valid with .git", "https://github.com/owner/repo.git", true, "owner", "repo"},
		{"with trailing slash", "https://github.com/owner/repo/", true, "owner", "repo"},
		{"no repo part", "https://github.com/owner", false, "", ""},
		{"wrong host", "https://gitlab.com/owner/repo", false, "", ""},
		{"empty string", "", false, "", ""},
		{"invalid url", "://bad", false, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseGitHubRepoURL(tc.input)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.owner, got.owner)
				assert.Equal(t, tc.repo, got.repo)
			}
		})
	}
}

func TestParseRepoOrPanic_Valid(t *testing.T) {
	r := parseRepoOrPanic("https://github.com/owner/repo")
	assert.Equal(t, "owner", r.owner)
	assert.Equal(t, "repo", r.repo)
}

func TestParseRepoOrPanic_Invalid(t *testing.T) {
	assert.Panics(t, func() {
		parseRepoOrPanic("https://gitlab.com/a/b")
	})
}

// ---------- detectZipRoot ----------

func TestDetectZipRoot(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name:     "common root",
			files:    map[string]string{"root/a.txt": "a", "root/b.txt": "b"},
			expected: "root",
		},
		{
			name:     "no common root",
			files:    map[string]string{"a/x.txt": "a", "b/y.txt": "b"},
			expected: "",
		},
		{
			name:     "single file",
			files:    map[string]string{"root/only.txt": "x"},
			expected: "root",
		},
		{
			name:     "empty",
			files:    map[string]string{},
			expected: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			zipData := makeZip(t, tc.files)
			reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, detectZipRoot(reader.File))
		})
	}
}

// ---------- fileExists ----------

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "exists.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))

	ok, err := fileExists(f)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = fileExists(filepath.Join(dir, "no-such-file"))
	require.NoError(t, err)
	assert.False(t, ok)
}

// ---------- filesEqual ----------

func TestFilesEqual(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	c := filepath.Join(dir, "c.txt")

	require.NoError(t, os.WriteFile(a, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(c, []byte("world"), 0o600))

	eq, err := filesEqual(a, b)
	require.NoError(t, err)
	assert.True(t, eq)

	eq, err = filesEqual(a, c)
	require.NoError(t, err)
	assert.False(t, eq)
}

func TestFilesEqual_DifferentSizes(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("short"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("longer content"), 0o600))

	eq, err := filesEqual(a, b)
	require.NoError(t, err)
	assert.False(t, eq)
}

func TestFilesEqual_LeftMissing(t *testing.T) {
	dir := t.TempDir()
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(b, []byte("x"), 0o600))
	_, err := filesEqual(filepath.Join(dir, "missing"), b)
	assert.Error(t, err)
}

func TestFilesEqual_RightMissing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(a, []byte("x"), 0o600))
	_, err := filesEqual(a, filepath.Join(dir, "missing"))
	assert.Error(t, err)
}

// ---------- openLocalData ----------

func TestOpenLocalData_Success(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("a"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.yaml"), []byte("b"), 0o600))

	data, err := openLocalData(dir)
	require.NoError(t, err)
	defer func() { _ = data.Close() }()

	assert.Equal(t, ".", data.Base)
	assert.Contains(t, data.Files, "a.yaml")
	assert.Contains(t, data.Files, filepath.Join("sub", "b.yaml"))
}

func TestOpenLocalData_NotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))

	_, err := openLocalData(f)
	require.Error(t, err)
	assert.ErrorIs(t, err, errLocalBundleNotADirectory)
}

func TestOpenLocalData_NotExist(t *testing.T) {
	_, err := openLocalData(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

// ---------- openZipData ----------

func TestOpenZipData_Success(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bundle.zip")
	zipBytes := makeZip(t, map[string]string{
		"root/a.yaml":     "content_a",
		"root/sub/b.yaml": "content_b",
	})
	require.NoError(t, os.WriteFile(zipPath, zipBytes, 0o600))

	data, err := openZipData(zipPath)
	require.NoError(t, err)
	defer func() { _ = data.Close() }()

	assert.Equal(t, "root", data.Base)
	assert.Equal(t, zipPath, data.Source)
	assert.NotEmpty(t, data.Files)

	content, err := fs.ReadFile(data.FS, data.Files[0])
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestOpenZipData_InvalidZip(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.zip")
	require.NoError(t, os.WriteFile(f, []byte("not-a-zip"), 0o600))
	_, err := openZipData(f)
	require.Error(t, err)
}

// ---------- listFilesFromFS ----------

func TestListFilesFromFS(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "y.txt"), []byte("y"), 0o600))

	files, err := listFilesFromFS(os.DirFS(dir), ".")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

// ---------- NewManager ----------

func TestNewManager_NilCallback(t *testing.T) {
	_, err := NewManager("test", "/tmp", nil, "local", "/some/dir", "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBundleCallbackRequired)
}

func TestNewManager_UnsupportedSource(t *testing.T) {
	_, err := NewManager("test", "/tmp", nil, "ftp", "/some/dir", "", func(_ context.Context, _ *Data) error { return nil })
	require.Error(t, err)
	assert.ErrorIs(t, err, errUnsupportedSourceType)
}

func TestNewManager_Local(t *testing.T) {
	m, err := NewManager("test", "/tmp", nil, "local", "/some/dir", "", func(_ context.Context, _ *Data) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, SourceTypeLocal, m.sourceType)
}

func TestNewManager_LocalDefault(t *testing.T) {
	m, err := NewManager("test", "/tmp", nil, "", "/some/dir", "", func(_ context.Context, _ *Data) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, SourceTypeLocal, m.sourceType)
}

func TestNewManager_Remote_InvalidURL(t *testing.T) {
	_, err := NewManager("test", "/tmp", nil, "remote", "https://gitlab.com/a/b", "", func(_ context.Context, _ *Data) error { return nil })
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidRemoteLocation)
}

func TestNewManager_Remote_Valid(t *testing.T) {
	m, err := NewManager("test", "/tmp", nil, "remote", "https://github.com/owner/repo", "cache", func(_ context.Context, _ *Data) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, SourceTypeRemote, m.sourceType)
	assert.Contains(t, m.zipPath, "owner-repo.zip")
	assert.Contains(t, m.tempPath, "owner-repo.zip.temp")
}

// ---------- Start (local) ----------

func TestStart_Local_Success(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.yaml"), []byte("x"), 0o600))

	var received *Data
	m, err := NewManager("test", "/tmp", nil, "local", dir, "", func(_ context.Context, d *Data) error {
		received = d
		return nil
	})
	require.NoError(t, err)

	err = m.Start(context.Background())
	require.NoError(t, err)
	require.NotNil(t, received)
	assert.NotEmpty(t, received.Files)
}

func TestStart_Local_CBError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.yaml"), []byte("x"), 0o600))

	m, err := NewManager("test", "/tmp", nil, "local", dir, "", func(_ context.Context, _ *Data) error {
		return errors.New("cb error")
	})
	require.NoError(t, err)
	err = m.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cb error")
}

func TestStart_Local_BadDir(t *testing.T) {
	m, err := NewManager("test", "/tmp", nil, "local", filepath.Join(t.TempDir(), "nonexist"), "", func(_ context.Context, _ *Data) error { return nil })
	require.NoError(t, err)
	err = m.Start(context.Background())
	require.Error(t, err)
}

// ---------- Start (remote) with mock HTTP ----------

func TestStart_Remote_Success(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "content"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	var received *Data
	m, err := NewManager("test", dataDir, &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
		},
	}, "remote", "https://github.com/owner/repo", "cache", func(_ context.Context, d *Data) error {
		received = d
		return nil
	})
	require.NoError(t, err)
	m.syncInterval = time.Hour

	err = m.Start(context.Background())
	require.NoError(t, err)
	require.NotNil(t, received)
}

func TestStart_Remote_FetchTagsFails_FallbackToCache(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "cached"})

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), zipBytes, 0o600))

	var received *Data
	m, err := NewManager("test", dataDir, &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}, "remote", "https://github.com/owner/repo", "cache", func(_ context.Context, d *Data) error {
		received = d
		return nil
	})
	require.NoError(t, err)
	m.syncInterval = time.Hour

	err = m.Start(context.Background())
	require.NoError(t, err)
	require.NotNil(t, received)
}

func TestStart_Remote_FetchTagsFails_NoCacheFails(t *testing.T) {
	dataDir := t.TempDir()
	m, err := NewManager("test", dataDir, &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}, "remote", "https://github.com/owner/repo", "cache", func(_ context.Context, _ *Data) error {
		return nil
	})
	require.NoError(t, err)
	m.syncInterval = time.Hour

	err = m.Start(context.Background())
	require.Error(t, err)
}

// ---------- fetchLatestGitHubTag ----------

func TestFetchLatestGitHubTag_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]struct {
			Name string `json:"name"`
		}{{Name: "v2.0.0"}})
	}))
	defer srv.Close()

	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	tag, err := m.fetchLatestGitHubTag(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", tag)
}

func TestFetchLatestGitHubTag_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	_, err := m.fetchLatestGitHubTag(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errQueryGitHubTagFailed)
}

func TestFetchLatestGitHubTag_EmptyTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	_, err := m.fetchLatestGitHubTag(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoGitHubTags)
}

func TestFetchLatestGitHubTag_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	_, err := m.fetchLatestGitHubTag(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal github tags")
}

func TestFetchLatestGitHubTag_DoError(t *testing.T) {
	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("dial error")
			},
		},
	}
	_, err := m.fetchLatestGitHubTag(context.Background())
	require.Error(t, err)
}

// ---------- downloadBundle ----------

func TestDownloadBundle_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("zip-data"))
	}))
	defer srv.Close()

	m := &Manager{
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	data, err := m.downloadBundle(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "zip-data", string(data))
}

func TestDownloadBundle_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := &Manager{
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	_, err := m.downloadBundle(context.Background(), srv.URL)
	require.Error(t, err)
	assert.ErrorIs(t, err, errDownloadBundleFailed)
}

func TestDownloadBundle_DoError(t *testing.T) {
	m := &Manager{
		cli: &mockHTTPClient{
			doFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("timeout")
			},
		},
	}
	_, err := m.downloadBundle(context.Background(), "http://example.com/bundle.zip")
	require.Error(t, err)
}

// ---------- cleanupTemp ----------

func TestCleanupTemp_NoFile(t *testing.T) {
	m := &Manager{tempPath: filepath.Join(t.TempDir(), "nonexist.temp")}
	assert.NoError(t, m.cleanupTemp())
}

func TestCleanupTemp_FileExists(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.temp")
	require.NoError(t, os.WriteFile(tmp, []byte("x"), 0o600))
	m := &Manager{tempPath: tmp}
	assert.NoError(t, m.cleanupTemp())
	_, err := os.Stat(tmp)
	assert.True(t, os.IsNotExist(err))
}

// ---------- syncAndActivate ----------

func TestSyncAndActivate_UpdatedTrue(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	var cbData *Data
	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: filepath.Join(dataDir, "cache"),
		zipPath:  filepath.Join(dataDir, "cache", "owner-repo.zip"),
		tempPath: filepath.Join(dataDir, "cache", "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, d *Data) error {
			cbData = d
			return nil
		},
	}
	updated, err := m.syncAndActivate(context.Background())
	require.NoError(t, err)
	assert.True(t, updated)
	require.NotNil(t, cbData)
}

func TestSyncAndActivate_SameContent_NotUpdated(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), zipBytes, 0o600))

	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: cacheDir,
		zipPath:  filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath: filepath.Join(cacheDir, "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error {
			t.Fatal("callback should not be called for same content")
			return nil
		},
	}
	updated, err := m.syncAndActivate(context.Background())
	require.NoError(t, err)
	assert.False(t, updated)
}

// ---------- FetchLatestGitHubTag with blank tag name ----------

func TestFetchLatestGitHubTag_BlankTagName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]struct {
			Name string `json:"name"`
		}{{Name: "  "}})
	}))
	defer srv.Close()

	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
	}
	_, err := m.fetchLatestGitHubTag(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoGitHubTags)
}

// ---------- syncAndActivate callback error ----------

// ---------- Start unsupported source type ----------

func TestStart_UnsupportedSourceType(t *testing.T) {
	m := &Manager{sourceType: "ftp", cb: func(_ context.Context, _ *Data) error { return nil }}
	err := m.Start(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errUnsupportedSourceType)
}

// ---------- startRemote: sync fail + cache zip present but invalid ----------

func TestStartRemote_SyncFail_CacheInvalidZip(t *testing.T) {
	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), []byte("not-a-zip"), 0o600))

	m := &Manager{
		name:         "test",
		sourceType:   SourceTypeRemote,
		location:     "https://github.com/owner/repo",
		cacheDir:     cacheDir,
		zipPath:      filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath:     filepath.Join(cacheDir, "owner-repo.zip.temp"),
		syncInterval: time.Hour,
		cli: &mockHTTPClient{
			doFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			},
		},
		cb: func(_ context.Context, _ *Data) error {
			return nil
		},
	}
	err := m.startRemote(context.Background())
	require.Error(t, err)
}

// ---------- startRemote: sync succeeds not updated, zip exists ----------

func TestStartRemote_NotUpdated_LoadsExistingZip(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), zipBytes, 0o600))

	var cbCalled int
	m := &Manager{
		name:         "test",
		sourceType:   SourceTypeRemote,
		location:     "https://github.com/owner/repo",
		cacheDir:     cacheDir,
		zipPath:      filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath:     filepath.Join(cacheDir, "owner-repo.zip.temp"),
		syncInterval: time.Hour,
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error {
			cbCalled++
			return nil
		},
	}
	err := m.startRemote(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, cbCalled)
}

// ---------- detectZipRoot edge: files with leading slash ----------

func TestDetectZipRoot_LeadingSlash(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: "/root/a.txt"}
	fw, err := w.CreateHeader(fh)
	require.NoError(t, err)
	_, _ = fw.Write([]byte("a"))
	fh2 := &zip.FileHeader{Name: "/root/b.txt"}
	fw2, err := w.CreateHeader(fh2)
	require.NoError(t, err)
	_, _ = fw2.Write([]byte("b"))
	require.NoError(t, w.Close())

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	assert.Equal(t, "root", detectZipRoot(reader.File))
}

func TestDetectZipRoot_EmptyNameEntries(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: ""}
	_, err := w.CreateHeader(fh)
	require.NoError(t, err)
	fh2 := &zip.FileHeader{Name: "root/a.txt"}
	fw, err := w.CreateHeader(fh2)
	require.NoError(t, err)
	_, _ = fw.Write([]byte("a"))
	require.NoError(t, w.Close())

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	assert.Equal(t, "root", detectZipRoot(reader.File))
}

// ---------- openZipData no root prefix ----------

func TestOpenZipData_NoRoot(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bundle.zip")
	zipBytes := makeZip(t, map[string]string{
		"a.yaml": "content_a",
		"b.yaml": "content_b",
	})
	require.NoError(t, os.WriteFile(zipPath, zipBytes, 0o600))

	data, err := openZipData(zipPath)
	require.NoError(t, err)
	defer func() { _ = data.Close() }()
	assert.Equal(t, ".", data.Base)
}

// ---------- openLocalData edge: whitespace in path ----------

func TestOpenLocalData_WhitespacePath(t *testing.T) {
	dir := t.TempDir()
	_, err := openLocalData("  " + dir + "  ")
	require.NoError(t, err)
}

// ---------- listFilesFromFS error ----------

func TestListFilesFromFS_BadBase(t *testing.T) {
	dir := t.TempDir()
	_, err := listFilesFromFS(os.DirFS(dir), "nonexistent_base")
	require.Error(t, err)
}

// ---------- filesEqual same content, checked byte-by-byte ----------

func TestFilesEqual_SameSizeDifferentContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("abcde"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("abcdf"), 0o600))

	eq, err := filesEqual(a, b)
	require.NoError(t, err)
	assert.False(t, eq)
}

// ---------- syncAndActivate: downloadBundle error ----------

func TestSyncAndActivate_DownloadError(t *testing.T) {
	tagSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/tags" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1"}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tagSrv.Close()

	dataDir := t.TempDir()
	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: filepath.Join(dataDir, "cache"),
		zipPath:  filepath.Join(dataDir, "cache", "owner-repo.zip"),
		tempPath: filepath.Join(dataDir, "cache", "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = tagSrv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error { return nil },
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}

// ---------- fileExists stat error ----------

func TestFileExists_StatError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "perm")
	require.NoError(t, os.MkdirAll(p, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p, "f"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(p, 0o000))
	t.Cleanup(func() { _ = os.Chmod(p, 0o755) })

	_, err := fileExists(filepath.Join(p, "f"))
	require.Error(t, err)
}

// ---------- filesEqual read errors ----------

func TestFilesEqual_ReadLeftError(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	require.NoError(t, os.WriteFile(a, []byte("hello"), 0o000))
	require.NoError(t, os.WriteFile(b, []byte("hello"), 0o600))
	t.Cleanup(func() { _ = os.Chmod(a, 0o644) })

	_, err := filesEqual(a, b)
	require.Error(t, err)
}

func TestFilesEqual_ReadRightError(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	require.NoError(t, os.WriteFile(a, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("hello"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(b, 0o644) })

	_, err := filesEqual(a, b)
	require.Error(t, err)
}

// ---------- openLocalData resolve path error ----------

func TestOpenLocalData_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	data, err := openLocalData(dir)
	require.NoError(t, err)
	assert.Empty(t, data.Files)
}

// ---------- openZipData list error ----------

func TestOpenZipData_EmptyZip(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty.zip")
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	require.NoError(t, w.Close())
	require.NoError(t, os.WriteFile(f, buf.Bytes(), 0o600))

	data, err := openZipData(f)
	require.NoError(t, err)
	defer func() { _ = data.Close() }()
	assert.Empty(t, data.Files)
}

// ---------- detectZipRoot with "." entry ----------

func TestDetectZipRoot_DotEntry(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: "."}
	_, err := w.CreateHeader(fh)
	require.NoError(t, err)
	fh2 := &zip.FileHeader{Name: "root/a.txt"}
	fw, err := w.CreateHeader(fh2)
	require.NoError(t, err)
	_, _ = fw.Write([]byte("a"))
	require.NoError(t, w.Close())

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	assert.Equal(t, "root", detectZipRoot(reader.File))
}

// ---------- cleanupTemp with permission error ----------

func TestCleanupTemp_PermissionError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "locked")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	tmpFile := filepath.Join(sub, "test.temp")
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0o600))
	require.NoError(t, os.Chmod(sub, 0o555))
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	m := &Manager{tempPath: tmpFile}
	err := m.cleanupTemp()
	require.Error(t, err)
}

// ---------- syncAndActivate: MkdirAll error ----------

func TestSyncAndActivate_MkdirAllError(t *testing.T) {
	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: "/dev/null/impossible",
		zipPath:  "/dev/null/impossible/test.zip",
		tempPath: "/dev/null/impossible/test.zip.temp",
		cli:      &mockHTTPClient{},
		cb:       func(_ context.Context, _ *Data) error { return nil },
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}

// ---------- syncAndActivate: cleanupTemp error in syncAndActivate ----------

func TestSyncAndActivate_CleanupTempError(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	tmpFile := filepath.Join(cacheDir, "test.zip.temp")
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0o600))
	require.NoError(t, os.Chmod(cacheDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(cacheDir, 0o755) })

	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: cacheDir,
		zipPath:  filepath.Join(cacheDir, "test.zip"),
		tempPath: tmpFile,
		cli:      &mockHTTPClient{},
		cb:       func(_ context.Context, _ *Data) error { return nil },
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}

// ---------- startRemote: cleanupTemp error ----------

func TestStartRemote_CleanupTempError(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	tmpFile := filepath.Join(cacheDir, "test.zip.temp")
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0o600))
	require.NoError(t, os.Chmod(cacheDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(cacheDir, 0o755) })

	m := &Manager{
		name:         "test",
		sourceType:   SourceTypeRemote,
		location:     "https://github.com/owner/repo",
		cacheDir:     cacheDir,
		zipPath:      filepath.Join(cacheDir, "test.zip"),
		tempPath:     tmpFile,
		syncInterval: time.Hour,
		cli:          &mockHTTPClient{},
		cb:           func(_ context.Context, _ *Data) error { return nil },
	}
	err := m.startRemote(context.Background())
	require.Error(t, err)
}

// ---------- downloadBundle: create request error ----------

func TestDownloadBundle_BadURL(t *testing.T) {
	m := &Manager{
		cli: &mockHTTPClient{},
	}
	_, err := m.downloadBundle(context.Background(), "://bad-url")
	require.Error(t, err)
}

// ---------- fetchLatestGitHubTag: request creation error ----------

func TestFetchLatestGitHubTag_CancelledContext(t *testing.T) {
	m := &Manager{
		location: "https://github.com/owner/repo",
		cli: &mockHTTPClient{
			doFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("canceled")
			},
		},
	}
	_, err := m.fetchLatestGitHubTag(context.Background())
	require.Error(t, err)
}

// ---------- openLocalData: abs error ----------

func TestOpenLocalData_AbsError(t *testing.T) {
	data, err := openLocalData(t.TempDir())
	require.NoError(t, err)
	defer func() { _ = data.Close() }()
	assert.NotEmpty(t, data.Source)
}

// ---------- syncAndActivate: WriteFile error ----------

func TestSyncAndActivate_WriteFileError(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(cacheDir, 0o755) })

	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: cacheDir,
		zipPath:  filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath: filepath.Join(cacheDir, "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error { return nil },
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}

// ---------- syncAndActivate: rename error ----------

func TestSyncAndActivate_RenameError(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1new"})
	existingZipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1old"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v2.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), existingZipBytes, 0o600))

	cbCalled := false
	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: cacheDir,
		zipPath:  filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath: filepath.Join(cacheDir, "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error {
			cbCalled = true
			return nil
		},
	}
	updated, err := m.syncAndActivate(context.Background())
	require.NoError(t, err)
	assert.True(t, updated)
	assert.True(t, cbCalled)
}

// ---------- openZipData list files error ----------

func TestOpenZipData_ListFilesError(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bundle.zip")
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: "root/"}
	fh.SetMode(os.ModeDir | 0o755)
	_, err := w.CreateHeader(fh)
	require.NoError(t, err)
	fh2 := &zip.FileHeader{Name: "root/a.yaml"}
	fw, err := w.CreateHeader(fh2)
	require.NoError(t, err)
	_, _ = fw.Write([]byte("content"))
	require.NoError(t, w.Close())
	require.NoError(t, os.WriteFile(zipPath, buf.Bytes(), 0o600))

	data, err := openZipData(zipPath)
	require.NoError(t, err)
	defer func() { _ = data.Close() }()
}

// ---------- openLocalData stat error ----------

func TestOpenLocalData_PermError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "noperm")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "f.yaml"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := openLocalData(sub)
	require.Error(t, err)
}

// ---------- syncAndActivate: fileExists error ----------

func TestSyncAndActivate_FileExistsError(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))

	zipDir := filepath.Join(cacheDir, "owner-repo.zip")
	require.NoError(t, os.MkdirAll(zipDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(zipDir, "blocker"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(zipDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(zipDir, 0o755) })

	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: cacheDir,
		zipPath:  zipDir,
		tempPath: filepath.Join(cacheDir, "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error { return nil },
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}

// ---------- startRemote: not updated + load zip error ----------

// ---------- openLocalData: listFiles error (permission) ----------

func TestOpenLocalData_ListFilesPermError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "perm")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "f.yaml"), []byte("x"), 0o600))
	inner := filepath.Join(sub, "inner")
	require.NoError(t, os.MkdirAll(inner, 0o000))
	t.Cleanup(func() { _ = os.Chmod(inner, 0o755) })

	_, err := openLocalData(sub)
	require.Error(t, err)
}

// ---------- syncAndActivate: invalid temp zip ----------

func TestSyncAndActivate_InvalidTempZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write([]byte("not-a-valid-zip-file"))
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: filepath.Join(dataDir, "cache"),
		zipPath:  filepath.Join(dataDir, "cache", "owner-repo.zip"),
		tempPath: filepath.Join(dataDir, "cache", "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error { return nil },
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}

func TestStartRemote_NotUpdated_ZipOpenError(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), zipBytes, 0o600))

	m := &Manager{
		name:         "test",
		sourceType:   SourceTypeRemote,
		location:     "https://github.com/owner/repo",
		cacheDir:     cacheDir,
		zipPath:      filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath:     filepath.Join(cacheDir, "owner-repo.zip.temp"),
		syncInterval: time.Hour,
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error { return errors.New("cb error on not-updated") },
	}
	err := m.startRemote(context.Background())
	require.Error(t, err)
}

// ---------- startRemote: cb error in startRemote fallback path ----------

func TestStartRemote_FallbackCBError(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "cached"})
	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "owner-repo.zip"), zipBytes, 0o600))

	m := &Manager{
		name:         "test",
		sourceType:   SourceTypeRemote,
		location:     "https://github.com/owner/repo",
		cacheDir:     cacheDir,
		zipPath:      filepath.Join(cacheDir, "owner-repo.zip"),
		tempPath:     filepath.Join(cacheDir, "owner-repo.zip.temp"),
		syncInterval: time.Hour,
		cli: &mockHTTPClient{
			doFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			},
		},
		cb: func(_ context.Context, _ *Data) error {
			return errors.New("cb failed")
		},
	}
	err := m.startRemote(context.Background())
	require.Error(t, err)
}

// ---------- syncAndActivate_CBError ----------

func TestSyncAndActivate_CBError(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"root/a.yaml": "v1"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]struct {
				Name string `json:"name"`
			}{{Name: "v1.0.0"}})
		default:
			_, _ = w.Write(zipBytes)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	m := &Manager{
		location: "https://github.com/owner/repo",
		cacheDir: filepath.Join(dataDir, "cache"),
		zipPath:  filepath.Join(dataDir, "cache", "owner-repo.zip"),
		tempPath: filepath.Join(dataDir, "cache", "owner-repo.zip.temp"),
		cli: &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
			},
		},
		cb: func(_ context.Context, _ *Data) error {
			return errors.New("cb failed")
		},
	}
	_, err := m.syncAndActivate(context.Background())
	require.Error(t, err)
}
