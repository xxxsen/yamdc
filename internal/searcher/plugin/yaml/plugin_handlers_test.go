package yaml

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestOnPrecheckRequest(t *testing.T) {
	plgYAML := `
version: 1
name: test
type: one-step
hosts:
  - https://example.com
precheck:
  number_patterns:
    - "^ABC"
  variables:
    clean_num: ${clean_number(${number})}
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true
`
	plg := mustCompilePlugin(t, plgYAML)
	ctx := pluginapi.InitContainer(context.Background())

	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = plg.OnPrecheckRequest(ctx, "XYZ-123")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckRequest_NoPrecheck(t *testing.T) {
	plgYAML := `
version: 1
name: test
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
`
	plg := mustCompilePlugin(t, plgYAML)
	ctx := pluginapi.InitContainer(context.Background())
	ok, err := plg.OnPrecheckRequest(ctx, "anything")
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- OnPrecheckResponse ---

func TestOnPrecheckResponse(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)

	tests := []struct {
		name   string
		code   int
		expect bool
	}{
		{name: "200", code: 200, expect: true},
		{name: "404", code: 404, expect: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsp := &http.Response{StatusCode: tt.code}
			ok, err := plg.OnPrecheckResponse(context.Background(), req, rsp)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, ok)
		})
	}
}

// --- OnDecorateMediaRequest ---

func TestOnDecorateMediaRequest(t *testing.T) {
	plgYAML := `
version: 1
name: test
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
  headers:
    X-Custom: header-val
  cookies:
    session: cookie-val
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
`
	plg := mustCompilePlugin(t, plgYAML)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, "https://example.com/page")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "header-val", req.Header.Get("X-Custom"))
	assert.Equal(t, "https://example.com/page", req.Header.Get("Referer"))
}

// --- buildRequestBodyReader ---

func TestOnGetHosts(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	hosts := plg.OnGetHosts(context.Background())
	assert.Equal(t, []string{"https://example.com"}, hosts)
}

// --- OnDecorateRequest (no-op) ---

func TestOnDecorateRequest(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	err := plg.OnDecorateRequest(context.Background(), req)
	require.NoError(t, err)
}

// --- setBodyContentType ---

func TestOnPrecheckRequest_WithPattern(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
precheck:
  number_patterns: ["^[A-Z]+-\\d+$"]
  variables:
    slug: "${to_lower(${number})}"
request:
  method: GET
  path: /search/${vars.slug}
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

	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = plg.OnPrecheckRequest(ctx, "abc123")
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- OnMakeHTTPRequest ---

func TestOnMakeHTTPRequest(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Contains(t, req.URL.String(), "/search/ABC-123")
}

func TestOnMakeHTTPRequest_MultiRequest(t *testing.T) {
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
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, req)
}

func TestOnPrecheckResponse_AcceptStatusCodes(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  accept_status_codes: [200, 301]
  not_found_status_codes: [404]
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
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)

	ok, err := plg.OnPrecheckResponse(ctx, req, &http.Response{StatusCode: 200})
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, _ = plg.OnPrecheckResponse(ctx, req, &http.Response{StatusCode: 404})
	assert.False(t, ok)

	_, err = plg.OnPrecheckResponse(ctx, req, &http.Response{StatusCode: 500})
	assert.Error(t, err)
}

// --- OnDecodeHTTPData ---

func TestOnDecodeHTTPData_HTML(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><head><title>MyTitle</title></head></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "MyTitle", mv.Title)
}

func TestOnDecodeHTTPData_JSON(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /api/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: "$.title"
    actors:
      selector:
        kind: jsonpath
        expr: "$.actors[*]"
      parser: string_list
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`{"title":"T","actors":["A","B"]}`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "T", mv.Title)
	assert.Equal(t, []string{"A", "B"}, mv.Actors)
}

func TestOnDecodeHTTPData_NotFound(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1[@class="title"]/text()
      required: true
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	data := []byte(`<html><body><h1>no-class</h1></body></html>`)
	_, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- applyPostprocess ---

func TestOnHandleHTTPRequest_MultiRequest_Match(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}", "${to_lower(${number})}"]
  unique: true
  request:
    method: GET
    path: /search/${candidate}
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

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		body := `<html><body><div class="found">ok</div></body></html>`
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(body)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, 200, rsp.StatusCode)
}

