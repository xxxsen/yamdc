package yaml

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"golang.org/x/net/html"
)

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
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
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

func TestDecodeBytes_AllCharsets(t *testing.T) {
	tests := []struct {
		name    string
		charset string
		wantErr bool
	}{
		{"empty", "", false},
		{"utf8", "utf-8", false},
		{"utf8_no_hyphen", "utf8", false},
		{"euc_jp", "euc-jp", false},
		{"unsupported", "windows-1252", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeBytes([]byte("hello"), tt.charset)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNormalizeLang_Extended(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"", ""},
		{"ja", "ja"},
		{"en", "en"},
		{"zh-cn", "zh-cn"},
		{"zh-tw", "zh-tw"},
		{"fr", "fr"},
		{"  JA  ", "ja"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeLang(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestCompileRequest_WithURL(t *testing.T) {
	spec := &RequestSpec{Method: "GET", URL: "https://example.com/${number}"}
	req, err := compileRequest(spec)
	require.NoError(t, err)
	assert.NotNil(t, req.rawURL)
	assert.Nil(t, req.path)
}

func TestCompileRequest_WithBrowser(t *testing.T) {
	spec := &RequestSpec{
		Method:  "GET",
		Path:    "/",
		Browser: &BrowserSpec{WaitSelector: ".loaded", WaitTimeout: 5},
	}
	req, err := compileRequest(spec)
	require.NoError(t, err)
	assert.NotNil(t, req.browser)
}

func TestCompileRequest_WithResponse(t *testing.T) {
	spec := &RequestSpec{
		Method:   "GET",
		Path:     "/",
		Response: &ResponseSpec{DecodeCharset: "euc-jp"},
	}
	req, err := compileRequest(spec)
	require.NoError(t, err)
	assert.Equal(t, "euc-jp", req.decodeCharset)
}

func TestValidatePluginSpec_Extended(t *testing.T) {
	tests := []struct {
		name    string
		spec    *PluginSpec
		wantErr bool
	}{
		{"bad_version", &PluginSpec{Version: 2, Name: "x", Type: "one-step", Hosts: []string{"h"}, Request: &RequestSpec{}}, true},
		{"empty_name", &PluginSpec{Version: 1, Name: "", Type: "one-step", Hosts: []string{"h"}, Request: &RequestSpec{}}, true},
		{"bad_type", &PluginSpec{Version: 1, Name: "x", Type: "bad", Hosts: []string{"h"}, Request: &RequestSpec{}}, true},
		{"invalid_fetch_type", &PluginSpec{Version: 1, Name: "x", Type: "one-step", Hosts: []string{"h"}, Request: &RequestSpec{}, FetchType: "curl"}, true},
		{"valid_browser_fetch_type", &PluginSpec{Version: 1, Name: "x", Type: "one-step", Hosts: []string{"h"}, Request: &RequestSpec{Method: "GET", Path: "/"}, FetchType: "browser"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePluginSpec(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompileScrape_Extended(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ScrapeSpec
		wantErr bool
	}{
		{"nil", nil, true},
		{"bad_format", &ScrapeSpec{Format: "xml"}, true},
		{"no_fields", &ScrapeSpec{Format: "html"}, true},
		{"no_selector", &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {}}}, true},
		{"html_wrong_selector_kind", &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.t"}}}}, true},
		{"json_wrong_selector_kind", &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t"}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileScrape(tt.spec)
			require.Error(t, err)
		})
	}
}

func TestCompileMultiRequest_Extended(t *testing.T) {
	tests := []struct {
		name string
		spec *MultiRequestSpec
	}{
		{"no_candidates", &MultiRequestSpec{Request: &RequestSpec{Method: "GET", Path: "/"}}},
		{"no_request", &MultiRequestSpec{Candidates: []string{"a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileMultiRequest(tt.spec)
			require.Error(t, err)
		})
	}
}

func TestCompileSearchSelect_Extended(t *testing.T) {
	tests := []struct {
		name string
		spec *SearchSelectWorkflowSpec
	}{
		{"no_selectors", &SearchSelectWorkflowSpec{NextRequest: &RequestSpec{Method: "GET", Path: "/"}}},
		{"no_next_request", &SearchSelectWorkflowSpec{Selectors: []*SelectorListSpec{{Name: "l", Kind: "xpath", Expr: "//a"}}}},
		{"unsupported_selector_kind", &SearchSelectWorkflowSpec{
			Selectors:   []*SelectorListSpec{{Name: "l", Kind: "css", Expr: "a"}},
			NextRequest: &RequestSpec{Method: "GET", Path: "/"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileSearchSelect(tt.spec)
			require.Error(t, err)
		})
	}
}

func TestCompileWorkflow_NilSpec(t *testing.T) {
	result, err := compileWorkflow(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCompileWorkflow_EmptySpec(t *testing.T) {
	result, err := compileWorkflow(&WorkflowSpec{})
	require.NoError(t, err)
	assert.Nil(t, result)
}

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
	rsp, err := plg.OnHandleHTTPRequest(ctx, func(_ context.Context, r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("<html></html>"))),
			Header:     make(http.Header),
		}, nil
	}, req)
	require.NoError(t, err)
	assert.Equal(t, 200, rsp.StatusCode)
}

func TestCheckAcceptedStatus_Extended(t *testing.T) {
	tests := []struct {
		name          string
		acceptCodes   []int
		notFoundCodes []int
		code          int
		wantErr       bool
	}{
		{"ok_default", nil, nil, 200, false},
		{"not_ok_default", nil, nil, 500, true},
		{"not_found_code", nil, []int{302}, 302, true},
		{"accept_ok", []int{200, 301}, nil, 200, false},
		{"accept_not_ok", []int{200}, nil, 500, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &compiledRequest{
				acceptStatusCodes:   tt.acceptCodes,
				notFoundStatusCodes: tt.notFoundCodes,
			}
			err := checkAcceptedStatus(spec, tt.code)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
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
			req, _ := http.NewRequest(http.MethodPost, "http://example.com/", nil)
			spec := &compiledRequest{body: &compiledRequestBody{kind: tt.kind}}
			setBodyContentType(req, spec)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, req.Header.Get("Content-Type"))
			}
		})
	}
}

func TestSetBodyContentType_NilBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	spec := &compiledRequest{}
	setBodyContentType(req, spec)
	assert.Empty(t, req.Header.Get("Content-Type"))
}

func TestAssignListField_Unsupported(t *testing.T) {
	err := assignListField(context.Background(), nil, "actors", []string{"a"}, ParserSpec{Kind: "custom"})
	require.ErrorIs(t, err, errUnsupportedListParser)
}

func TestAssignStringField_Unsupported(t *testing.T) {
	err := assignStringField(context.Background(), nil, "title", "t", ParserSpec{Kind: "bogus"})
	require.ErrorIs(t, err, errUnsupportedParser)
}

func TestApplyPostprocess_SwitchConfig(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Postprocess = &PostprocessSpec{
		SwitchConfig: &SwitchConfigSpec{
			DisableReleaseDateCheck: true,
			DisableNumberReplace:    true,
		},
	}
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	data := []byte(`<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, mv.SwithConfig.DisableReleaseDateCheck)
	assert.True(t, mv.SwithConfig.DisableNumberReplace)
}

func TestParseDurationMMSS_Extended(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"1:30", 90},
		{"0:00", 0},
		{"invalid", 0},
		{"a:b", 0},
		{"10:x", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expect, parseDurationMMSS(tt.input))
		})
	}
}

func TestNewFromBytes_InvalidYAML2(t *testing.T) {
	_, err := NewFromBytes([]byte("{{invalid"))
	require.Error(t, err)
}

func TestNewFromBytes_InvalidSpec(t *testing.T) {
	data := []byte(`version: 1
name: test
type: bad-type
hosts: ["h"]
request:
  method: GET
  path: /
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: "//t"`)
	_, err := NewFromBytes(data)
	require.Error(t, err)
}

func TestCompilePrecheck_NilSpec(t *testing.T) {
	result, err := compilePrecheck(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCompilePostprocess_NilSpec(t *testing.T) {
	result, err := compilePostprocess(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMovieMetaStringMap_Full(t *testing.T) {
	mv := &model.MovieMeta{
		Number: "N", Title: "T", Plot: "P", Studio: "S", Label: "L", Series: "SE",
		Cover: &model.File{Name: "c.jpg"}, Poster: &model.File{Name: "p.jpg"},
	}
	m := movieMetaStringMap(mv)
	assert.Equal(t, "T", m["title"])
	assert.Equal(t, "c.jpg", m["cover"])
	assert.Equal(t, "p.jpg", m["poster"])
}

func TestMovieMetaStringMap_NilCoverPoster(t *testing.T) {
	mv := &model.MovieMeta{Number: "N"}
	m := movieMetaStringMap(mv)
	_, hasCover := m["cover"]
	assert.False(t, hasCover)
}

func buildTestPlugin(t *testing.T, spec *PluginSpec) *SearchPlugin {
	t.Helper()
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	return &SearchPlugin{spec: compiled}
}

func TestCompileTemplateMap_Error(t *testing.T) {
	_, err := compileTemplateMap(map[string]string{"k": "${bad_fn()}"})
	require.Error(t, err)
}

func TestCompileRequest_BodyError(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", Path: "/", Body: &RequestBodySpec{Kind: "invalid_kind"}})
	require.Error(t, err)
}

func TestCompileRequest_QueryError(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", Path: "/", Query: map[string]string{"k": "${bad_fn()}"}})
	require.Error(t, err)
}

func TestCompileRequest_HeadersError(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", Path: "/", Headers: map[string]string{"k": "${bad_fn()}"}})
	require.Error(t, err)
}

func TestCompileRequest_CookiesError(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", Path: "/", Cookies: map[string]string{"k": "${bad_fn()}"}})
	require.Error(t, err)
}

func TestCompileRequest_PathTemplateError(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", Path: "${bad_fn()}"})
	require.Error(t, err)
}

func TestCompileRequest_URLTemplateError(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", URL: "${bad_fn()}"})
	require.Error(t, err)
}

func TestCompileRequest_UnsupportedMethod(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "DELETE", Path: "/"})
	require.ErrorIs(t, err, errUnsupportedRequestMethod)
}

func TestCompileRequest_PathAndURLExclusive(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET", Path: "/a", URL: "http://b"})
	require.ErrorIs(t, err, errRequestPathAndURLExclusive)
}

func TestCompileRequest_MethodRequired(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Path: "/"})
	require.ErrorIs(t, err, errRequestMethodRequired)
}

func TestCompileRequest_PathOrURLRequired(t *testing.T) {
	_, err := compileRequest(&RequestSpec{Method: "GET"})
	require.ErrorIs(t, err, errRequestPathOrURLRequired)
}

func TestCompileRequest_NilSpec(t *testing.T) {
	result, err := compileRequest(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCompileSearchSelect_MissingSelectors(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{})
	require.ErrorIs(t, err, errSearchSelectRequiresSelector)
}

func TestCompileSearchSelect_MissingNextRequest(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors: []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
	})
	require.ErrorIs(t, err, errSearchSelectNextRequestRequired)
}

func TestCompileSearchSelect_InvalidNextRequest(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors: []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
		NextRequest: &RequestSpec{Method: "DELETE", Path: "/"},
	})
	require.Error(t, err)
}

func TestCompileSearchSelect_InvalidMatchCondition(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
		NextRequest: &RequestSpec{Method: "GET", Path: "/detail"},
		Match:       &ConditionGroupSpec{Conditions: []string{"bad_cond"}},
	})
	require.Error(t, err)
}

func TestCompileSearchSelect_InvalidItemVarTemplate(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:     []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
		NextRequest:   &RequestSpec{Method: "GET", Path: "/detail"},
		ItemVariables: map[string]string{"k": "${bad_fn()}"},
	})
	require.Error(t, err)
}

func TestCompileSearchSelect_UnsupportedSelectorKind(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "css", Expr: ".link"}},
		NextRequest: &RequestSpec{Method: "GET", Path: "/detail"},
	})
	require.ErrorIs(t, err, errUnsupportedSelectorKind)
}

func TestCompileSearchSelect_InvalidReturnTemplate(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
		NextRequest: &RequestSpec{Method: "GET", Path: "/detail"},
		Return:      "${bad_fn()}",
	})
	require.Error(t, err)
}

func TestCompileMultiRequest_RequestError(t *testing.T) {
	_, err := compileMultiRequest(&MultiRequestSpec{
		Candidates: []string{"c1"},
		Request:    &RequestSpec{Method: "DELETE", Path: "/"},
	})
	require.Error(t, err)
}

func TestCompileMultiRequest_ConditionError(t *testing.T) {
	_, err := compileMultiRequest(&MultiRequestSpec{
		Candidates:  []string{"c1"},
		Request:     &RequestSpec{Method: "GET", Path: "/search"},
		SuccessWhen: &ConditionGroupSpec{Conditions: []string{"bad_cond"}},
	})
	require.Error(t, err)
}

func TestCompileMultiRequest_CandidateTemplateError(t *testing.T) {
	_, err := compileMultiRequest(&MultiRequestSpec{
		Candidates: []string{"${bad_fn()}"},
		Request:    &RequestSpec{Method: "GET", Path: "/search"},
	})
	require.Error(t, err)
}

func TestCompileRequestBody_ValuesError(t *testing.T) {
	_, err := compileRequestBody(&RequestBodySpec{Kind: "form", Values: map[string]string{"k": "${bad_fn()}"}})
	require.Error(t, err)
}

func TestCompileRequestBody_ContentError(t *testing.T) {
	_, err := compileRequestBody(&RequestBodySpec{Kind: "raw", Content: "${bad_fn()}"})
	require.Error(t, err)
}

func TestCompilePostprocess_AssignError(t *testing.T) {
	_, err := compilePostprocess(&PostprocessSpec{Assign: map[string]string{"k": "${bad_fn()}"}})
	require.Error(t, err)
}

func TestCompilePrecheck_VariableError(t *testing.T) {
	_, err := compilePrecheck(&PrecheckSpec{Variables: map[string]string{"k": "${bad_fn()}"}})
	require.Error(t, err)
}

func TestCompileScrape_FieldSelectorRequired(t *testing.T) {
	_, err := compileScrape(&ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {}}})
	require.Error(t, err)
}

func TestCompileScrape_HTMLSelectorKindError(t *testing.T) {
	_, err := compileScrape(&ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
		"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
	}})
	require.Error(t, err)
}

func TestCompileScrape_JSONSelectorKindError(t *testing.T) {
	_, err := compileScrape(&ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
		"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title"}},
	}})
	require.Error(t, err)
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
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	spec := &compiledRequest{query: map[string]*template{"k": badTmpl}}
	err = applyRequestParams(req, spec, &evalContext{})
	require.Error(t, err)
}

func TestApplyRequestParams_HeaderError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	spec := &compiledRequest{headers: map[string]*template{"k": badTmpl}}
	err = applyRequestParams(req, spec, &evalContext{})
	require.Error(t, err)
}

func TestApplyRequestParams_CookieError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
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
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/img.jpg", nil)
	err = plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
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

func mustCompileTemplate(t *testing.T, raw string) *template {
	t.Helper()
	tmpl, err := compileTemplate(raw)
	require.NoError(t, err)
	return tmpl
}

func TestDecodeHTML_RequiredListEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"actors": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//span[@class='actor']/text()"}, Required: true},
		}},
	}
	plg := buildTestPlugin(t, spec)
	node := helperParseHTMLStr(t, `<html><body><p>no actors here</p></body></html>`)
	mv, err := plg.decodeHTML(context.Background(), node)
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}},
	}
	plg := buildTestPlugin(t, spec)
	_, err := plg.decodeJSON(context.Background(), []byte(`not-json`))
	require.Error(t, err)
}

func TestDecodeJSON_RequiredListEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"actors": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors"}, Required: true},
		}},
	}
	plg := buildTestPlugin(t, spec)
	mv, err := plg.decodeJSON(context.Background(), []byte(`{"title":"T"}`))
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestDecodeJSON_RequiredStringEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}, Required: true},
		}},
	}
	plg := buildTestPlugin(t, spec)
	mv, err := plg.decodeJSON(context.Background(), []byte(`{"other":"value"}`))
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestOnHandleHTTPRequest_WorkflowNilMultiRequestNil(t *testing.T) {
	plg := buildTestPlugin(t, simpleOneStepSpec("https://example.com"))
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	invoker := func(ctx context.Context, r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
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
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/img.jpg", nil)
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
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/img.jpg", nil)
	err = plg.OnDecorateMediaRequest(ctx, req)
	require.Error(t, err)
}

func TestValidatePluginSpec_RequestAndMultiExclusive(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.MultiRequest = &MultiRequestSpec{
		Candidates: []string{"a"},
		Request:    &RequestSpec{Method: "GET", Path: "/"},
	}
	_, err := validatePluginSpec(spec)
	require.ErrorIs(t, err, errRequestAndMultiRequestExclusive)
}

func TestValidatePluginSpec_TwoStepRequiresSearchSelect(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Type = "two-step"
	_, err := validatePluginSpec(spec)
	require.ErrorIs(t, err, errTwoStepRequiresSearchSelect)
}

func TestValidatePluginSpec_InvalidFetchType(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.FetchType = "puppeteer"
	_, err := validatePluginSpec(spec)
	require.Error(t, err)
}

func helperParseHTMLStr(t *testing.T, s string) *html.Node {
	t.Helper()
	n, err := htmlquery.Parse(strings.NewReader(s))
	require.NoError(t, err)
	return n
}
