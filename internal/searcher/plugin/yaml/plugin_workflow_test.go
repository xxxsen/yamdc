package yaml

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestCheckBaseResponseStatus(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	err := checkBaseResponseStatus(plg, 200)
	require.NoError(t, err)

	err = checkBaseResponseStatus(plg, 500)
	require.Error(t, err)
}

// --- NewFromBytes ---

func TestCheckBaseResponseStatus_WithMultiRequest(t *testing.T) {
	reqSpec := &compiledRequest{acceptStatusCodes: []int{200}}
	plg := &SearchPlugin{spec: &compiledPlugin{
		multiRequest: &compiledMultiRequest{request: reqSpec},
	}}
	assert.NoError(t, checkBaseResponseStatus(plg, 200))
	assert.Error(t, checkBaseResponseStatus(plg, 500))
}

func TestCheckBaseResponseStatus_NoRequest(t *testing.T) {
	plg := &SearchPlugin{spec: &compiledPlugin{}}
	assert.NoError(t, checkBaseResponseStatus(plg, 200))
	assert.NoError(t, checkBaseResponseStatus(plg, 500))
}

// --- Workflow with match conditions ---

func TestCollectSelectorResults(t *testing.T) {
	htmlStr := `<html><body><a href="1">A</a><a href="2">B</a></body></html>`
	node, _ := parseHTML(htmlStr)
	selectors := []*compiledSelectorList{
		{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
		{name: "text", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/text()"}},
	}
	results, count, err := collectSelectorResults(node, selectors)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []string{"1", "2"}, results["link"])
	assert.Equal(t, []string{"A", "B"}, results["text"])
}

func TestCollectSelectorResults_Mismatch(t *testing.T) {
	htmlStr := `<html><body><a href="1">A</a><a href="2">B</a><span>only-one</span></body></html>`
	node, _ := parseHTML(htmlStr)
	selectors := []*compiledSelectorList{
		{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
		{name: "span", compiledSelector: compiledSelector{kind: "xpath", expr: "//span/text()"}},
	}
	_, _, err := collectSelectorResults(node, selectors)
	assert.ErrorIs(t, err, errSelectorCountMismatch)
}

func TestHandleResponse_SelectorCountMismatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
      - name: text
        kind: xpath
        expr: "//span/text()"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		body := `<html><body><a href="/1">A</a><a href="/2">B</a><span>only one</span></body></html>`
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(body)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- handleResponse with no match ---

func TestHandleResponse_NoMatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
      - name: text
        kind: xpath
        expr: "//a/text()"
    match:
      mode: and
      conditions:
        - 'contains("${item.text}", "NOPE")'
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		body := `<html><body><a href="/1">A</a><a href="/2">B</a></body></html>`
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(body)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- followNextRequest error ---

func TestHandleResponse_NextRequestFails(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Body:       nopCloser([]byte("not found")),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- OnPrecheckResponse: all branches ---

func TestFollowNextRequest_StatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
      response:
        accept_status_codes: [200]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 403,
			Body:       nopCloser([]byte("forbidden")),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- handleRequest invoker error ---

func TestHandleRequest_InvokerError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- multiRequest.handle: invoker error ---

func TestMultiRequestHandle_InvokerError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection timeout")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- ConditionGroup OR mode evaluation ---

func TestHandleResponse_ReadBodyError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  response:
    decode_charset: "unknown-charset"
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><a href="/1">a</a></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- followNextRequest invoker error ---

func TestFollowNextRequest_InvokerError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return nil, fmt.Errorf("detail page unreachable")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- multiRequest.handle with status rejected then skip ---

func TestMultiRequestHandle_StatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["bad", "${number}"]
  request:
    method: GET
    path: /search/${candidate}
    response:
      accept_status_codes: [200]
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 404,
				Body:       nopCloser([]byte("not found")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><div class="found">ok</div><title>Found</title></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
}

// --- multiRequest.handle: unique dedup ---

func TestMultiRequestHandle_UniqueDedup(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}", "${number}"]
  unique: true
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><div>ok</div></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, 1, callCount)
}

// --- handleResponse with checkBaseResponseStatus rejection (multi_request path) ---

func TestHandleResponse_BaseStatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
    response:
      accept_status_codes: [200]
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       nopCloser([]byte(`<html><body><div>ok</div><a href="/x">x</a></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}