func TestOnHandleHTTPRequest_MultiRequest_NoMatch(t *testing.T) {
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

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body>no match</body></html>`)),
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

func TestOnHandleHTTPRequest_MultiRequest_StatusRejected(t *testing.T) {
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
		return &http.Response{
			StatusCode: 404,
			Body:       nopCloser([]byte("not found")),
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

// --- OnHandleHTTPRequest with workflow ---

func TestOnHandleHTTPRequest_Workflow(t *testing.T) {
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
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>Detail</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

func TestOnHandleHTTPRequest_PlainRequest(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><title>T</title></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

// --- OnHandleHTTPRequest with multi_request + workflow ---

func TestOnHandleHTTPRequest_MultiRequest_Workflow(t *testing.T) {
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
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
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
				Body:       nopCloser([]byte(`<html><body><div class="found">ok</div><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>Detail</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

// --- checkBaseResponseStatus ---

func TestOnHandleHTTPRequest_WorkflowWithMatch(t *testing.T) {
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
      - name: txt
        kind: xpath
        expr: "//a/text()"
    item_variables:
      slug: "${item.link}"
    match:
      mode: and
      conditions:
        - 'contains("${item.txt}", "video")'
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
				Body: nopCloser([]byte(`<html><body>
					<a href="/detail/1">image</a>
					<a href="/detail/2">video page</a>
				</body></html>`)),
				Header: make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>Detail</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

// --- Workflow with expect_count ---

func TestOnHandleHTTPRequest_WorkflowExpectCountMismatch(t *testing.T) {
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
    match:
      mode: and
      conditions:
        - 'contains("${item.link}", "video")'
      expect_count: 5
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
			Body:       nopCloser([]byte(`<html><body><a href="/video/1">video</a></body></html>`)),
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

// --- OnDecodeHTTPData with JSON required not found ---

func TestOnDecodeHTTPData_JSON_RequiredNotFound(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /api/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: "$.title"
      required: true
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`{"other":"value"}`)
	_, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnDecodeHTTPData_JSON_WithListFields(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /api/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: "$.title"
    genres:
      selector:
        kind: jsonpath
        expr: "$.genres[*]"
      parser: string_list
    sample_images:
      selector:
        kind: jsonpath
        expr: "$.images[*]"
      parser: string_list
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`{"title":"T","genres":["Action","Drama"],"images":["img1.jpg"]}`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"Action", "Drama"}, mv.Genres)
	assert.Len(t, mv.SampleImages, 1)
}

// --- compilePlugin with full workflow + multi_request ---

func TestOnDecodeHTTPData_HTML_AllFields(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
    actors:
      selector:
        kind: xpath
        expr: "//div[@class='actors']/span/text()"
      parser: string_list
    genres:
      selector:
        kind: xpath
        expr: "//div[@class='genres']/span/text()"
      parser: string_list
      required: true
    sample_images:
      selector:
        kind: xpath
        expr: "//div[@class='images']/img/@src"
      parser: string_list
    cover:
      selector:
        kind: xpath
        expr: "//img[@class='cover']/@src"
    poster:
      selector:
        kind: xpath
        expr: "//img[@class='poster']/@src"
    number:
      selector:
        kind: xpath
        expr: "//span[@class='num']/text()"
    plot:
      selector:
        kind: xpath
        expr: "//div[@class='plot']/text()"
    studio:
      selector:
        kind: xpath
        expr: "//span[@class='studio']/text()"
    label:
      selector:
        kind: xpath
        expr: "//span[@class='label']/text()"
    director:
      selector:
        kind: xpath
        expr: "//span[@class='director']/text()"
    series:
      selector:
        kind: xpath
        expr: "//span[@class='series']/text()"
    release_date:
      selector:
        kind: xpath
        expr: "//span[@class='date']/text()"
      parser: date_only
    duration:
      selector:
        kind: xpath
        expr: "//span[@class='dur']/text()"
      parser: duration_default
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><body>
		<h1>Title</h1>
		<div class="actors"><span>A</span><span>B</span></div>
		<div class="genres"><span>Action</span></div>
		<div class="images"><img src="img1.jpg"/></div>
		<img class="cover" src="cover.jpg"/>
		<img class="poster" src="poster.jpg"/>
		<span class="num">NUM-1</span>
		<div class="plot">Plot text</div>
		<span class="studio">Studio</span>
		<span class="label">Label</span>
		<span class="director">Dir</span>
		<span class="series">Series</span>
		<span class="date">2024-01-15</span>
		<span class="dur">120分</span>
	</body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Title", mv.Title)
	assert.Equal(t, []string{"A", "B"}, mv.Actors)
	assert.Equal(t, []string{"Action"}, mv.Genres)
	assert.Len(t, mv.SampleImages, 1)
	assert.Equal(t, "cover.jpg", mv.Cover.Name)
	assert.Equal(t, "poster.jpg", mv.Poster.Name)
}

// --- nopCloser helper ---

func TestOnDecorateMediaRequest_WithReferer(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, "https://example.com/page")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", req.Header.Get("Referer"))
}

