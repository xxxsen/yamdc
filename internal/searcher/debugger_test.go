package searcher

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/store"
)

type debuggerTestClient struct{}

func (debuggerTestClient) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected http request: %s", req.URL.String())
}

type precheckFalsePlugin struct {
	api.DefaultPlugin
}

func (p *precheckFalsePlugin) OnPrecheckRequest(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type mockCleaner struct {
	res *movieidcleaner.Result
	err error
}

func (m *mockCleaner) Clean(_ string) (*movieidcleaner.Result, error) { return m.res, m.err }
func (m *mockCleaner) Explain(_ string) (*movieidcleaner.ExplainResult, error) {
	return nil, nil
}

func TestDebuggerUsesSnapshotCreators(t *testing.T) {
	oldCtx := factory.NewRegisterContext()
	oldCtx.Register("old", func(_ interface{}) (api.IPlugin, error) {
		return &precheckFalsePlugin{}, nil
	})
	factory.Swap(oldCtx)

	debugger := NewDebugger(debuggerTestClient{}, store.NewMemStorage(), nil, []string{"old"}, nil)

	newCtx := factory.NewRegisterContext()
	newCtx.Register("new", func(_ interface{}) (api.IPlugin, error) {
		return &precheckFalsePlugin{}, nil
	})
	factory.Swap(newCtx)

	plugins := debugger.Plugins()
	require.Equal(t, []string{"old"}, plugins.Available)

	result, err := debugger.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: false,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"old"}, result.UsedPlugins)
	require.Len(t, result.PluginResults, 1)
	require.Equal(t, "old", result.PluginResults[0].Plugin)
	require.False(t, result.PluginResults[0].Found)
}

func TestDebuggerPlugins(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("plgA", func(_ interface{}) (api.IPlugin, error) { return &precheckFalsePlugin{}, nil })
	ctx.Register("plgB", func(_ interface{}) (api.IPlugin, error) { return &precheckFalsePlugin{}, nil })
	d := &Debugger{
		cli:     &debuggerTestClient{},
		storage: store.NewMemStorage(),
	}
	d.SwapState([]string{"plgA", "plgB"}, map[string][]string{"CAT": {"plgA"}}, ctx.Snapshot())
	plugins := d.Plugins()
	assert.Contains(t, plugins.Available, "plgA")
	assert.Contains(t, plugins.Available, "plgB")
	assert.Equal(t, []string{"plgA", "plgB"}, plugins.Default)
}

func TestDebugSearch_EmptyInput(t *testing.T) {
	d := NewDebugger(&debuggerTestClient{}, store.NewMemStorage(), nil, nil, nil)
	_, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: ""})
	require.ErrorIs(t, err, errInputRequired)
}

func TestDebugSearch_PluginNotFound(t *testing.T) {
	d := NewDebugger(&debuggerTestClient{}, store.NewMemStorage(), nil, nil, nil)
	_, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123", Plugins: []string{"nonexistent"}})
	require.ErrorIs(t, err, errDebugPluginNotFound)
}

func TestDebugSearch_WithCleaner(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("test-plg", func(_ interface{}) (api.IPlugin, error) { return &precheckFalsePlugin{}, nil })
	cleaner := &mockCleaner{
		res: &movieidcleaner.Result{
			Normalized:      "ABC-123",
			CategoryMatched: true,
			Category:        "TEST",
		},
	}
	d := &Debugger{
		cli:     &debuggerTestClient{},
		storage: store.NewMemStorage(),
		cleaner: cleaner,
	}
	d.SwapState([]string{"test-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: true,
	})
	require.NoError(t, err)
	require.Equal(t, "TEST", result.Category)
}

func TestDebugSearch_CleanerError(t *testing.T) {
	cleaner := &mockCleaner{err: errors.New("cleaner failed")}
	d := &Debugger{
		cli:     &debuggerTestClient{},
		storage: store.NewMemStorage(),
		cleaner: cleaner,
	}
	d.SwapState(nil, nil, map[string]factory.CreatorFunc{})
	_, err := d.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: true,
	})
	require.Error(t, err)
}

