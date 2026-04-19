package yaml

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestValidatePluginSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *PluginSpec
		wantErr bool
	}{
		{name: "invalid_version", spec: &PluginSpec{Version: 2}, wantErr: true},
		{name: "empty_name", spec: &PluginSpec{Version: 1}, wantErr: true},
		{name: "invalid_type", spec: &PluginSpec{Version: 1, Name: "test", Type: "bad"}, wantErr: true},
		{name: "no_hosts", spec: &PluginSpec{Version: 1, Name: "test", Type: "one-step"}, wantErr: true},
		{name: "both_request_and_multi", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "one-step", Hosts: []string{"h"},
			Request:      &RequestSpec{Method: "GET", Path: "/"},
			MultiRequest: &MultiRequestSpec{},
		}, wantErr: true},
		{name: "two_step_no_workflow", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "two-step", Hosts: []string{"h"},
			Request: &RequestSpec{Method: "GET", Path: "/"},
		}, wantErr: true},
		{name: "no_request_or_multi", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "one-step", Hosts: []string{"h"},
		}, wantErr: true},
		{name: "invalid_fetch_type", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "one-step", Hosts: []string{"h"},
			FetchType: "bad", Request: &RequestSpec{Method: "GET", Path: "/"},
		}, wantErr: true},
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

func TestCompileRequest_Errors(t *testing.T) {
	tests := []struct {
		name string
		spec *RequestSpec
	}{
		{name: "no_method", spec: &RequestSpec{Path: "/"}},
		{name: "no_path_or_url", spec: &RequestSpec{Method: "GET"}},
		{name: "both_path_and_url", spec: &RequestSpec{Method: "GET", Path: "/", URL: "http://x"}},
		{name: "unsupported_method", spec: &RequestSpec{Method: "DELETE", Path: "/"}},
		{name: "unsupported_body_kind", spec: &RequestSpec{Method: "POST", Path: "/", Body: &RequestBodySpec{Kind: "xml"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileRequest(tt.spec)
			require.Error(t, err)
		})
	}
}

func TestCompileRequest_Nil(t *testing.T) {
	r, err := compileRequest(nil)
	require.NoError(t, err)
	require.Nil(t, r)
}

func TestCompileRequest_Full(t *testing.T) {
	spec := &RequestSpec{
		Method:              "POST",
		Path:                "/api",
		Query:               map[string]string{"q": "val"},
		Headers:             map[string]string{"X-Key": "v"},
		Cookies:             map[string]string{"session": "abc"},
		Body:                &RequestBodySpec{Kind: "json", Values: map[string]string{"a": "b"}},
		Response:            &ResponseSpec{DecodeCharset: "utf-8"},
		Browser:             &BrowserSpec{WaitSelector: "div", WaitTimeout: 10},
		AcceptStatusCodes:   []int{200, 201},
		NotFoundStatusCodes: []int{404},
	}
	r, err := compileRequest(spec)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "POST", r.method)
	assert.NotNil(t, r.body)
	assert.NotNil(t, r.browser)
}