// --- resolveRequestURL with rawURL ---

func TestOnDecodeHTTPData_UnsupportedCharset(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  response:
    decode_charset: "windows-1252"
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
	_, _, err := plg.OnDecodeHTTPData(ctx, []byte(`<html><title>T</title></html>`))
	require.Error(t, err)
}

// --- renderFormBody / renderJSONBody / renderRawBody ---

func TestOnDecodeHTTPData_WithPostprocess(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
postprocess:
  assign:
    title: "${meta.title} (post)"
  defaults:
    title_lang: ja
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><head><title>OrigTitle</title></head></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, mv.Title, "post")
}

// --- handleResponse error paths ---

func TestOnPrecheckResponse_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		status   int
		expected bool
	}{
		{
			name: "no_final_request_404",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   404,
			expected: false,
		},
		{
			name: "not_found_status_code",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  not_found_status_codes: [302, 301]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   302,
			expected: false,
		},
		{
			name: "accept_status_codes_match",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  accept_status_codes: [200, 201]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   201,
			expected: true,
		},
		{
			name: "no_accept_codes_with_404",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   404,
			expected: false,
		},
		{
			name: "no_accept_codes_non_404",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   200,
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plg := mustCompilePlugin(t, tc.yaml)
			ctx := pluginapi.InitContainer(context.Background())
			rsp := &http.Response{StatusCode: tc.status, Body: nopCloser(nil)}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
			ok, err := plg.OnPrecheckResponse(ctx, req, rsp)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

// --- OnPrecheckResponse: accept_status_codes reject with error ---

func TestOnPrecheckResponse_AcceptCodesReject(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  accept_status_codes: [200]
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
	rsp := &http.Response{StatusCode: 500, Body: nopCloser(nil)}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	ok, err := plg.OnPrecheckResponse(ctx, req, rsp)
	require.Error(t, err)
	assert.False(t, ok)
}

// --- buildRequest: empty host fallback ---

func TestOnMakeHTTPRequest_NilRequest_WithMultiRequest(t *testing.T) {
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
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.NotNil(t, req)
}

// --- OnDecodeHTTPData_JSON_InvalidJSON ---

func TestOnDecodeHTTPData_JSON_InvalidJSON(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: $.title
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, _, err := plg.OnDecodeHTTPData(ctx, []byte(`not valid json`))
	require.Error(t, err)
}

// --- OnDecodeHTTPData unsupported format ---

func TestOnDecodeHTTPData_UnsupportedFormat2(t *testing.T) {
	spec, err := compilePlugin(&PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	})
	require.NoError(t, err)
	spec.scrape.format = "xml"
	plg := &SearchPlugin{spec: spec}
	ctx := pluginapi.InitContainer(context.Background())
	_, _, err = plg.OnDecodeHTTPData(ctx, []byte("<xml>data</xml>"))
	require.Error(t, err)
}

// --- checkAcceptedStatus: notFoundStatusCodes and no acceptStatusCodes non-200 ---

func TestOnDecodeHTTPData_WithAssignPostprocess(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
    number:
      selector:
        kind: xpath
        expr: //span[@class="num"]/text()
postprocess:
  assign:
    title: "${meta.title} [edited]"
  defaults:
    title_lang: ja
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><head><title>Orig</title></head><body><span class="num">ABC-123</span></body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, mv.Title, "[edited]")
}

