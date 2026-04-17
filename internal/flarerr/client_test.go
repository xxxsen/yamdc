package flarerr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEndpoint(t *testing.T) {
	t.Parallel()
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
			assert.Equal(t, tt.want, normalizeEndpoint(tt.in))
		})
	}
}

func TestSolveRequest_nonGETRejected(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://origin.test/", nil)
	require.NoError(t, err)
	result, err := solveRequest("http://unused", 1500*time.Millisecond, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errFlareOnlyGET)
	assert.Contains(t, err.Error(), http.MethodPost)
}

func newFlareSolverMux(t *testing.T, v1Handler http.HandlerFunc) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1", v1Handler)
	return mux
}

func TestSolveRequest_success(t *testing.T) {
	t.Parallel()
	payload := flareResponse{
		Status: "ok",
		Solution: flareSolution{
			Status:   http.StatusOK,
			Response: "<html>hello</html>",
			Cookies: []flareCookie{
				{Name: "cf_clearance", Value: "abc123", Domain: ".example.com", Path: "/", Secure: true, HTTPOnly: true},
			},
		},
	}
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var got flareRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "request.get", got.Cmd)
		assert.Equal(t, "https://target.test/page", got.URL)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://target.test/page", nil)
	require.NoError(t, err)
	result, err := solveRequest(ts.URL, 2*time.Second, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, "<html>hello</html>", string(result.HTML))
	require.Len(t, result.Cookies, 1)
	assert.Equal(t, "cf_clearance", result.Cookies[0].Name)
	assert.Equal(t, "abc123", result.Cookies[0].Value)
	assert.True(t, result.Cookies[0].Secure)
	assert.True(t, result.Cookies[0].HttpOnly)
}

func TestSolveRequest_errorStatus(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(flareResponse{
			Status:  "error",
			Message: "challenge failed",
		})
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://target.test/", nil)
	require.NoError(t, err)
	result, err := solveRequest(ts.URL, time.Second, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errFlareResponseStatus)
	assert.Contains(t, err.Error(), "challenge failed")
}

func TestSolveRequest_decodeError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json{"))
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://target.test/", nil)
	require.NoError(t, err)
	result, err := solveRequest(ts.URL, time.Second, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "decode flare response")
}

// abruptCloseListener accepts TCP connections and closes them immediately.
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

func TestSolveRequest_postError(t *testing.T) {
	t.Parallel()
	endpoint := abruptCloseListener(t)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://target.test/", nil)
	require.NoError(t, err)
	result, err := solveRequest(endpoint, time.Second, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "post to flare solver")
}

func TestSolveResult_toHTTPResponse(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/", nil)
	require.NoError(t, err)
	sr := &solveResult{
		StatusCode: 201,
		HTML:       []byte("payload"),
	}
	resp := sr.toHTTPResponse(req)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 201, resp.StatusCode)
	assert.Equal(t, int64(len("payload")), resp.ContentLength)
	assert.Same(t, req, resp.Request)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "payload", string(body))
}

func TestSolveRequest_noCookies(t *testing.T) {
	t.Parallel()
	payload := flareResponse{
		Status: "ok",
		Solution: flareSolution{
			Status:   http.StatusOK,
			Response: "nocookies",
		},
	}
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://target.test/", nil)
	require.NoError(t, err)
	result, err := solveRequest(ts.URL, time.Second, req)
	require.NoError(t, err)
	assert.Empty(t, result.Cookies)
}

func TestSolveResult_toHTTPResponse_emptyBody(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/", nil)
	require.NoError(t, err)
	sr := &solveResult{StatusCode: 204, HTML: nil}
	resp := sr.toHTTPResponse(req)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 204, resp.StatusCode)
	assert.Equal(t, int64(0), resp.ContentLength)
	body, _ := io.ReadAll(resp.Body)
	assert.Empty(t, body)
}

func TestSolveRequest_maxTimeoutPassedCorrectly(t *testing.T) {
	t.Parallel()
	var captured flareRequest
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(flareResponse{
			Status:   "ok",
			Solution: flareSolution{Status: 200, Response: "ok"},
		})
	}))
	defer ts.Close()

	timeout := 12345 * time.Millisecond
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://target.test/", nil)
	require.NoError(t, err)
	_, err = solveRequest(ts.URL, timeout, req)
	require.NoError(t, err)
	assert.Equal(t, int(timeout.Milliseconds()), captured.MaxTimeout)
}

func TestSolveRequest_multipleCookies(t *testing.T) {
	t.Parallel()
	payload := flareResponse{
		Status: "ok",
		Solution: flareSolution{
			Status:   200,
			Response: "multi",
			Cookies: []flareCookie{
				{Name: "a", Value: "1", Domain: ".t.com", Path: "/"},
				{Name: "b", Value: "2", Domain: ".t.com", Path: "/x", Secure: true},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	ts := httptest.NewServer(newFlareSolverMux(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://t.com/", nil)
	require.NoError(t, err)
	result, err := solveRequest(ts.URL, time.Second, req)
	require.NoError(t, err)
	require.Len(t, result.Cookies, 2)
	assert.Equal(t, "a", result.Cookies[0].Name)
	assert.Equal(t, "b", result.Cookies[1].Name)
	assert.True(t, result.Cookies[1].Secure)
}

func TestSolveResult_toHTTPResponse_largeBody(t *testing.T) {
	t.Parallel()
	big := bytes.Repeat([]byte("x"), 1<<16)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/", nil)
	require.NoError(t, err)
	sr := &solveResult{StatusCode: 200, HTML: big}
	resp := sr.toHTTPResponse(req)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, int64(len(big)), resp.ContentLength)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, big, body)
}
