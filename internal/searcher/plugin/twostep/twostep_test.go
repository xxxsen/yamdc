package twostep

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

func deepBrokenHTML() string {
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString("<div>")
	}
	for i := 0; i < 600; i++ {
		sb.WriteString("</div>")
	}
	return sb.String()
}

func listPageHTML() string {
	return `<!DOCTYPE html><html><body>
<ul id="links"><li><a href="/detail/1">one</a></li><li><a href="/detail/2">two</a></li></ul>
</body></html>`
}

func TestIsCodeInValidStatusCodeList(t *testing.T) {
	assert.True(t, isCodeInValidStatusCodeList([]int{200, 404}, 200))
	assert.False(t, isCodeInValidStatusCodeList([]int{200}, 404))
	assert.False(t, isCodeInValidStatusCodeList(nil, 200))
}

func TestHandleXPathTwoStepSearch_FirstInvokerError(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network down")
	}
	xctx := &XPathTwoStepContext{ValidStatusCode: []int{200}}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), invoker, req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step search failed")
}

func TestHandleXPathTwoStepSearch_InvalidStatus(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("err")),
		}, nil
	}
	xctx := &XPathTwoStepContext{ValidStatusCode: []int{200}}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotInValidList)
}

func TestHandleXPathTwoStepSearch_HTMLParseFails(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(deepBrokenHTML())),
		}, nil
	}
	xctx := &XPathTwoStepContext{ValidStatusCode: []int{200}}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read data as html")
}

func TestHandleXPathTwoStepSearch_ResultCountMismatch(t *testing.T) {
	html := `<html><body>
<span id="a">1</span><span id="a">2</span>
<span id="b">only</span>
</body></html>`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(html))}, nil
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode:       []int{200},
		CheckResultCountMatch: true,
		Ps: []*XPathPair{
			{Name: "a", XPath: `//*[@id='a']`},
			{Name: "b", XPath: `//*[@id='b']`},
		},
		LinkSelector: func(_ []*XPathPair) (string, bool, error) { return "", false, nil },
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errResultCountMismatch)
}

func TestHandleXPathTwoStepSearch_NoResultsWhenCheckEnabled(t *testing.T) {
	html := `<html><body><span id="a">x</span></body></html>`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(html))}, nil
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode:       []int{200},
		CheckResultCountMatch: true,
		Ps: []*XPathPair{
			{Name: "a", XPath: `//*[@id='missing']`},
		},
		LinkSelector: func(_ []*XPathPair) (string, bool, error) { return "", false, nil },
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoResultFound)
}

func TestHandleXPathTwoStepSearch_LinkSelectorError(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(listPageHTML()))}, nil
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode: []int{200},
		Ps: []*XPathPair{
			{Name: "hrefs", XPath: `//*[@id='links']//a/@href`},
		},
		LinkSelector: func(_ []*XPathPair) (string, bool, error) {
			return "", false, fmt.Errorf("user reject")
		},
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "select link from result")
}

func TestHandleXPathTwoStepSearch_LinkSelectorNotOK(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(listPageHTML()))}, nil
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode: []int{200},
		Ps: []*XPathPair{
			{Name: "hrefs", XPath: `//*[@id='links']//a/@href`},
		},
		LinkSelector: func(_ []*XPathPair) (string, bool, error) {
			return "", false, nil
		},
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoLinkSelectResult)
}

func TestHandleXPathTwoStepSearch_SecondRequestBuildFails(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(listPageHTML()))}, nil
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode: []int{200},
		Ps: []*XPathPair{
			{Name: "hrefs", XPath: `//*[@id='links']//a/@href`},
		},
		LinkSelector: func(_ []*XPathPair) (string, bool, error) {
			return ":", true, nil // invalid URL when joined with empty prefix
		},
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-create result page link")
}

func TestHandleXPathTwoStepSearch_SecondInvokerError(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	calls := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(listPageHTML()))}, nil
		}
		return nil, fmt.Errorf("second hop failed")
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode: []int{200},
		LinkPrefix:      "https://example.com",
		Ps: []*XPathPair{
			{Name: "hrefs", XPath: `//*[@id='links']//a/@href`},
		},
		LinkSelector: func(ps []*XPathPair) (string, bool, error) {
			require.NotEmpty(t, ps[0].Result)
			return ps[0].Result[0], true, nil
		},
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "second hop")
}

func TestHandleXPathTwoStepSearch_Success(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/start", nil)
	require.NoError(t, err)
	finalBody := `<html><body><h1>done</h1></body></html>`
	calls := 0
	invoker := func(_ context.Context, r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(listPageHTML()))}, nil
		}
		assert.Equal(t, "https://example.com/detail/1", r.URL.String())
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(finalBody))}, nil
	}
	xctx := &XPathTwoStepContext{
		ValidStatusCode: []int{200},
		LinkPrefix:      "https://example.com",
		Ps: []*XPathPair{
			{Name: "hrefs", XPath: `//*[@id='links']//a/@href`},
		},
		LinkSelector: func(ps []*XPathPair) (string, bool, error) {
			return ps[0].Result[0], true, nil
		},
	}
	rsp, err := HandleXPathTwoStepSearch(context.Background(), api.HTTPInvoker(invoker), req, xctx)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	data, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(data), "done")
	_ = rsp.Body.Close()
}
