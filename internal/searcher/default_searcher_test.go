package searcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/store"
)

func newOKResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Header:     make(http.Header),
	}
}

func buildSearcher(t *testing.T, plg api.IPlugin, cli *http.Client) ISearcher {
	t.Helper()
	httpCli := &mockHTTPClient{}
	if cli != nil {
		httpCli = &mockHTTPClient{do: cli.Do}
	}
	s, err := NewDefaultSearcher("test", plg,
		WithHTTPClient(httpCli), WithStorage(store.NewMemStorage()))
	require.NoError(t, err)
	return s
}

type failPutStorage struct {
	store.IStorage
}

func (s *failPutStorage) GetData(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("not found")
}

func (s *failPutStorage) PutData(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return errors.New("put failed")
}

func (s *failPutStorage) IsDataExist(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// --- NewDefaultSearcher tests ---

func TestNewDefaultSearcher(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr error
	}{
		{
			name:    "nil_client",
			opts:    []Option{WithStorage(store.NewMemStorage())},
			wantErr: errHTTPClientNil,
		},
		{
			name:    "nil_storage",
			opts:    []Option{WithHTTPClient(&mockHTTPClient{})},
			wantErr: errStorageNil,
		},
		{
			name: "success",
			opts: []Option{WithHTTPClient(&mockHTTPClient{}), WithStorage(store.NewMemStorage())},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewDefaultSearcher("test", &api.DefaultPlugin{}, tt.opts...)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, s)
			} else {
				require.NoError(t, err)
				require.NotNil(t, s)
			}
		})
	}
}

func TestMustNewDefaultSearcher_Panics(t *testing.T) {
	require.Panics(t, func() {
		MustNewDefaultSearcher("test", &api.DefaultPlugin{})
	})
}

// --- DefaultSearcher.Name ---

func TestDefaultSearcherName(t *testing.T) {
	s, err := NewDefaultSearcher("test-name", &api.DefaultPlugin{},
		WithHTTPClient(&mockHTTPClient{}), WithStorage(store.NewMemStorage()))
	require.NoError(t, err)
	assert.Equal(t, "test-name", s.Name())
}

// --- DefaultSearcher.Check ---

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		doFn    func(req *http.Request) (*http.Response, error)
		wantErr bool
	}{
		{
			name:  "no_hosts",
			hosts: nil,
		},
		{
			name:  "success",
			hosts: []string{"will-be-replaced"},
			doFn: func(_ *http.Request) (*http.Response, error) {
				return newOKResponse("ok"), nil
			},
		},
		{
			name:  "non_200",
			hosts: []string{"will-be-replaced"},
			doFn: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Status:     "503 Service Unavailable",
					Body:       io.NopCloser(bytes.NewReader(nil)),
					Header:     make(http.Header),
				}, nil
			},
			wantErr: true,
		},
		{
			name:  "network_error",
			hosts: []string{"will-be-replaced"},
			doFn: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("net error")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()
			hosts := tt.hosts
			cli := &mockHTTPClient{do: tt.doFn}
			if len(hosts) > 0 {
				hosts = []string{srv.URL}
				if tt.doFn == nil {
					cli = &mockHTTPClient{do: func(req *http.Request) (*http.Response, error) {
						return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
					}}
				}
			}
			plg := &fullPlugin{hosts: hosts, precheckOK: true}
			s, err := NewDefaultSearcher("check-test", plg,
				WithHTTPClient(cli), WithStorage(store.NewMemStorage()))
			require.NoError(t, err)
			err = s.Check(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- Search (full flow) ---

func TestSearch_PrecheckFails(t *testing.T) {
	plg := &fullPlugin{precheckOK: false, precheckErr: errors.New("precheck error")}
	s := buildSearcher(t, plg, nil)
	num := mustParseNumber(t, "ABC-123")
	_, _, err := s.Search(context.Background(), num)
	require.Error(t, err)
}

func TestSearch_PrecheckNotOK(t *testing.T) {
	plg := &fullPlugin{precheckOK: false}
	s := buildSearcher(t, plg, nil)
	num := mustParseNumber(t, "ABC-123")
	_, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestSearch_MakeRequestFails(t *testing.T) {
	plg := &fullPlugin{
		precheckOK: true,
		makeReqFn: func(_ context.Context, _ string) (*http.Request, error) {
			return nil, errors.New("make req error")
		},
	}
	s := buildSearcher(t, plg, nil)
	num := mustParseNumber(t, "ABC-123")
	_, _, err := s.Search(context.Background(), num)
	require.Error(t, err)
}

func TestSearch_DecodeNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      false,
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestSearch_VerifyMetaFailsCover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number: "ABC-123",
			Title:  "Title",
			Cover:  nil,
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestSearch_VerifyMetaFailsNoNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number:      "",
			Title:       "Title",
			Cover:       &model.File{Name: "cover.jpg"},
			ReleaseDate: 123456,
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestSearch_VerifyMetaFailsNoTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number:      "ABC-123",
			Title:       "",
			Cover:       &model.File{Name: "cover.jpg"},
			ReleaseDate: 123456,
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestSearch_VerifyMetaFailsNoReleaseDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number:      "ABC-123",
			Title:       "Title",
			Cover:       &model.File{Name: "cover.jpg"},
			ReleaseDate: 0,
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestSearch_SuccessWithDisableReleaseDateCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cover.jpg" {
			_, _ = w.Write([]byte("image-data"))
			return
		}
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number:      "ABC-123",
			Title:       "Title",
			Cover:       &model.File{Name: srv.URL + "/cover.jpg"},
			ReleaseDate: 0,
			SwithConfig: model.SwitchConfig{DisableReleaseDateCheck: true},
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	meta, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, meta)
	require.Equal(t, "test", meta.ExtInfo.ScrapeInfo.Source)
}