// --- OnDecorateMediaRequest with cookies ---

func TestOnDecorateMediaRequest_WithCookies(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  cookies:
    session: "token123"
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	cookies := req.Cookies()
	assert.NotEmpty(t, cookies)
}

// --- followNextRequest with status rejected ---

func TestOnDecodeHTTPData_HTML_RequiredStringEmpty(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      required: true
      selector:
        kind: xpath
        expr: //h1[@class="missing"]/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, ok, err := plg.OnDecodeHTTPData(ctx, []byte(`<html><body><h1>Found</h1></body></html>`))
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- decodeJSON with required string field empty ---

func TestOnDecodeHTTPData_JSON_RequiredStringEmpty(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: json
  fields:
    title:
      required: true
      selector:
        kind: jsonpath
        expr: $.missing_field
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, ok, err := plg.OnDecodeHTTPData(ctx, []byte(`{"other": "value"}`))
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- multiRequest + workflow: handleResponse path ---

func TestOnHandleHTTPRequest_MultiRequestWithWorkflow2(t *testing.T) {
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
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
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
				Body:       nopCloser([]byte(`<html><body><div class="found">ok</div><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>DetailTitle</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
}

// --- evalTemplateExpr tests ---

func TestOnPrecheckRequest_NilPrecheck(t *testing.T) {
	plg := buildTestPlugin(t, simpleOneStepSpec("https://example.com"))
	ctx := pluginapi.InitContainer(context.Background())
	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestOnPrecheckRequest_PatternNoMatch2(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Precheck = &PrecheckSpec{NumberPatterns: []string{`^MATCH-\d+$`}}
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckRequest_PatternMatches(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Precheck = &PrecheckSpec{NumberPatterns: []string{`^ABC-\d+$`}}
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestOnPrecheckRequest_WithVariables(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Precheck = &PrecheckSpec{
		Variables: map[string]string{"slug": "${to_lower(${number})}"},
	}
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)
	val, found := pluginapi.GetContainerValue(ctx, ctxVarKey("slug"))
	assert.True(t, found)
	assert.Equal(t, "abc-123", val)
}

func TestOnMakeHTTPRequest_MultiRequestPlaceholder(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestOnPrecheckResponse_NotFoundDefault(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	plg := buildTestPlugin(t, spec)
	rsp := &http.Response{StatusCode: 404}
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, rsp)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckResponse_NotFoundStatusCodes(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.NotFoundStatusCodes = []int{302}
	plg := buildTestPlugin(t, spec)
	rsp := &http.Response{StatusCode: 302}
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, rsp)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckResponse_AcceptCodes(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.AcceptStatusCodes = []int{200, 301}
	plg := buildTestPlugin(t, spec)

	rsp := &http.Response{StatusCode: 200}
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, rsp)
	require.NoError(t, err)
	assert.True(t, ok)

	rsp = &http.Response{StatusCode: 500}
	_, err = plg.OnPrecheckResponse(context.Background(), nil, rsp)
	require.Error(t, err)
}

func TestOnDecodeHTTPData_HTMLSuccess(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	data := []byte(`<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "T", mv.Title)
}

func TestOnDecodeHTTPData_JSONSuccess(t *testing.T) {
	spec := jsonScrapeSpec("https://example.com")
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	data := []byte(`{"title":"T","actors":["A"]}`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "T", mv.Title)
}

func TestOnDecodeHTTPData_RequiredMissing(t *testing.T) {
	spec := simpleOneStepSpecRequired("https://example.com")
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	data := []byte(`<html><body></body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, mv)
}

func TestOnDecorateMediaRequest_Extended(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.Headers = map[string]string{"Referer": "${host}"}
	spec.Request.Cookies = map[string]string{"session": "abc"}
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, "https://example.com/page")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", req.Header.Get("Referer"))
}

func TestOnDecorateMediaRequest_NilFinalRequest(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.request = nil
	compiled.workflow = nil
	plg := &SearchPlugin{spec: compiled}

	ctx := pluginapi.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/img.jpg", nil)
	err = plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
}

func TestOnHandleHTTPRequest_SimplePassThrough(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("<html></html>"))),
			Header:     make(http.Header),
		}, nil
	}, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.NoError(t, err)
	assert.Equal(t, 200, rsp.StatusCode)
}

func TestOnPrecheckResponse_FinalReqNilNotFound(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.request = nil
	compiled.multiRequest = nil
	plg := &SearchPlugin{spec: compiled}
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, &http.Response{StatusCode: http.StatusNotFound})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckResponse_FinalReqNilOK(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.request = nil
	compiled.multiRequest = nil
	plg := &SearchPlugin{spec: compiled}
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, &http.Response{StatusCode: http.StatusOK})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestOnPrecheckResponse_NotFoundCustomCodes(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.NotFoundStatusCodes = []int{403, 404}
	plg := buildTestPlugin(t, spec)
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, &http.Response{StatusCode: 403})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckResponse_NoAcceptCodesDefaultOK(t *testing.T) {
	plg := buildTestPlugin(t, simpleOneStepSpec("https://example.com"))
	ok, err := plg.OnPrecheckResponse(context.Background(), nil, &http.Response{StatusCode: http.StatusOK})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestOnPrecheckResponse_AcceptCodesRejected500(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.AcceptStatusCodes = []int{200}
	plg := buildTestPlugin(t, spec)
	_, err := plg.OnPrecheckResponse(context.Background(), nil, &http.Response{StatusCode: 500})
	require.ErrorIs(t, err, errStatusCodeNotAccepted)
}

func TestOnDecodeHTTPData_UnsupportedFormat(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.scrape.format = "xml"
	plg := &SearchPlugin{spec: compiled}
	_, _, err = plg.OnDecodeHTTPData(context.Background(), []byte("<xml/>"))
	require.ErrorIs(t, err, errUnsupportedScrapeFormat)
}

func TestOnDecodeHTTPData_NilMeta(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Scrape.Fields["title"].Required = true
	plg := buildTestPlugin(t, spec)
	_, found, err := plg.OnDecodeHTTPData(context.Background(), []byte("<html><body></body></html>"))
	require.NoError(t, err)
	assert.False(t, found)
}

func TestOnMakeHTTPRequest_RequestNilAndMultiNil(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.request = nil
	compiled.multiRequest = nil
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	_, err = plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.ErrorIs(t, err, errRequestNil)
}

func TestOnDecorateMediaRequest_NilBaseReq(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.request = nil
	compiled.multiRequest = nil
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/img.jpg", nil)
	err = plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
}

func TestOnHandleHTTPRequest_WorkflowNilMultiRequestNil(t *testing.T) {
	plg := buildTestPlugin(t, simpleOneStepSpec("https://example.com"))
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.NoError(t, err)
	assert.Equal(t, 200, rsp.StatusCode)
}

func TestOnDecorateMediaRequest_HeaderAndCookieError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	spec := simpleOneStepSpec("https://example.com")
	compiled, cerr := compilePlugin(spec)
	require.NoError(t, cerr)
	compiled.request.headers = map[string]*template{"X-Bad": badTmpl}
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/img.jpg", nil)
	err = plg.OnDecorateMediaRequest(ctx, req)
	require.Error(t, err)
}

func TestOnDecorateMediaRequest_CookieRenderError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	spec := simpleOneStepSpec("https://example.com")
	compiled, cerr := compilePlugin(spec)
	require.NoError(t, cerr)
	compiled.request.cookies = map[string]*template{"sid": badTmpl}
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/img.jpg", nil)
	err = plg.OnDecorateMediaRequest(ctx, req)
	require.Error(t, err)
}
