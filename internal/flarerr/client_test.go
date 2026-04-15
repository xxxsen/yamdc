package flarerr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func okEmptyResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

type mockHTTPClient struct {
	doFn func(*http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.doFn == nil {
		return nil, errors.New("doFn not set")
	}
	return m.doFn(req)
}

func TestNew_endpointPrefixes(t *testing.T) {
	t.Parallel()
	impl := &http.Client{}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "http_prefix", in: "http://127.0.0.1:9", want: "http://127.0.0.1:9"},
		{name: "https_prefix", in: "https://127.0.0.1:9", want: "https://127.0.0.1:9"},
		{name: "no_scheme", in: "127.0.0.1:9", want: "http://127.0.0.1:9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := New(impl, tt.in)
			require.NoError(t, err)
			sc, ok := c.(*solverClient)
			require.True(t, ok)
			assert.Equal(t, tt.want, sc.endpoint)
			assert.Equal(t, defaultByPassClientTimeout, sc.timeout)
			assert.NotNil(t, sc.byPastMap)
			assert.False(t, sc.tested)
		})
	}
}

func TestSolverClient_AddHost(t *testing.T) {
	t.Parallel()
	c, err := New(&http.Client{}, "http://localhost")
	require.NoError(t, err)
	sc := c.(*solverClient)

	t.Run("plain_host", func(t *testing.T) {
		require.NoError(t, sc.AddHost("cdn.example.test"))
		_, ok := sc.byPastMap["cdn.example.test"]
		assert.True(t, ok)
	})

	t.Run("http_url_strips_scheme", func(t *testing.T) {
		require.NoError(t, sc.AddHost("http://alpha.example.test:9090/path?q=1"))
		_, ok := sc.byPastMap["alpha.example.test:9090"]
		assert.True(t, ok)
	})

	t.Run("https_url_strips_scheme", func(t *testing.T) {
		require.NoError(t, sc.AddHost("https://beta.example.test/path"))
		_, ok := sc.byPastMap["beta.example.test"]
		assert.True(t, ok)
	})

	t.Run("invalid_url", func(t *testing.T) {
		err := sc.AddHost("http://%zz")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse host url")
	})
}

func TestMustAddToSolverList(t *testing.T) {
	t.Parallel()
	t.Run("normal", func(t *testing.T) {
		c, err := New(&http.Client{}, "http://localhost")
		require.NoError(t, err)
		assert.NotPanics(t, func() {
			MustAddToSolverList(c, "h1.example", "https://h2.example/x")
		})
		sc := c.(*solverClient)
		_, ok1 := sc.byPastMap["h1.example"]
		_, ok2 := sc.byPastMap["h2.example"]
		assert.True(t, ok1)
		assert.True(t, ok2)
	})

	t.Run("panic_on_add_host_error", func(t *testing.T) {
		bad := &stubSolverClient{addHostErr: errors.New("forced")}
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			MustAddToSolverList(bad, "any")
		}()
		require.NotNil(t, recovered)
		assert.Contains(t, fmt.Sprint(recovered), "add host:any")
	})
}

type stubSolverClient struct {
	addHostErr error
}

func (s *stubSolverClient) AddHost(string) error { return s.addHostErr }

func (s *stubSolverClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("stub: Do should not be called in this test")
}

func TestSolverClient_convertRequest(t *testing.T) {
	t.Parallel()
	b := &solverClient{timeout: 1500 * time.Millisecond}
	t.Run("get_ok", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://origin.test/resource", nil)
		require.NoError(t, err)
		fr, err := b.convertRequest(req)
		require.NoError(t, err)
		require.NotNil(t, fr)
		assert.Equal(t, "request.get", fr.Cmd)
		assert.Equal(t, "https://origin.test/resource", fr.URL)
		assert.Equal(t, 1500, fr.MaxTimeout)
	})

	t.Run("non_get_rejected", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://origin.test/", nil)
		require.NoError(t, err)
		fr, err := b.convertRequest(req)
		require.Error(t, err)
		assert.Nil(t, fr)
		assert.ErrorIs(t, err, errFlareOnlyGET)
		assert.Contains(t, err.Error(), http.MethodPost)
	})
}

func TestSolverClient_isNeedByPass(t *testing.T) {
	t.Parallel()
	b := &solverClient{
		byPastMap: map[string]struct{}{
			"bypass.test": {},
		},
	}
	t.Run("host_in_map", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://bypass.test/page", nil)
		require.NoError(t, err)
		assert.True(t, b.isNeedByPass(req))
	})
	t.Run("host_not_in_map", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://other.test/page", nil)
		require.NoError(t, err)
		assert.False(t, b.isNeedByPass(req))
	})
}

func TestSolverClient_testHost(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		b := &solverClient{}
		err := b.testHost(context.Background(), ts.Client(), ts.URL)
		require.NoError(t, err)
	})

	t.Run("do_error", func(t *testing.T) {
		b := &solverClient{}
		impl := &mockHTTPClient{doFn: func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}}
		err := b.testHost(context.Background(), impl, "http://unused")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execute test request")
	})
}

func newFlareSolverMux(t *testing.T, v1Handler http.HandlerFunc) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "only GET on /", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1", v1Handler)
	return mux
}

func TestSolverClient_Do_passthrough(t *testing.T) {
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "POST /v1 should not be called", http.StatusInternalServerError)
	}))
	defer ts.Close()

	var calls int
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			assert.Equal(t, ts.URL, req.URL.String())
			return okEmptyResponse(), nil
		}
		assert.Equal(t, "https://passthrough.test/doc", req.URL.String())
		return &http.Response{
			StatusCode: http.StatusTeapot,
			Body:       io.NopCloser(strings.NewReader("tea")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}}

	c, err := New(impl, ts.URL)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://passthrough.test/doc", nil)
	require.NoError(t, err)

	rsp, err := c.Do(req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, http.StatusTeapot, rsp.StatusCode)
	body, _ := io.ReadAll(rsp.Body)
	assert.Equal(t, "tea", string(body))
	assert.Equal(t, 2, calls)
	assert.True(t, c.(*solverClient).tested)
}