func TestSearch_FullSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cover.jpg", "/poster.jpg", "/sample1.jpg":
			_, _ = w.Write([]byte("image-data"))
		default:
			_, _ = w.Write([]byte("body"))
		}
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number:       "ABC-123",
			Title:        "Title",
			Cover:        &model.File{Name: srv.URL + "/cover.jpg"},
			Poster:       &model.File{Name: srv.URL + "/poster.jpg"},
			SampleImages: []*model.File{{Name: srv.URL + "/sample1.jpg"}},
			ReleaseDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	meta, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, meta)
	require.Equal(t, "ABC-123", meta.Number)
	require.NotEmpty(t, meta.Cover.Key)
	require.NotEmpty(t, meta.Poster.Key)
}

func TestSearch_WithCacheEnabled(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cover.jpg" {
			_, _ = w.Write([]byte("image"))
			return
		}
		callCount++
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeOK:      true,
		decodeData: &model.MovieMeta{
			Number:      "ABC-123",
			Title:       "Title",
			Cover:       &model.File{Name: srv.URL + "/cover.jpg"},
			ReleaseDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		},
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	storage := store.NewMemStorage()
	s, err := NewDefaultSearcher("test", plg,
		WithHTTPClient(srv.Client()), WithStorage(storage), WithSearchCache(true))
	require.NoError(t, err)

	num := mustParseNumber(t, "ABC-123")
	meta, found, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, meta)
	require.Equal(t, 1, callCount)

	meta2, found2, err2 := s.Search(context.Background(), num)
	require.NoError(t, err2)
	require.True(t, found2)
	require.NotNil(t, meta2)
	require.Equal(t, 1, callCount)
}

func TestSearch_PrecheckRspNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: false,
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, _, err := s.Search(context.Background(), num)
	require.Error(t, err)
}

func TestSearch_HTTPNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, _, err := s.Search(context.Background(), num)
	require.Error(t, err)
}

func TestSearch_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:    true,
		precheckRspOK: true,
		decodeErr:     errors.New("decode error"),
		makeReqFn: func(ctx context.Context, _ string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		},
	}
	s := buildSearcher(t, plg, srv.Client())
	num := mustParseNumber(t, "ABC-123")
	_, _, err := s.Search(context.Background(), num)
	require.Error(t, err)
}

// --- fixMeta / fixSingleURL ---

