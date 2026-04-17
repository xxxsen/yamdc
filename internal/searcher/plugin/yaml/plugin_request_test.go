package yaml

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/flarerr"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestBuildURL(t *testing.T) {
	assert.Equal(t, "https://example.com/path", buildURL("https://example.com", "/path"))
	assert.Contains(t, buildURL("://bad", "/path"), "/path")
}

// --- timeParse / softTimeParse ---

func TestBuildRequestBodyReader(t *testing.T) {
	ctx := &evalContext{number: "ABC-123"}

	jsonBody, _ := compileRequestBody(&RequestBodySpec{Kind: "json", Values: map[string]string{"a": "b"}})
	r, err := buildRequestBodyReader(&compiledRequest{body: jsonBody}, ctx)
	require.NoError(t, err)
	require.NotNil(t, r)

	rawBody, _ := compileRequestBody(&RequestBodySpec{Kind: "raw", Content: "raw data"})
	r, err = buildRequestBodyReader(&compiledRequest{body: rawBody}, ctx)
	require.NoError(t, err)
	require.NotNil(t, r)

	r, err = buildRequestBodyReader(&compiledRequest{body: nil}, ctx)
	require.NoError(t, err)
	require.Nil(t, r)

	rawNoContent, _ := compileRequestBody(&RequestBodySpec{Kind: "raw"})
	r, err = buildRequestBodyReader(&compiledRequest{body: rawNoContent}, ctx)
	require.NoError(t, err)
	require.Nil(t, r)
}

// --- previewBody ---

func TestSetBodyContentType(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	setBodyContentType(req, &compiledRequest{body: nil})
	assert.Empty(t, req.Header.Get("Content-Type"))

	formBody, _ := compileRequestBody(&RequestBodySpec{Kind: "form"})
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	setBodyContentType(req2, &compiledRequest{body: formBody})
	assert.Equal(t, "application/x-www-form-urlencoded", req2.Header.Get("Content-Type"))

	jsonBody, _ := compileRequestBody(&RequestBodySpec{Kind: "json"})
	req3, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	setBodyContentType(req3, &compiledRequest{body: jsonBody})
	assert.Equal(t, "application/json", req3.Header.Get("Content-Type"))

	req4, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	req4.Header.Set("Content-Type", "custom")
	setBodyContentType(req4, &compiledRequest{body: formBody})
	assert.Equal(t, "custom", req4.Header.Get("Content-Type"))
}

// --- CompileDraft ---

func TestBuildRequest_FormBody(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "POST",
			Path:   "/api",
			Body:   &RequestBodySpec{Kind: "form", Values: map[string]string{"q": "${number}"}},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
}

func TestBuildRequest_JSONBody(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "POST",
			Path:   "/api",
			Body:   &RequestBodySpec{Kind: "json", Values: map[string]string{"q": "${number}"}},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestBuildRequest_RawBody(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "POST",
			Path:   "/api",
			Body:   &RequestBodySpec{Kind: "raw", Content: "raw-data-${number}"},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.NotNil(t, req.Body)
}

func TestBuildRequest_WithURL(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "GET",
			URL:    "https://other.com/search/${number}",
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "https://other.com/search/ABC", req.URL.String())
}

// --- applyRequestParams ---

func TestApplyRequestParams(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	queryTmpl, _ := compileTemplate("val")
	headerTmpl, _ := compileTemplate("hval")
	cookieTmpl, _ := compileTemplate("cval")
	spec := &compiledRequest{
		query:   map[string]*template{"q": queryTmpl},
		headers: map[string]*template{"X-H": headerTmpl},
		cookies: map[string]*template{"c": cookieTmpl},
	}
	err := applyRequestParams(req, spec, &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "val", req.URL.Query().Get("q"))
	assert.Equal(t, "hval", req.Header.Get("X-H"))
	found := false
	for _, c := range req.Cookies() {
		if c.Name == "c" && c.Value == "cval" {
			found = true
		}
	}
	assert.True(t, found)
}

// --- applyFetchTypeContext ---

func TestApplyFetchTypeContext_GoHTTP(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	result := plg.applyFetchTypeContext(req, plg.spec.request)
	assert.Equal(t, req, result)
}