func TestSolverClient_Do_bypass_ok(t *testing.T) {
	wantBody := flareResponse{
		Status:  "ok",
		Message: "",
		Solution: flareSolution{
			Status:   http.StatusOK,
			Response: "<html>bypassed</html>",
		},
	}
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "want POST", http.StatusMethodNotAllowed)
			return
		}
		var got flareRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if got.Cmd != "request.get" || got.URL != "https://shielded.test/item" ||
			got.MaxTimeout != int(defaultByPassClientTimeout.Milliseconds()) {
			http.Error(w, "unexpected flare request payload", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wantBody)
	}))
	defer ts.Close()

	var calls int
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, ts.URL, req.URL.String())
		return okEmptyResponse(), nil
	}}

	c, err := New(impl, ts.URL)
	require.NoError(t, err)
	require.NoError(t, c.AddHost("shielded.test"))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://shielded.test/item", nil)
	require.NoError(t, err)

	rsp, err := c.Do(req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, _ := io.ReadAll(rsp.Body)
	assert.Equal(t, "<html>bypassed</html>", string(body))
	assert.Equal(t, 1, calls)
}

func TestSolverClient_Do_bypass_solver_error_status(t *testing.T) {
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(flareResponse{
			Status:  "error",
			Message: "challenge failed",
		})
	}))
	defer ts.Close()

	impl := &mockHTTPClient{doFn: func(*http.Request) (*http.Response, error) {
		return okEmptyResponse(), nil
	}}
	c, err := New(impl, ts.URL)
	require.NoError(t, err)
	require.NoError(t, c.AddHost("badflare.test"))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://badflare.test/", nil)
	require.NoError(t, err)

	rsp, err := c.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Nil(t, rsp)
	assert.ErrorIs(t, err, errFlareResponseStatus)
	assert.Contains(t, err.Error(), "challenge failed")
}

func TestSolverClient_Do_post_rejected_for_bypass_host(t *testing.T) {
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "solver should not be called when method is POST", http.StatusInternalServerError)
	}))
	defer ts.Close()

	impl := &mockHTTPClient{doFn: func(*http.Request) (*http.Response, error) {
		return okEmptyResponse(), nil
	}}
	c, err := New(impl, ts.URL)
	require.NoError(t, err)
	require.NoError(t, c.AddHost("postblock.test"))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://postblock.test/submit", strings.NewReader("x"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	rsp, err := c.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Nil(t, rsp)
	assert.ErrorIs(t, err, errFlareOnlyGET)
}

func TestSolverClient_Do_test_host_failure(t *testing.T) {
	impl := &mockHTTPClient{doFn: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("solver unreachable")
	}}
	c, err := New(impl, "http://127.0.0.1:9")
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://any.test/", nil)
	require.NoError(t, err)

	rsp, err := c.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Nil(t, rsp)
	assert.Contains(t, err.Error(), "test solver host failed")
	assert.Contains(t, err.Error(), "http://127.0.0.1:9")
}

func TestSolverClient_Do_passthrough_impl_error(t *testing.T) {
	ts := httptest.NewServer(newFlareSolverMux(t, func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()

	var calls int
	impl := &mockHTTPClient{doFn: func(_ *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return okEmptyResponse(), nil
		}
		return nil, errors.New("upstream closed")
	}}

	c, err := New(impl, ts.URL)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://passthrough-err.test/", nil)
	require.NoError(t, err)

	rsp, err := c.Do(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Nil(t, rsp)
	assert.Contains(t, err.Error(), "solver passthrough request")
	assert.Equal(t, 2, calls)
}

func TestSolverClient_handleByPassRequest_decode_error(t *testing.T) {
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json{"))
	}))
	defer ts.Close()

	b := &solverClient{
		endpoint:  ts.URL,
		timeout:   time.Second,
		byPastMap: make(map[string]struct{}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://decode.test/p", nil)
	require.NoError(t, err)

	rsp, err := b.handleByPassRequest(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Nil(t, rsp)
	assert.Contains(t, err.Error(), "decode flare response")
}

// abruptCloseListener accepts one TCP connection and closes it immediately so
// http.Post fails quickly without relying on OS TCP timeout behavior.
func abruptCloseListener(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})
	return "http://" + ln.Addr().String()
}

func TestSolverClient_handleByPassRequest_post_error(t *testing.T) {
	b := &solverClient{
		endpoint:  abruptCloseListener(t),
		timeout:   time.Second,
		byPastMap: make(map[string]struct{}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://posterr.test/", nil)
	require.NoError(t, err)

	rsp, err := b.handleByPassRequest(req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Nil(t, rsp)
	assert.Contains(t, err.Error(), "post to flare solver")
}

func TestSolverClient_handleByPassRequest_success_direct(t *testing.T) {
	payload := flareResponse{
		Status: "ok",
		Solution: flareSolution{
			Status:   201,
			Response: "payload",
		},
	}
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer ts.Close()

	b := &solverClient{
		endpoint:  ts.URL,
		timeout:   2 * time.Second,
		byPastMap: make(map[string]struct{}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://direct.test/z", nil)
	require.NoError(t, err)

	rsp, err := b.handleByPassRequest(req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, 201, rsp.StatusCode)
	body, _ := io.ReadAll(rsp.Body)
	assert.Equal(t, "payload", string(body))
}