func TestCompileRequestBody(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		wantErr bool
	}{
		{name: "form", kind: "form"},
		{name: "json", kind: "json"},
		{name: "raw", kind: "raw"},
		{name: "invalid", kind: "xml", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileRequestBody(&RequestBodySpec{Kind: tt.kind})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompileRequestBody_WithContent(t *testing.T) {
	b, err := compileRequestBody(&RequestBodySpec{Kind: "raw", Content: "data"})
	require.NoError(t, err)
	require.NotNil(t, b.content)
}

func TestCompileMultiRequest(t *testing.T) {
	tests := []struct {
		name    string
		spec    *MultiRequestSpec
		wantErr bool
	}{
		{name: "no_candidates", spec: &MultiRequestSpec{Request: &RequestSpec{Method: "GET", Path: "/"}}, wantErr: true},
		{name: "no_request", spec: &MultiRequestSpec{Candidates: []string{"a"}}, wantErr: true},
		{name: "valid", spec: &MultiRequestSpec{
			Candidates: []string{"a"},
			Request:    &RequestSpec{Method: "GET", Path: "/"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileMultiRequest(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompileScrape(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ScrapeSpec
		wantErr bool
	}{
		{name: "nil", spec: nil, wantErr: true},
		{name: "bad_format", spec: &ScrapeSpec{Format: "xml"}, wantErr: true},
		{name: "no_fields", spec: &ScrapeSpec{Format: "html"}, wantErr: true},
		{name: "field_no_selector", spec: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {},
		}}, wantErr: true},
		{name: "html_non_xpath", spec: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}}, wantErr: true},
		{name: "json_non_jsonpath", spec: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title"}},
		}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileScrape(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompilePostprocess(t *testing.T) {
	p, err := compilePostprocess(nil)
	require.NoError(t, err)
	require.Nil(t, p)

	p, err = compilePostprocess(&PostprocessSpec{
		Assign: map[string]string{"number": "${number}"},
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestCompilePrecheck(t *testing.T) {
	p, err := compilePrecheck(nil)
	require.NoError(t, err)
	require.Nil(t, p)

	p, err = compilePrecheck(&PrecheckSpec{
		NumberPatterns: []string{`^ABC`},
		Variables:      map[string]string{"x": "${number}"},
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Len(t, p.numberPatterns, 1)
}

// --- applyStringTransforms ---

func TestNewFromBytes_InvalidYAML(t *testing.T) {
	_, err := NewFromBytes([]byte(":::invalid"))
	require.Error(t, err)
}

func TestNewFromBytes_Valid(t *testing.T) {
	plg, err := NewFromBytes([]byte(minimalOneStepYAML()))
	require.NoError(t, err)
	require.NotNil(t, plg)
}

// --- OnGetHosts ---

func TestCompileWorkflow_Exhaustive(t *testing.T) {
	_, err := compileWorkflow(nil)
	assert.NoError(t, err)

	_, err = compileWorkflow(&WorkflowSpec{})
	assert.NoError(t, err)

	_, err = compileWorkflow(&WorkflowSpec{SearchSelect: &SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a/@href"}},
		Return:      "${item.link}",
		NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
	}})
	assert.NoError(t, err)
}

func TestCompileSearchSelect_Exhaustive(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{})
	assert.ErrorIs(t, err, errSearchSelectRequiresSelector)

	_, err = compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors: []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
	})
	assert.ErrorIs(t, err, errSearchSelectNextRequestRequired)

	_, err = compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "jsonpath", Expr: "$.a"}},
		NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
	})
	assert.ErrorIs(t, err, errUnsupportedSelectorKind)

	result, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:     []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
		Return:        "${item.link}",
		NextRequest:   &RequestSpec{Method: "GET", Path: "/${value}"},
		ItemVariables: map[string]string{"slug": "${item.link}"},
		Match:         &ConditionGroupSpec{Mode: "and", Conditions: []string{`contains("${item.link}", "video")`}},
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.itemVariables, 1)
}

// --- NEW: OnPrecheckRequest_WithPattern ---

func TestCompilePlugin_FullTwoStep(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "full",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Precheck: &PrecheckSpec{
			NumberPatterns: []string{`.*`},
			Variables:      map[string]string{"slug": "${number}"},
		},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{SearchSelect: &SearchSelectWorkflowSpec{
			Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a/@href"}},
			Return:      "${item.link}",
			NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
		}},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}},
		}},
		Postprocess: &PostprocessSpec{Assign: map[string]string{"title": "${meta.title}"}},
	}
	_, err := compilePlugin(spec)
	assert.NoError(t, err)
}

func TestCompilePlugin_WithMultiRequest(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "mr",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates:  []string{"${number}"},
			Request:     &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{Mode: "and", Conditions: []string{`selector_exists(xpath("//div"))`}},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}},
		}},
	}
	_, err := compilePlugin(spec)
	assert.NoError(t, err)
}

// --- OnDecodeHTTPData with HTML list fields ---

func TestCompilePlugin_ErrorInScrape(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInRequest(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInMultiRequest(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Request: &RequestSpec{Method: "GET", Path: "/"},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInWorkflow(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:    []string{"https://example.com"},
		Request:  &RequestSpec{Method: "GET", Path: "/"},
		Workflow: &WorkflowSpec{SearchSelect: &SearchSelectWorkflowSpec{}},
		Scrape:   &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInPostprocess(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:       []string{"https://example.com"},
		Request:     &RequestSpec{Method: "GET", Path: "/"},
		Scrape:      &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
		Postprocess: &PostprocessSpec{Assign: map[string]string{"title": "${invalid_unclosed"}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInPrecheck(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:    []string{"https://example.com"},
		Precheck: &PrecheckSpec{Variables: map[string]string{"x": "${invalid_unclosed"}},
		Request:  &RequestSpec{Method: "GET", Path: "/"},
		Scrape:   &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

// --- Condition Eval regex_match ---

func TestCompilePlugin_WithPrecheckVariables(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
precheck:
  variables:
    upper_number: "${to_upper(${number})}"
request:
  method: GET
  path: /search/${vars.upper_number}
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
	ctx = meta.SetNumberID(ctx, "abc-123")
	ok, err := plg.OnPrecheckRequest(ctx, "abc-123")
	require.NoError(t, err)
	assert.True(t, ok)
	v, _ := pluginapi.GetContainerValue(ctx, ctxVarKey("upper_number"))
	assert.Equal(t, "ABC-123", v)
}

// --- OnDecodeHTTPData with assign postprocess ---

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
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
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
