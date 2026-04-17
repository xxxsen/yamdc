package flarerr

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
)

// httpClientWrap is a context-driven wrapper that routes requests through
// FlareSolverr when flarerr.Params is present in the request context.
// It mirrors the design of browser.httpClientWrap.
type httpClientWrap struct {
	impl     client.IHTTPClient
	endpoint string
	timeout  time.Duration
	jar      *cookiejar.Jar
}

// NewHTTPClient wraps impl so that requests carrying flarerr.Params in their
// context are automatically routed through the FlareSolverr endpoint.
// Non-flagged requests pass through to impl with cookie injection.
func NewHTTPClient(impl client.IHTTPClient, endpoint string) client.IHTTPClient {
	jar, _ := cookiejar.New(nil)
	return &httpClientWrap{
		impl:     impl,
		endpoint: normalizeEndpoint(endpoint),
		timeout:  defaultByPassClientTimeout,
		jar:      jar,
	}
}

func (c *httpClientWrap) Do(req *http.Request) (*http.Response, error) {
	params := GetParams(req.Context())
	if params == nil {
		c.injectCookies(req)
		resp, err := c.impl.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		return resp, nil
	}
	result, err := solveRequest(c.endpoint, c.timeout, req)
	if err != nil {
		return nil, fmt.Errorf("flaresolverr solve failed: %w", err)
	}
	if len(result.Cookies) > 0 {
		c.jar.SetCookies(req.URL, result.Cookies)
	}
	return result.toHTTPResponse(req), nil
}

// injectCookies adds solver-originated cookies to the request so that
// subsequent non-solver HTTP calls (e.g. image downloads) carry the
// session established during FlareSolverr navigation.
func (c *httpClientWrap) injectCookies(req *http.Request) {
	existing := make(map[string]struct{})
	for _, ck := range req.Cookies() {
		existing[ck.Name] = struct{}{}
	}
	for _, ck := range c.jar.Cookies(req.URL) {
		if _, dup := existing[ck.Name]; !dup {
			req.AddCookie(ck)
		}
	}
}
