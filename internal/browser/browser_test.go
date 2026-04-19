package browser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- mock helpers ----------

type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

type mockNavigator struct {
	navigateFunc func(ctx context.Context, url string, params *Params) (*NavigateResult, error)
	closeFunc    func() error
}

func (m *mockNavigator) Navigate(ctx context.Context, url string, params *Params) (*NavigateResult, error) {
	return m.navigateFunc(ctx, url, params)
}

func (m *mockNavigator) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// ---------- NewHTTPClient / httpClientWrap tests ----------

func TestNewHTTPClient_NoParams_DelegatesToImpl(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("from-impl"))
	}))
	defer srv.Close()

	impl := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req) //nolint:gosec // 测试场景, 无生产风险
		},
	}
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, _ *Params) (*NavigateResult, error) {
			t.Fatal("navigator should not be called without params")
			return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
		},
	}
	client := NewHTTPClient(impl, nav)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "from-impl", string(body))
}

func TestNewHTTPClient_NoParams_ImplError(t *testing.T) {
	impl := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		},
	}
	nav := &mockNavigator{}
	client := NewHTTPClient(impl, nav)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	rsp, err := client.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http request failed")
}

func TestNewHTTPClient_WithParams_DelegatesToNavigator(t *testing.T) {
	impl := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			t.Fatal("impl should not be called when params present")
			return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
		},
	}
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, rawURL string, params *Params) (*NavigateResult, error) {
			assert.Contains(t, rawURL, "http://")
			assert.Equal(t, "//div", params.WaitSelector)
			return &NavigateResult{
				HTML:    []byte("<html>browser-html</html>"),
				Cookies: []*http.Cookie{{Name: "sid", Value: "xyz"}},
			}, nil
		},
	}
	client := NewHTTPClient(impl, nav)

	params := &Params{
		WaitSelector: "//div",
		WaitTimeout:  5 * time.Second,
		Headers:      http.Header{"X-Custom": []string{"val"}},
	}
	ctx := WithParams(context.Background(), params)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/page", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "<html>browser-html</html>", string(body))
	assert.Equal(t, int64(len("<html>browser-html</html>")), resp.ContentLength)
}

func TestNewHTTPClient_WithParams_NavigatorError(t *testing.T) {
	impl := &mockHTTPClient{}
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, _ *Params) (*NavigateResult, error) {
			return nil, errors.New("browser crashed")
		},
	}
	client := NewHTTPClient(impl, nav)

	params := &Params{WaitSelector: "//div"}
	ctx := WithParams(context.Background(), params)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	rsp, err := client.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "browser navigate failed")
}

func TestNewHTTPClient_WithParams_NilHeaders(t *testing.T) {
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, p *Params) (*NavigateResult, error) {
			assert.Nil(t, p.Headers)
			return &NavigateResult{HTML: []byte("ok")}, nil
		},
	}
	client := NewHTTPClient(&mockHTTPClient{}, nav)

	params := &Params{WaitSelector: "//body", Headers: nil}
	ctx := WithParams(context.Background(), params)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
}

func TestNewHTTPClient_CookieJar_PersistsAcrossCalls(t *testing.T) {
	callCount := 0
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, _ *Params) (*NavigateResult, error) {
			callCount++
			return &NavigateResult{
				HTML:    []byte("ok"),
				Cookies: []*http.Cookie{{Name: "sid", Value: "abc", Path: "/"}},
			}, nil
		},
	}
	client := NewHTTPClient(&mockHTTPClient{}, nav)

	u, _ := url.Parse("http://example.com/page")
	params := &Params{WaitSelector: "//div"}
	ctx := WithParams(context.Background(), params)

	req1 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req1 = req1.WithContext(ctx)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	req2 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req2 = req2.WithContext(ctx)
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	_ = resp2.Body.Close()
	assert.Equal(t, 2, callCount)
}

