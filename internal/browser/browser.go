package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"

	"github.com/xxxsen/yamdc/internal/client"
)

// NavigateResult holds the output of a browser navigation.
//
// NOTE: Browser mode always returns HTTP 200 regardless of the actual page
// status code. This means plugin-level status code checks
// (accept_status_codes / not_found_status_codes) do not take effect when
// fetch_type=browser. Plugins should rely on scrape-layer field validation
// (required fields) to detect invalid pages instead.
type NavigateResult struct {
	HTML    []byte
	Cookies []*http.Cookie
}

type INavigator interface {
	Navigate(ctx context.Context, url string, params *Params) (*NavigateResult, error)
	Close() error
}

type httpClientWrap struct {
	impl      client.IHTTPClient
	navigator INavigator
	jar       *cookiejar.Jar
}

func NewHTTPClient(impl client.IHTTPClient, navigator INavigator) client.IHTTPClient {
	jar, _ := cookiejar.New(nil)
	return &httpClientWrap{impl: impl, navigator: navigator, jar: jar}
}

func (c *httpClientWrap) Do(req *http.Request) (*http.Response, error) {
	params := GetParams(req.Context())
	if params == nil {
		c.injectCookies(req)
		return c.impl.Do(req)
	}
	localParams := *params
	localParams.Cookies = append(req.Cookies(), c.jar.Cookies(req.URL)...)
	if params.Headers != nil {
		localParams.Headers = params.Headers.Clone()
	}
	result, err := c.navigator.Navigate(req.Context(), req.URL.String(), &localParams)
	if err != nil {
		return nil, fmt.Errorf("browser navigate failed: %w", err)
	}
	if len(result.Cookies) > 0 {
		c.jar.SetCookies(req.URL, result.Cookies)
	}
	return &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		ContentLength: int64(len(result.HTML)),
		Body:          io.NopCloser(bytes.NewReader(result.HTML)),
		Request:       req,
	}, nil
}

// injectCookies adds browser-originated cookies to the request so that
// subsequent non-browser HTTP calls (e.g. image downloads) carry the
// session established during browser navigation.
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
