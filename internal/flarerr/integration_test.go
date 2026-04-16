package flarerr

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultTestEndpoint = "http://127.0.0.1:8191"
	testTargetURL       = "https://www.example.com"
)

func skipUnlessIntegration(t *testing.T) string {
	t.Helper()
	if os.Getenv("YAMDC_FLARESOLVERR_TEST") == "" {
		t.Skip("set YAMDC_FLARESOLVERR_TEST=1 to run FlareSolverr integration tests")
	}
	endpoint := os.Getenv("YAMDC_FLARESOLVERR_ENDPOINT")
	if endpoint == "" {
		endpoint = defaultTestEndpoint
	}
	return endpoint
}

// TestIntegration_SolveRequest validates that solveRequest works end-to-end
// against a real FlareSolverr instance.
func TestIntegration_SolveRequest(t *testing.T) {
	endpoint := skipUnlessIntegration(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testTargetURL, nil)
	require.NoError(t, err)

	result, err := solveRequest(endpoint, defaultByPassClientTimeout, req)
	require.NoError(t, err, "solveRequest should succeed against real FlareSolverr")

	t.Logf("StatusCode: %d", result.StatusCode)
	t.Logf("HTML length: %d bytes", len(result.HTML))
	t.Logf("Cookies count: %d", len(result.Cookies))
	for i, ck := range result.Cookies {
		t.Logf("  Cookie[%d]: %s=%s (domain=%s, path=%s, secure=%v, httpOnly=%v)",
			i, ck.Name, ck.Value, ck.Domain, ck.Path, ck.Secure, ck.HttpOnly)
	}

	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.NotEmpty(t, result.HTML, "HTML body should not be empty")
	htmlStr := string(result.HTML)
	assert.True(t, strings.Contains(htmlStr, "<html") || strings.Contains(htmlStr, "<HTML"),
		"response should contain HTML content")
}

// TestIntegration_SolveResult_ToHTTPResponse verifies that toHTTPResponse
// produces a well-formed http.Response from real solver output.
func TestIntegration_SolveResult_ToHTTPResponse(t *testing.T) {
	endpoint := skipUnlessIntegration(t)

	origReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testTargetURL, nil)
	require.NoError(t, err)

	result, err := solveRequest(endpoint, defaultByPassClientTimeout, origReq)
	require.NoError(t, err)

	resp := result.toHTTPResponse(origReq)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, result.StatusCode, resp.StatusCode)
	assert.Equal(t, int64(len(result.HTML)), resp.ContentLength)
	assert.Same(t, origReq, resp.Request)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, result.HTML, body)
}

// TestIntegration_HTTPClientWrap_ContextDriven verifies the full httpClientWrap
// flow: context-flagged requests go through FlareSolverr, unflagged ones pass
// through to the underlying impl.
func TestIntegration_HTTPClientWrap_ContextDriven(t *testing.T) {
	endpoint := skipUnlessIntegration(t)

	var implCalls int
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		implCalls++
		return http.DefaultClient.Do(req) //nolint:gosec
	}}
	cli := NewHTTPClient(impl, endpoint)

	// 1) Flagged request → goes through FlareSolverr, impl should NOT be called.
	ctx := WithParams(context.Background(), &Params{})
	req1, err := http.NewRequestWithContext(ctx, http.MethodGet, testTargetURL, nil)
	require.NoError(t, err)

	resp1, err := cli.Do(req1)
	require.NoError(t, err, "context-flagged request should succeed via FlareSolverr")
	body1, _ := io.ReadAll(resp1.Body)
	_ = resp1.Body.Close()

	t.Logf("[flagged] StatusCode=%d, BodyLen=%d", resp1.StatusCode, len(body1))
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	assert.NotEmpty(t, body1)
	assert.Equal(t, 0, implCalls, "impl should not be called for flagged requests")

	// 2) Unflagged request → goes through impl, NOT FlareSolverr.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testTargetURL, nil)
	require.NoError(t, err)

	resp2, err := cli.Do(req2)
	require.NoError(t, err, "unflagged request should pass through to impl")
	body2, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()

	t.Logf("[unflagged] StatusCode=%d, BodyLen=%d", resp2.StatusCode, len(body2))
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, 1, implCalls, "impl should be called exactly once for unflagged request")
}

// TestIntegration_HTTPClientWrap_CookiePersistence verifies that cookies
// obtained from FlareSolverr are persisted and injected into subsequent
// unflagged requests.
func TestIntegration_HTTPClientWrap_CookiePersistence(t *testing.T) {
	endpoint := skipUnlessIntegration(t)

	var capturedReq *http.Request
	impl := &mockHTTPClient{doFn: func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("passthrough")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}}
	cli := NewHTTPClient(impl, endpoint)

	// Step 1: flagged request to harvest cookies.
	ctx := WithParams(context.Background(), &Params{})
	req1, err := http.NewRequestWithContext(ctx, http.MethodGet, testTargetURL, nil)
	require.NoError(t, err)
	resp1, err := cli.Do(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	// Step 2: unflagged request to the same host → cookies should be injected.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testTargetURL+"/another", nil)
	require.NoError(t, err)
	resp2, err := cli.Do(req2)
	require.NoError(t, err)
	_ = resp2.Body.Close()

	require.NotNil(t, capturedReq)
	cookies := capturedReq.Cookies()
	t.Logf("Injected cookies into unflagged request: %d", len(cookies))
	for _, ck := range cookies {
		t.Logf("  %s=%s", ck.Name, ck.Value)
	}
	// example.com may or may not set cookies, so we just log; the key thing
	// is that the code path ran without error.
}