func TestInjectCookies_NoDuplicates(t *testing.T) {
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, _ *Params) (*NavigateResult, error) {
			return &NavigateResult{
				HTML:    []byte("ok"),
				Cookies: []*http.Cookie{{Name: "browser_ck", Value: "bval", Path: "/"}},
			}, nil
		},
	}
	var capturedReq *http.Request
	impl := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(nil),
			}, nil
		},
	}
	client := NewHTTPClient(impl, nav)

	u, _ := url.Parse("http://example.com/page")

	params := &Params{WaitSelector: "//div"}
	ctx := WithParams(context.Background(), params)
	req1 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req1 = req1.WithContext(ctx)
	resp, err := client.Do(req1)
	require.NoError(t, err)
	_ = resp.Body.Close()

	req2 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req2 = req2.WithContext(context.Background())
	req2.AddCookie(&http.Cookie{Name: "browser_ck", Value: "existing"})
	rsp2, err := client.Do(req2)
	if rsp2 != nil && rsp2.Body != nil {
		defer func() { _ = rsp2.Body.Close() }()
	}
	require.NoError(t, err)

	cookies := capturedReq.Cookies()
	count := 0
	for _, c := range cookies {
		if c.Name == "browser_ck" {
			count++
		}
	}
	assert.Equal(t, 1, count, "should not duplicate existing cookies")
}

func TestNewHTTPClient_NoCookiesReturned(t *testing.T) {
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, _ *Params) (*NavigateResult, error) {
			return &NavigateResult{HTML: []byte("ok"), Cookies: nil}, nil
		},
	}
	client := NewHTTPClient(&mockHTTPClient{}, nav)

	params := &Params{WaitSelector: "//div"}
	ctx := WithParams(context.Background(), params)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
}

// ---------- injectCookies (httpClientWrap) edge cases ----------

func TestInjectCookies_EmptyJar(t *testing.T) {
	var capturedReq *http.Request
	impl := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{StatusCode: 200, Body: io.NopCloser(nil)}, nil
		},
	}
	client := NewHTTPClient(impl, &mockNavigator{})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Empty(t, capturedReq.Cookies())
}

func TestInjectCookies_WithExistingCookies(t *testing.T) {
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, _ *Params) (*NavigateResult, error) {
			return &NavigateResult{
				HTML:    []byte("ok"),
				Cookies: []*http.Cookie{{Name: "from_browser", Value: "bv", Path: "/"}},
			}, nil
		},
	}
	var capturedReq *http.Request
	impl := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{StatusCode: 200, Body: io.NopCloser(nil)}, nil
		},
	}
	client := NewHTTPClient(impl, nav)

	u, _ := url.Parse("http://example.com/page")

	ctx := WithParams(context.Background(), &Params{WaitSelector: "//div"})
	req1 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req1 = req1.WithContext(ctx)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	req2 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req2 = req2.WithContext(context.Background())
	rsp2, err := client.Do(req2)
	if rsp2 != nil && rsp2.Body != nil {
		defer func() { _ = rsp2.Body.Close() }()
	}
	require.NoError(t, err)

	found := false
	for _, c := range capturedReq.Cookies() {
		if c.Name == "from_browser" && c.Value == "bv" {
			found = true
		}
	}
	assert.True(t, found, "browser cookie should be injected into non-browser request")
}

// ---------- Params with cookies in non-browser mode ----------

func TestNewHTTPClient_WithParams_RequestCookiesIncluded(t *testing.T) {
	var capturedParams *Params
	nav := &mockNavigator{
		navigateFunc: func(_ context.Context, _ string, p *Params) (*NavigateResult, error) {
			capturedParams = p
			return &NavigateResult{HTML: []byte("ok")}, nil
		},
	}
	client := NewHTTPClient(&mockHTTPClient{}, nav)

	params := &Params{WaitSelector: "//div"}
	ctx := WithParams(context.Background(), params)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	req.AddCookie(&http.Cookie{Name: "req_ck", Value: "rv"})

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	found := false
	for _, c := range capturedParams.Cookies {
		if c.Name == "req_ck" && c.Value == "rv" {
			found = true
		}
	}
	assert.True(t, found, "request cookies should be forwarded to navigator")
}