func TestFixSingleURL(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "protocol_relative", input: "//cdn.example.com/img.jpg", expect: "https://cdn.example.com/img.jpg"},
		{name: "root_relative", input: "/img/cover.jpg", expect: "https://example.com/img/cover.jpg"},
		{name: "absolute", input: "https://other.com/img.jpg", expect: "https://other.com/img.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/search", nil)
			ds := &DefaultSearcher{}
			val := tt.input
			ds.fixSingleURL(req, &val, "https://example.com")
			assert.Equal(t, tt.expect, val)
		})
	}
}

func TestFixMeta(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/search", nil)
	ds := &DefaultSearcher{}
	meta := &model.MovieMeta{
		Cover:        &model.File{Name: "//cdn.example.com/cover.jpg"},
		Poster:       &model.File{Name: "/poster.jpg"},
		SampleImages: []*model.File{{Name: "//cdn.example.com/s1.jpg"}},
	}
	ds.fixMeta(context.Background(), req, meta)
	assert.Equal(t, "https://cdn.example.com/cover.jpg", meta.Cover.Name)
	assert.Equal(t, "https://example.com/poster.jpg", meta.Poster.Name)
	assert.Equal(t, "https://cdn.example.com/s1.jpg", meta.SampleImages[0].Name)
}

func TestFixMeta_NilCoverPoster(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/search", nil)
	ds := &DefaultSearcher{}
	meta := &model.MovieMeta{Cover: nil, Poster: nil}
	ds.fixMeta(context.Background(), req, meta)
	assert.Nil(t, meta.Cover)
	assert.Nil(t, meta.Poster)
}

// --- storeImageData edge cases ---

func TestStoreImageData_FailedDownload(t *testing.T) {
	cli := &mockHTTPClient{do: func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("download failed")
	}}
	ds := &DefaultSearcher{
		cc:  &config{cli: cli, storage: store.NewMemStorage()},
		plg: &fullPlugin{},
	}
	meta := &model.MovieMeta{
		Cover:  &model.File{Name: "http://example.com/cover.jpg"},
		Poster: &model.File{Name: "http://example.com/poster.jpg"},
	}
	ds.storeImageData(context.Background(), meta)
	assert.Nil(t, meta.Cover)
	assert.Nil(t, meta.Poster)
}

func TestStoreImageData_EmptyURL(_ *testing.T) {
	ds := &DefaultSearcher{
		cc: &config{cli: &mockHTTPClient{}, storage: store.NewMemStorage()},
	}
	meta := &model.MovieMeta{
		Cover:  &model.File{Name: ""},
		Poster: nil,
	}
	ds.storeImageData(context.Background(), meta)
}

func TestFetchImageData_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	plg := &fullPlugin{}
	ds := &DefaultSearcher{
		cc:  &config{cli: srv.Client(), storage: store.NewMemStorage()},
		plg: plg,
	}
	_, err := ds.fetchImageData(context.Background(), srv.URL+"/image.jpg")
	require.Error(t, err)
}

func TestFetchImageData_DecorateImageError(t *testing.T) {
	plg := &fullPlugin{decorateMediaErr: errors.New("media error")}
	ds := &DefaultSearcher{plg: plg, cc: &config{cli: &mockHTTPClient{}, storage: store.NewMemStorage()}}
	_, err := ds.fetchImageData(context.Background(), "http://example.com/img.jpg")
	require.Error(t, err)
}

// --- loadCacheData ---

func TestLoadCacheData_HitsCache(t *testing.T) {
	s := store.NewMemStorage()
	_ = s.PutData(context.Background(), "key1", []byte("cached"), 0)
	ds := &DefaultSearcher{cc: &config{storage: s}}
	data, err := ds.loadCacheData(context.Background(), "key1", time.Hour, func() ([]byte, error) {
		return nil, errors.New("should not be called")
	})
	require.NoError(t, err)
	assert.Equal(t, "cached", string(data))
}

func TestLoadCacheData_LoaderFails(t *testing.T) {
	ds := &DefaultSearcher{cc: &config{storage: store.NewMemStorage()}}
	_, err := ds.loadCacheData(context.Background(), "missing", time.Hour, func() ([]byte, error) {
		return nil, errors.New("loader error")
	})
	require.Error(t, err)
}