func TestApplyFetchTypeContext_Flaresolverr(t *testing.T) {
	yamlStr := `
version: 1
name: test-flare
type: one-step
fetch_type: flaresolverr
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
`
	plg := mustCompilePlugin(t, yamlStr)
	assert.Equal(t, fetchTypeFlaresolverr, plg.spec.fetchType)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.NotNil(t, req)

	bp := browser.GetParams(req.Context())
	assert.Nil(t, bp, "browser params should not be set for flaresolverr")

	fp := flarerr.GetParams(req.Context())
	assert.NotNil(t, fp, "flarerr params should be set for flaresolverr fetch_type")
}

func TestValidate_FlaresolverrFetchType(t *testing.T) {
	yamlStr := `
version: 1
name: test-flare
type: one-step
fetch_type: flaresolverr
hosts: ["https://example.com"]
request:
  method: GET
  path: /x
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	assert.Equal(t, fetchTypeFlaresolverr, plg.spec.fetchType)
}

func TestValidate_InvalidFetchType(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: unknown
hosts: ["https://example.com"]
request:
  method: GET
  path: /x
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	_, err := NewFromBytes([]byte(yamlStr))
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidFetchType)
}

func TestApplyBrowserContext_BrowserFetchType(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  browser:
    wait_selector: ".content"
    wait_timeout: 5
  headers:
    X-Custom: "val"
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
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.NotNil(t, req)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, ".content", params.WaitSelector)
	assert.Equal(t, 5*time.Second, params.WaitTimeout)
	assert.Equal(t, time.Duration(0), params.WaitStableDuration)
}

func TestApplyBrowserContext_WaitStable_Explicit(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  browser:
    wait_stable: 10
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
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, 10*time.Second, params.WaitStableDuration)
}

func TestApplyBrowserContext_WaitStable_DefaultWhenNoSelector(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
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
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, "", params.WaitSelector)
	assert.Equal(t, defaultWaitStable, params.WaitStableDuration)
}

func TestApplyBrowserContext_WaitStable_NotAppliedWithSelector(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  browser:
    wait_selector: "//div"
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
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, "//div", params.WaitSelector)
	assert.Equal(t, time.Duration(0), params.WaitStableDuration)
}

// --- template edge cases ---

func TestFinalRequest(t *testing.T) {
	reqSpec := &compiledRequest{method: "GET"}

	p1 := &compiledPlugin{request: reqSpec}
	assert.Equal(t, reqSpec, p1.finalRequest())

	nextReq := &compiledRequest{method: "POST"}
	p2 := &compiledPlugin{workflow: &compiledSearchSelectWorkflow{nextRequest: nextReq}}
	assert.Equal(t, nextReq, p2.finalRequest())

	mrReq := &compiledRequest{method: "PUT"}
	p3 := &compiledPlugin{multiRequest: &compiledMultiRequest{request: mrReq}}
	assert.Equal(t, mrReq, p3.finalRequest())
}

// --- jsonpath edge cases ---

func TestResolveRequestURL(t *testing.T) {
	pathTmpl, _ := compileTemplate("/search/${number}")
	spec := &compiledRequest{path: pathTmpl}
	u, err := resolveRequestURL(spec, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Contains(t, u, "/search/ABC")

	urlTmpl, _ := compileTemplate("https://other.com/${number}")
	spec2 := &compiledRequest{rawURL: urlTmpl}
	u2, err := resolveRequestURL(spec2, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.Equal(t, "https://other.com/ABC", u2)
}

// --- OnDecodeHTTPData with unsupported charset ---

func TestRenderFormBody(t *testing.T) {
	tmpl, _ := compileTemplate("${number}")
	body := &compiledRequestBody{kind: "form", values: map[string]*template{"q": tmpl}}
	reader, err := renderFormBody(body, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestRenderJSONBody(t *testing.T) {
	tmpl, _ := compileTemplate("${number}")
	body := &compiledRequestBody{kind: "json", values: map[string]*template{"q": tmpl}}
	reader, err := renderJSONBody(body, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestRenderRawBody(t *testing.T) {
	tmpl, _ := compileTemplate("data-${number}")
	body := &compiledRequestBody{kind: "raw", content: tmpl}
	reader, err := renderRawBody(body, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.NotNil(t, reader)

	body2 := &compiledRequestBody{kind: "raw", content: nil}
	reader2, err := renderRawBody(body2, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, reader2)
}

// --- buildRequestBodyReader ---

func TestBuildRequestBodyReader_NilBody(t *testing.T) {
	spec := &compiledRequest{}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, reader)
}

// --- condition evalTwoStringCondition all branches ---

func TestBuildRequest_EmptyHostFallback(t *testing.T) {
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
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	evalCtx := &evalContext{number: "ABC-123"}
	req, err := plg.buildRequest(ctx, plg.spec.request, evalCtx)
	require.NoError(t, err)
	assert.Contains(t, req.URL.String(), "example.com")
}

// --- buildRequest: with body (form) ---

func TestBuildRequest_WithFormBody(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: POST
  path: /search
  body:
    kind: form
    values:
      q: "${number}"
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
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
}

// --- buildRequest: with query params ---

func TestBuildRequest_WithQueryParams(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search
  query:
    q: "${number}"
    page: "1"
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
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Contains(t, req.URL.RawQuery, "q=ABC-123")
}

// --- buildRequest: with headers and cookies ---

func TestBuildRequest_WithHeadersAndCookies(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  headers:
    X-Custom: "test-val"
  cookies:
    session: "abc"
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
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Equal(t, "test-val", req.Header.Get("X-Custom"))
}

// --- compilePlugin: all branches ---

func TestResolveRequestURL_RawURL(t *testing.T) {
	tmpl, err := compileTemplate("https://example.com/search/${number}")
	require.NoError(t, err)
	spec := &compiledRequest{rawURL: tmpl}
	url, err := resolveRequestURL(spec, &evalContext{number: "ABC-123"})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/search/ABC-123", url)
}

func TestResolveRequestURL_PathBased(t *testing.T) {
	tmpl, err := compileTemplate("/search/${number}")
	require.NoError(t, err)
	spec := &compiledRequest{path: tmpl}
	url, err := resolveRequestURL(spec, &evalContext{number: "ABC-123", host: "https://example.com"})
	require.NoError(t, err)
	assert.Contains(t, url, "search/ABC-123")
}

func TestBuildRequestBodyReader_NilBodySpec(t *testing.T) {
	spec := &compiledRequest{}
	body, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, body)
}

func TestBuildRequestBodyReader_FormKind(t *testing.T) {
	tmpl, err := compileTemplate("val")
	require.NoError(t, err)
	spec := &compiledRequest{
		body: &compiledRequestBody{
			kind:   bodyKindForm,
			values: map[string]*template{"key": tmpl},
		},
	}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	require.NotNil(t, reader)
	data, _ := io.ReadAll(reader)
	assert.Contains(t, string(data), "key=val")
}

func TestBuildRequestBodyReader_JSONKind(t *testing.T) {
	tmpl, err := compileTemplate("value")
	require.NoError(t, err)
	spec := &compiledRequest{
		body: &compiledRequestBody{
			kind:   bodyKindJSON,
			values: map[string]*template{"field": tmpl},
		},
	}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	require.NotNil(t, reader)
	data, _ := io.ReadAll(reader)
	assert.Contains(t, string(data), "field")
}

func TestBuildRequestBodyReader_RawKind(t *testing.T) {
	tmpl, err := compileTemplate("raw content")
	require.NoError(t, err)
	spec := &compiledRequest{
		body: &compiledRequestBody{
			kind:    bodyKindRaw,
			content: tmpl,
		},
	}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	require.NotNil(t, reader)
	data, _ := io.ReadAll(reader)
	assert.Equal(t, "raw content", string(data))
}

func TestBuildRequestBodyReader_RawNilContent(t *testing.T) {
	spec := &compiledRequest{
		body: &compiledRequestBody{kind: bodyKindRaw},
	}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, reader)
}

func TestBuildRequestBodyReader_UnknownKind(t *testing.T) {
	spec := &compiledRequest{
		body: &compiledRequestBody{kind: "unknown"},
	}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, reader)
}

func TestRenderFormBody_Success(t *testing.T) {
	tmpl, err := compileTemplate("v1")
	require.NoError(t, err)
	body := &compiledRequestBody{values: map[string]*template{"k1": tmpl}}
	reader, err := renderFormBody(body, &evalContext{})
	require.NoError(t, err)
	data, _ := io.ReadAll(reader)
	assert.Contains(t, string(data), "k1=v1")
}

func TestRenderJSONBody_Success(t *testing.T) {
	tmpl, err := compileTemplate("v1")
	require.NoError(t, err)
	body := &compiledRequestBody{values: map[string]*template{"k1": tmpl}}
	reader, err := renderJSONBody(body, &evalContext{})
	require.NoError(t, err)
	data, _ := io.ReadAll(reader)
	assert.Contains(t, string(data), `"k1"`)
}

func TestRenderRawBody_WithTemplate(t *testing.T) {
	tmpl, err := compileTemplate("raw ${number}")
	require.NoError(t, err)
	body := &compiledRequestBody{content: tmpl}
	reader, err := renderRawBody(body, &evalContext{number: "123"})
	require.NoError(t, err)
	data, _ := io.ReadAll(reader)
	assert.Equal(t, "raw 123", string(data))
}

func TestApplyRequestParams_AllTypes(t *testing.T) {
	tmpl, err := compileTemplate("qval")
	require.NoError(t, err)
	hdrTmpl, err := compileTemplate("hval")
	require.NoError(t, err)
	cookieTmpl, err := compileTemplate("cval")
	require.NoError(t, err)
	spec := &compiledRequest{
		query:   map[string]*template{"q": tmpl},
		headers: map[string]*template{"X-Custom": hdrTmpl},
		cookies: map[string]*template{"session": cookieTmpl},
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	err = applyRequestParams(req, spec, &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "qval", req.URL.Query().Get("q"))
	assert.Equal(t, "hval", req.Header.Get("X-Custom"))
}

func TestBuildURL_InvalidHost(t *testing.T) {
	result := buildURL("://bad", "/path")
	assert.Equal(t, "://bad/path", result)
}

func TestBuildURL_InvalidPath(t *testing.T) {
	result := buildURL("https://example.com", "://bad-path")
	assert.NotEmpty(t, result)
}

func TestSetBodyContentType_Extended(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected string
	}{
		{"form", bodyKindForm, "application/x-www-form-urlencoded"},
		{"json", bodyKindJSON, "application/json"},
		{"raw", bodyKindRaw, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
			spec := &compiledRequest{body: &compiledRequestBody{kind: tt.kind}}
			setBodyContentType(req, spec)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, req.Header.Get("Content-Type"))
			}
		})
	}
}

func TestSetBodyContentType_NilBody(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	spec := &compiledRequest{}
	setBodyContentType(req, spec)
	assert.Empty(t, req.Header.Get("Content-Type"))
}

func TestRenderFormBody_Error(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	body := &compiledRequestBody{kind: bodyKindForm, values: map[string]*template{"k": badTmpl}}
	_, err = renderFormBody(body, &evalContext{})
	require.Error(t, err)
}

func TestRenderJSONBody_Error(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	body := &compiledRequestBody{kind: bodyKindJSON, values: map[string]*template{"k": badTmpl}}
	_, err = renderJSONBody(body, &evalContext{})
	require.Error(t, err)
}

func TestRenderRawBody_Error(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	body := &compiledRequestBody{kind: bodyKindRaw, content: badTmpl}
	_, err = renderRawBody(body, &evalContext{})
	require.Error(t, err)
}

func TestApplyRequestParams_QueryError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	spec := &compiledRequest{query: map[string]*template{"k": badTmpl}}
	err = applyRequestParams(req, spec, &evalContext{})
	require.Error(t, err)
}

func TestApplyRequestParams_HeaderError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	spec := &compiledRequest{headers: map[string]*template{"k": badTmpl}}
	err = applyRequestParams(req, spec, &evalContext{})
	require.Error(t, err)
}

func TestApplyRequestParams_CookieError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	spec := &compiledRequest{cookies: map[string]*template{"k": badTmpl}}
	err = applyRequestParams(req, spec, &evalContext{})
	require.Error(t, err)
}

func TestResolveRequestURL_PathRenderError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	spec := &compiledRequest{path: badTmpl}
	_, err = resolveRequestURL(spec, &evalContext{})
	require.Error(t, err)
}

func TestResolveRequestURL_RawURLRenderError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	spec := &compiledRequest{rawURL: badTmpl}
	_, err = resolveRequestURL(spec, &evalContext{})
	require.Error(t, err)
}

func TestBuildRequest_BodyBuildError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	plg := buildTestPlugin(t, simpleOneStepSpec("https://example.com"))
	spec := &compiledRequest{
		method: http.MethodPost,
		path:   mustCompileTemplate(t, "/test"),
		body:   &compiledRequestBody{kind: bodyKindRaw, content: badTmpl},
	}
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	_, err = plg.buildRequest(ctx, spec, &evalContext{host: "https://example.com"})
	require.Error(t, err)
}

func TestBuildRequest_ApplyParamsError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	plg := buildTestPlugin(t, simpleOneStepSpec("https://example.com"))
	spec := &compiledRequest{
		method: http.MethodGet,
		path:   mustCompileTemplate(t, "/test"),
		query:  map[string]*template{"k": badTmpl},
	}
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	_, err = plg.buildRequest(ctx, spec, &evalContext{host: "https://example.com"})
	require.Error(t, err)
}