func TestDebugSearch_CleanerNilResult(t *testing.T) {
	cleaner := &mockCleaner{res: nil}
	ctx := factory.NewRegisterContext()
	ctx.Register("test-plg", func(_ interface{}) (api.IPlugin, error) { return &precheckFalsePlugin{}, nil })
	d := &Debugger{
		cli:     &debuggerTestClient{},
		storage: store.NewMemStorage(),
		cleaner: cleaner,
	}
	d.SwapState([]string{"test-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: true,
	})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_WithUncensor(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("test-plg", func(_ interface{}) (api.IPlugin, error) { return &precheckFalsePlugin{}, nil })
	cleaner := &mockCleaner{
		res: &movieidcleaner.Result{
			Normalized:      "ABC-123",
			UncensorMatched: true,
			Uncensor:        true,
		},
	}
	d := &Debugger{
		cli:     &debuggerTestClient{},
		storage: store.NewMemStorage(),
		cleaner: cleaner,
	}
	d.SwapState([]string{"test-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: true,
	})
	require.NoError(t, err)
	require.True(t, result.Uncensor)
}

func TestDebugSearch_ResolvePluginsWithCategory(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("cat-plg", func(_ interface{}) (api.IPlugin, error) { return &precheckFalsePlugin{}, nil })
	cleaner := &mockCleaner{
		res: &movieidcleaner.Result{
			Normalized:      "ABC-123",
			CategoryMatched: true,
			Category:        "MYCAT",
		},
	}
	d := &Debugger{
		cli:     &debuggerTestClient{},
		storage: store.NewMemStorage(),
		cleaner: cleaner,
	}
	d.SwapState([]string{}, map[string][]string{"MYCAT": {"cat-plg"}}, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: true,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cat-plg"}, result.UsedPlugins)
}

func TestDebugSearch_SkipAssets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>ok</body></html>`))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("test-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: true,
			decodeOK:      true,
			decodeData: &model.MovieMeta{
				Number:      "ABC-123",
				Title:       "T",
				Cover:       &model.File{Name: srv.URL + "/cover.jpg"},
				ReleaseDate: 1,
			},
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{
		cli:     srv.Client(),
		storage: store.NewMemStorage(),
	}
	d.SwapState([]string{"test-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		SkipAssets: true,
	})
	require.NoError(t, err)
	require.True(t, result.Found)
}

// --- helper function tests ---

func TestNormalizePluginList(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{name: "empty", input: nil, expected: nil},
		{name: "dedup", input: []string{"a", "b", "a"}, expected: []string{"a", "b"}},
		{name: "comma_separated", input: []string{"a,b, c"}, expected: []string{"a", "b", "c"}},
		{name: "blank_items", input: []string{" , ,a"}, expected: []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePluginList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloneStringMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string][]string
		expectNE bool
	}{
		{name: "nil", input: nil, expectNE: false},
		{name: "empty", input: map[string][]string{}, expectNE: false},
		{name: "with_data", input: map[string][]string{"k": {"v1", "v2"}}, expectNE: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cloneStringMap(tt.input)
			require.NotNil(t, result)
			if tt.expectNE {
				require.Len(t, result, len(tt.input))
			}
		})
	}
}

func TestRequestURL(t *testing.T) {
	assert.Equal(t, "", requestURL(nil))
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/path", nil)
	assert.Equal(t, "https://example.com/path", requestURL(req))
}

func TestHasFileKey(t *testing.T) {
	assert.False(t, hasFileKey(nil))
	assert.False(t, hasFileKey(&model.File{Key: ""}))
	assert.False(t, hasFileKey(&model.File{Key: "   "}))
	assert.True(t, hasFileKey(&model.File{Key: "abc"}))
}

func TestCountSampleKeys(t *testing.T) {
	assert.Equal(t, 0, countSampleKeys(nil))
	assert.Equal(t, 1, countSampleKeys([]*model.File{{Key: "a"}, {Key: ""}}))
}

func TestMetaHasAssets(t *testing.T) {
	assert.False(t, metaHasAssets(nil))
	assert.False(t, metaHasAssets(&model.MovieMeta{}))
	assert.True(t, metaHasAssets(&model.MovieMeta{Cover: &model.File{Key: "k"}}))
	assert.True(t, metaHasAssets(&model.MovieMeta{Poster: &model.File{Key: "k"}}))
	assert.True(t, metaHasAssets(&model.MovieMeta{SampleImages: []*model.File{{Key: "k"}}}))
}

func TestBoolMessage(t *testing.T) {
	assert.Equal(t, "yes", boolMessage(true, "yes", "no"))
	assert.Equal(t, "no", boolMessage(false, "yes", "no"))
}

func TestCollectVisiblePlugins(t *testing.T) {
	creators := map[string]factory.CreatorFunc{
		"a": func(_ interface{}) (api.IPlugin, error) { return nil, nil },
		"b": func(_ interface{}) (api.IPlugin, error) { return nil, nil },
	}
	result := collectVisiblePlugins(
		[]string{"a", "b", "  ", "a", "nonexistent"},
		map[string][]string{"CAT": {"b", "a", "missing"}},
		creators,
	)
	assert.Equal(t, []string{"a", "b"}, result)
}

// --- debugger extended coverage ---

func TestDebugSearch_FullFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cover.jpg" {
			_, _ = w.Write([]byte("image"))
			return
		}
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("found-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: true,
			decodeOK:      true,
			decodeData: &model.MovieMeta{
				Number:      "ABC-123",
				Title:       "T",
				Cover:       &model.File{Name: srv.URL + "/cover.jpg"},
				ReleaseDate: 1,
			},
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{cli: srv.Client(), storage: store.NewMemStorage()}
	d.SwapState([]string{"found-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123", SkipAssets: false})
	require.NoError(t, err)
	require.True(t, result.Found)
	require.Equal(t, "found-plg", result.MatchedPlugin)
}

func TestDebugSearch_PrecheckError(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("err-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{precheckOK: false, precheckErr: errors.New("precheck fail")}, nil
	})
	d := &Debugger{cli: &debuggerTestClient{}, storage: store.NewMemStorage()}
	d.SwapState([]string{"err-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
	require.NotEmpty(t, result.PluginResults[0].Error)
}

func TestDebugSearch_MakeRequestError(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("req-err-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK: true,
			makeReqFn: func(_ context.Context, _ string) (*http.Request, error) {
				return nil, errors.New("make req error")
			},
		}, nil
	})
	d := &Debugger{cli: &debuggerTestClient{}, storage: store.NewMemStorage()}
	d.SwapState([]string{"req-err-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_HandleHTTPReturnsNilResponse(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("nil-rsp-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK: true,
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, "http://example.com/", nil)
			},
			handleHTTPReqFn: func(_ context.Context, _ api.HTTPInvoker, _ *http.Request) (*http.Response, error) {
				return nil, nil
			},
		}, nil
	})
	d := &Debugger{cli: &debuggerTestClient{}, storage: store.NewMemStorage()}
	d.SwapState([]string{"nil-rsp-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_HTTPError(t *testing.T) {
	ctx := factory.NewRegisterContext()
	ctx.Register("http-err-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK: true,
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, "http://example.com/", nil)
			},
			handleHTTPReqFn: func(_ context.Context, _ api.HTTPInvoker, _ *http.Request) (*http.Response, error) {
				return nil, errors.New("http error")
			},
		}, nil
	})
	d := &Debugger{cli: &debuggerTestClient{}, storage: store.NewMemStorage()}
	d.SwapState([]string{"http-err-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_PrecheckResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("rsp-err-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: false,
			precheckRspErr: errors.New("precheck rsp error"),
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{cli: srv.Client(), storage: store.NewMemStorage()}
	d.SwapState([]string{"rsp-err-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_Non200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("500-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: true,
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{cli: srv.Client(), storage: store.NewMemStorage()}
	d.SwapState([]string{"500-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("dec-err-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: true,
			decodeErr:     errors.New("decode error"),
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{cli: srv.Client(), storage: store.NewMemStorage()}
	d.SwapState([]string{"dec-err-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_DecodeNotSucc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("no-dec-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: true,
			decodeOK:      false,
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{cli: srv.Client(), storage: store.NewMemStorage()}
	d.SwapState([]string{"no-dec-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123"})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestDebugSearch_VerifyMetaFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	ctx := factory.NewRegisterContext()
	ctx.Register("verify-fail-plg", func(_ interface{}) (api.IPlugin, error) {
		return &fullPlugin{
			precheckOK:    true,
			precheckRspOK: true,
			decodeOK:      true,
			decodeData:    &model.MovieMeta{Number: "ABC-123", Title: "T"},
			makeReqFn: func(ctx2 context.Context, _ string) (*http.Request, error) {
				return http.NewRequestWithContext(ctx2, http.MethodGet, srv.URL, nil)
			},
		}, nil
	})
	d := &Debugger{cli: srv.Client(), storage: store.NewMemStorage()}
	d.SwapState([]string{"verify-fail-plg"}, nil, ctx.Snapshot())
	result, err := d.DebugSearch(context.Background(), DebugSearchOptions{Input: "ABC-123", SkipAssets: true})
	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestCloneCreators(t *testing.T) {
	in := map[string]factory.CreatorFunc{
		"a": func(_ interface{}) (api.IPlugin, error) { return nil, nil },
	}
	out := cloneCreators(in)
	require.Len(t, out, 1)
}

func TestDebugOnePlugin_CreatorError(t *testing.T) {
	d := &Debugger{cli: &debuggerTestClient{}, storage: store.NewMemStorage()}
	d.SwapState(nil, nil, map[string]factory.CreatorFunc{
		"bad": func(_ interface{}) (api.IPlugin, error) {
			return nil, errors.New("creator error")
		},
	})
	_, err := d.debugOnePlugin(context.Background(), "bad", mustParseNumber(t, "ABC-123"), false)
	require.Error(t, err)
}

func TestResolveNumber_ParseError(t *testing.T) {
	d := &Debugger{}
	_, err := d.resolveNumber("", false, &DebugSearchResult{})
	require.Error(t, err)
}

func TestTryCleanInput_EmptyNormalized(t *testing.T) {
	cleaner := &mockCleaner{res: &movieidcleaner.Result{Normalized: "  "}}
	d := &Debugger{cleaner: cleaner}
	num, err := d.tryCleanInput("test", &DebugSearchResult{})
	require.NoError(t, err)
	require.Nil(t, num)
}

func TestResolvePlugins_NilNumber(t *testing.T) {
	d := &Debugger{
		defaultPlugins:  []string{"a"},
		categoryPlugins: map[string][]string{},
	}
	result := d.resolvePlugins(nil)
	require.Equal(t, []string{"a"}, result)
}
