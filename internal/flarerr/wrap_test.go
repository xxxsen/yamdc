package flarerr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientWrap_NoParams_DelegatesToImpl(t *testing.T) {
	var implCalled bool
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		implCalled = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("from-impl")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}}
	cli := NewHTTPClient(impl, "http://unused:9999")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/page", nil)
	resp, err := cli.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "from-impl", string(body))
	assert.True(t, implCalled)
}

func TestHTTPClientWrap_NoParams_ImplError(t *testing.T) {
	impl := &mockHTTPClient{doFn: func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	}}
	cli := NewHTTPClient(impl, "http://unused:9999")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/", nil)
	rsp, err := cli.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http request failed")
}

func TestHTTPClientWrap_WithParams_RoutesToSolver(t *testing.T) {
	wantHTML := "<html>solved</html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1" && r.Method == http.MethodPost {
			var fr flareRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&fr))
			assert.Equal(t, "request.get", fr.Cmd)
			assert.Equal(t, "https://target.test/item", fr.URL)
			resp := flareResponse{
				Status: "ok",
				Solution: flareSolution{
					Status:   http.StatusOK,
					Response: wantHTML,
					Cookies: []flareCookie{
						{Name: "cf_ck", Value: "xyz", Path: "/", Domain: "target.test"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer ts.Close()

	impl := &mockHTTPClient{doFn: func(_ *http.Request) (*http.Response, error) {
		t.Fatal("impl should not be called for solver requests")
		return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
	}}
	cli := NewHTTPClient(impl, ts.URL)

	ctx := WithParams(context.Background(), &Params{})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://target.test/item", nil)
	resp, err := cli.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, wantHTML, string(body))
}

func TestHTTPClientWrap_WithParams_SolverError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(flareResponse{
			Status:  "error",
			Message: "challenge failed",
		})
	}))
	defer ts.Close()

	cli := NewHTTPClient(&mockHTTPClient{}, ts.URL)
	ctx := WithParams(context.Background(), &Params{})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://fail.test/", nil)
	rsp, err := cli.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flaresolverr solve failed")
}

func TestHTTPClientWrap_WithParams_PostRejected(t *testing.T) {
	cli := NewHTTPClient(&mockHTTPClient{}, "http://unused:9999")
	ctx := WithParams(context.Background(), &Params{})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://fail.test/", nil)
	rsp, err := cli.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errFlareOnlyGET)
}

func TestHTTPClientWrap_CookieJar_PersistsAcrossCalls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1" {
			resp := flareResponse{
				Status: "ok",
				Solution: flareSolution{
					Status:   http.StatusOK,
					Response: "ok",
					Cookies: []flareCookie{
						{Name: "session", Value: "abc", Path: "/", Domain: "jar.test"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	var capturedReq *http.Request
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: 200, Body: io.NopCloser(nil), Request: req}, nil
	}}
	cli := NewHTTPClient(impl, ts.URL)

	u, _ := url.Parse("http://jar.test/page")

	ctx := WithParams(context.Background(), &Params{})
	req1 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req1 = req1.WithContext(ctx)
	resp1, err := cli.Do(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	req2 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req2 = req2.WithContext(context.Background())
	resp2, err := cli.Do(req2)
	require.NoError(t, err)
	if resp2.Body != nil {
		_ = resp2.Body.Close()
	}

	found := false
	for _, ck := range capturedReq.Cookies() {
		if ck.Name == "session" && ck.Value == "abc" {
			found = true
		}
	}
	assert.True(t, found, "solver cookies should be injected into subsequent non-solver requests")
}

func TestHTTPClientWrap_InjectCookies_NoDuplicates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := flareResponse{
			Status: "ok",
			Solution: flareSolution{
				Status:   http.StatusOK,
				Response: "ok",
				Cookies: []flareCookie{
					{Name: "dup_ck", Value: "solver_val", Path: "/", Domain: "dup.test"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	var capturedReq *http.Request
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: 200, Body: io.NopCloser(nil), Request: req}, nil
	}}
	cli := NewHTTPClient(impl, ts.URL)

	u, _ := url.Parse("http://dup.test/page")

	ctx := WithParams(context.Background(), &Params{})
	req1 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req1 = req1.WithContext(ctx)
	resp1, err := cli.Do(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	req2 := &http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}
	req2 = req2.WithContext(context.Background())
	req2.AddCookie(&http.Cookie{Name: "dup_ck", Value: "existing"})
	resp2, err := cli.Do(req2)
	require.NoError(t, err)
	if resp2.Body != nil {
		_ = resp2.Body.Close()
	}

	count := 0
	for _, ck := range capturedReq.Cookies() {
		if ck.Name == "dup_ck" {
			count++
		}
	}
	assert.Equal(t, 1, count, "should not duplicate existing cookies")
}

func TestHTTPClientWrap_InjectCookies_EmptyJar(t *testing.T) {
	var capturedReq *http.Request
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: 200, Body: io.NopCloser(nil), Request: req}, nil
	}}
	cli := NewHTTPClient(impl, "http://unused:9999")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://empty.test/", nil)
	resp, err := cli.Do(req)
	require.NoError(t, err)
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	assert.Empty(t, capturedReq.Cookies())
}