func TestLoadCacheData_LoaderSuccess(t *testing.T) {
	ds := &DefaultSearcher{cc: &config{storage: store.NewMemStorage()}}
	data, err := ds.loadCacheData(context.Background(), "k2", time.Hour, func() ([]byte, error) {
		return []byte("fresh"), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(data))
}

// --- decorateRequest / decorateImageRequest ---

func TestDecorateRequest_PluginError(t *testing.T) {
	plg := &fullPlugin{decorateReqErr: errors.New("decorate error")}
	ds := &DefaultSearcher{plg: plg}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	err := ds.decorateRequest(context.Background(), req)
	require.Error(t, err)
}

func TestDecorateImageRequest_PluginError(t *testing.T) {
	plg := &fullPlugin{decorateMediaErr: errors.New("media decorate error")}
	ds := &DefaultSearcher{plg: plg}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	err := ds.decorateImageRequest(context.Background(), req)
	require.Error(t, err)
}

func TestSetDefaultHTTPOptions_SetsReferer(t *testing.T) {
	ds := &DefaultSearcher{}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/search?q=1", nil)
	err := ds.setDefaultHTTPOptions(req)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/", req.Header.Get("Referer"))
}

func TestSetDefaultHTTPOptions_DoesNotOverrideReferer(t *testing.T) {
	ds := &DefaultSearcher{}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/search", nil)
	req.Header.Set("Referer", "https://other.com/")
	err := ds.setDefaultHTTPOptions(req)
	require.NoError(t, err)
	assert.Equal(t, "https://other.com/", req.Header.Get("Referer"))
}

// --- verifyMeta ---

func TestVerifyMeta(t *testing.T) {
	ds := &DefaultSearcher{}
	tests := []struct {
		name    string
		meta    *model.MovieMeta
		wantErr error
	}{
		{name: "valid", meta: &model.MovieMeta{
			Number: "A-1", Title: "T", Cover: &model.File{Name: "c.jpg"}, ReleaseDate: 1,
		}},
		{name: "no_cover", meta: &model.MovieMeta{
			Number: "A-1", Title: "T", Cover: nil,
		}, wantErr: errNoCover},
		{name: "empty_cover_name", meta: &model.MovieMeta{
			Number: "A-1", Title: "T", Cover: &model.File{Name: ""},
		}, wantErr: errNoCover},
		{name: "no_number", meta: &model.MovieMeta{
			Title: "T", Cover: &model.File{Name: "c.jpg"},
		}, wantErr: errNoNumber},
		{name: "no_title", meta: &model.MovieMeta{
			Number: "A-1", Cover: &model.File{Name: "c.jpg"},
		}, wantErr: errNoTitle},
		{name: "no_release_date", meta: &model.MovieMeta{
			Number: "A-1", Title: "T", Cover: &model.File{Name: "c.jpg"}, ReleaseDate: 0,
		}, wantErr: errNoReleaseDate},
		{name: "disable_release_date_check", meta: &model.MovieMeta{
			Number: "A-1", Title: "T", Cover: &model.File{Name: "c.jpg"}, ReleaseDate: 0,
			SwithConfig: model.SwitchConfig{DisableReleaseDateCheck: true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ds.verifyMeta(tt.meta)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- saveRemoteURLData ---

func TestSaveRemoteURLData_ExistingKey(t *testing.T) {
	storage := store.NewMemStorage()
	_ = storage.PutData(context.Background(), "da39a3ee5e6b4b0d3255bfef95601890afd80709", []byte("exists"), 0)
	ds := &DefaultSearcher{
		cc:  &config{cli: &mockHTTPClient{}, storage: storage},
		plg: &fullPlugin{},
	}
	result := ds.saveRemoteURLData(context.Background(), []string{""})
	assert.Empty(t, result)
}

func TestSaveRemoteURLData_StorePutFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()
	ds := &DefaultSearcher{
		plg: &fullPlugin{},
		cc:  &config{cli: srv.Client(), storage: &failPutStorage{}},
	}
	result := ds.saveRemoteURLData(context.Background(), []string{srv.URL + "/img.jpg"})
	assert.Contains(t, result, srv.URL+"/img.jpg")
}

// --- onRetriveData ---

func TestOnRetriveData_CacheUnmarshalFails(t *testing.T) {
	storage := store.NewMemStorage()
	_ = storage.PutData(context.Background(), "test:ABC-123:v1", []byte("not-json"), 0)
	plg := &fullPlugin{precheckOK: true, precheckRspOK: true}
	ds := &DefaultSearcher{
		name: "test",
		cc:   &config{cli: &mockHTTPClient{}, storage: storage, searchCache: true},
		plg:  plg,
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/search", nil)
	num := mustParseNumber(t, "ABC-123")
	ctx := api.InitContainer(context.Background())
	_, err := ds.onRetriveData(ctx, req, num)
	require.Error(t, err)
}

func TestOnRetriveData_NoCacheMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{precheckOK: true, precheckRspOK: true}
	ds := &DefaultSearcher{
		name: "test",
		cc:   &config{cli: srv.Client(), storage: store.NewMemStorage(), searchCache: false},
		plg:  plg,
	}
	ctx := api.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	num := mustParseNumber(t, "ABC-123")
	data, err := ds.onRetriveData(ctx, req, num)
	require.NoError(t, err)
	assert.Equal(t, "body", string(data))
}

func TestOnRetriveData_CacheMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{precheckOK: true, precheckRspOK: true}
	storage := store.NewMemStorage()
	ds := &DefaultSearcher{
		name: "test",
		cc:   &config{cli: srv.Client(), storage: storage, searchCache: true},
		plg:  plg,
	}
	ctx := api.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	num := mustParseNumber(t, "ABC-123")
	data, err := ds.onRetriveData(ctx, req, num)
	require.NoError(t, err)
	assert.Equal(t, "body", string(data))

	cached, cacheErr := storage.GetData(context.Background(), "test:ABC-123:v1")
	require.NoError(t, cacheErr)
	var cacheCtx searchCacheContext
	require.NoError(t, json.Unmarshal(cached, &cacheCtx))
	assert.Equal(t, "body", cacheCtx.SearchData)
}

func TestOnRetriveData_HandleHTTPError(t *testing.T) {
	plg := &fullPlugin{
		precheckOK: true,
		handleHTTPReqFn: func(_ context.Context, _ api.HTTPInvoker, _ *http.Request) (*http.Response, error) {
			return nil, errors.New("handle error")
		},
	}
	ds := &DefaultSearcher{
		name: "test",
		cc:   &config{cli: &mockHTTPClient{}, storage: store.NewMemStorage()},
		plg:  plg,
	}
	ctx := api.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/", nil)
	num := mustParseNumber(t, "ABC-123")
	_, err := ds.onRetriveData(ctx, req, num)
	require.Error(t, err)
}

func TestOnRetriveData_PrecheckRspError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	plg := &fullPlugin{
		precheckOK:     true,
		precheckRspErr: errors.New("precheck rsp error"),
	}
	ds := &DefaultSearcher{
		name: "test",
		cc:   &config{cli: srv.Client(), storage: store.NewMemStorage()},
		plg:  plg,
	}
	ctx := api.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	num := mustParseNumber(t, "ABC-123")
	_, err := ds.onRetriveData(ctx, req, num)
	require.Error(t, err)
}

func TestOnRetriveData_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	plg := &fullPlugin{precheckOK: true, precheckRspOK: true}
	ds := &DefaultSearcher{
		name: "test",
		cc:   &config{cli: srv.Client(), storage: store.NewMemStorage()},
		plg:  plg,
	}
	ctx := api.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	num := mustParseNumber(t, "ABC-123")
	_, err := ds.onRetriveData(ctx, req, num)
	require.Error(t, err)
}

// --- invokeHTTPRequest ---

func TestInvokeHTTPRequest_DecorateError(t *testing.T) {
	plg := &fullPlugin{decorateReqErr: errors.New("decorate error")}
	ds := &DefaultSearcher{
		plg: plg,
		cc:  &config{cli: &mockHTTPClient{}, storage: store.NewMemStorage()},
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	rsp, err := ds.invokeHTTPRequest(context.Background(), req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- checkOneRequest ---

func TestCheckOneRequest_DecorateError(t *testing.T) {
	plg := &fullPlugin{decorateReqErr: errors.New("decorate error")}
	ds := &DefaultSearcher{plg: plg, cc: &config{cli: &mockHTTPClient{}, storage: store.NewMemStorage()}}
	err := ds.checkOneRequest(context.Background(), "http://example.com")
	require.Error(t, err)
}
