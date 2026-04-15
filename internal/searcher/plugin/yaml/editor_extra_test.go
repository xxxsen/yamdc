package yaml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestDebugScrapeDecodeFields_UnsupportedFormat(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.scrape.format = "unsupported"
	plg := &SearchPlugin{spec: compiled}
	result := &ScrapeDebugResult{Fields: map[string]FieldDebugResult{}}
	debugScrapeDecodeFields(context.Background(), plg, result, nil)
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "unsupported scrape format")
}

func TestDebugScrape_JSON_WithSampleImages(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"title":"T","sample_images":["img1.jpg","img2.jpg"]}`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title":         {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
				"sample_images": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.sample_images[*]"}, Parser: ParserSpec{Kind: "string_list"}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Len(t, result.Meta.SampleImages, 2)
}

func TestDebugScrape_JSON_DurationParser(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"title":"T","duration":"01:30:00"}`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title":    {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
				"duration": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.duration"}, Parser: ParserSpec{Kind: "duration_hhmmss"}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugScrape_JSON_TimeFormatParser(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"title":"T","release_date":"2024-01-15"}`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title":        {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
				"release_date": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.release_date"}, Parser: ParserSpec{Kind: "time_format", Layout: "2006-01-02"}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugScrape_HTML_WithDateLayoutSoft(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><span class="date">2024-01-15</span></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
				"release_date": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="date"]/text()`}, Parser: ParserSpec{Kind: "date_layout_soft", Layout: "2006-01-02"}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugRequest_MultiRequest_SuccessWhenNotMatched(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><span>no match</span></body></html>`), nil
	}}
	spec := multiRequestSpec("https://example.com")
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Attempts[0].Matched)
}

func TestDebugWorkflow_MultiRequest_BuildRequestFails(t *testing.T) {
	spec := multiRequestWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.multiRequest.request.method = "INVALID\x00METHOD"
	plg := &SearchPlugin{spec: compiled}

	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := &WorkflowDebugResult{}
	_, _, result = debugWorkflowMultiRequestPhase(ctx, &testHTTPClient{}, plg, evalCtx, result)
	assert.NotEmpty(t, result.Error)
}

func TestIterateMultiRequestCandidates_NoCandidates(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	plg := compileTestPlugin(t, spec)
	plg.spec.multiRequest.candidates = nil
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	_, err := iterateMultiRequestCandidates(ctx, &testHTTPClient{}, plg, evalCtx)
	require.ErrorIs(t, err, errNoMultiRequestCandidateTried)
}

func TestRequestDebug_NilBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	dbg := requestDebug(req)
	assert.Empty(t, dbg.Body)
}

func TestTraceAssignStringField_DateParser(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "release_date", "2024-01-15",
		ParserSpec{Kind: "time_format", Layout: "2006-01-02"})
	require.NoError(t, err)
	assert.NotZero(t, result)
}

func TestTraceAssignStringField_DurationMMSS(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "duration", "1:30",
		ParserSpec{Kind: "duration_mmss"})
	require.NoError(t, err)
	assert.NotZero(t, result)
}

func TestTraceAssignStringField_UnknownParserDefault(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "title", "T",
		ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "T", result)
}

func TestDebugScrape_WithPostprocessAssign(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">orig</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	spec.Postprocess = &PostprocessSpec{
		Assign: map[string]string{"title": "${to_upper(${meta.title})}"},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "ORIG", result.Meta.Title)
}

func TestDebugWorkflow_SingleRequest_BuildError(t *testing.T) {
	spec := twoStepWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.request.method = "INVALID\x00METHOD"
	plg := &SearchPlugin{spec: compiled}

	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := &WorkflowDebugResult{}
	resp, result := debugWorkflowSingleRequestPhase(ctx, &testHTTPClient{}, plg, evalCtx, result)
	assert.Nil(t, resp)
	assert.NotEmpty(t, result.Error)
}

func TestDebugFollowNextRequest_CaptureError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		}, nil
	}}
	plg := compileTestPlugin(t, twoStepWorkflowSpec("https://example.com"))
	matched := []*evalContext{{
		number: "ABC-123", host: "https://example.com",
		item: map[string]string{"link": "/detail/1"},
	}}
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	_, _, err := debugFollowNextRequest(ctx, cli, plg, plg.spec.workflow, matched)
	require.Error(t, err)
}

func TestDebugWorkflow_MultiRequest_CaptureResponseError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		}, nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}"},
			Request:    &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{
				Mode:       "and",
				Conditions: []string{`selector_exists(xpath("//div"))`},
			},
		},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a/@href"}},
				Return:      "${item.link}",
				NextRequest: &RequestSpec{Method: "GET", Path: "${value}"},
			},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}},
		}},
	}
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

func TestTryDebugMultiRequestCandidate_CaptureResponseError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		}, nil
	}}
	plg := compileTestPlugin(t, multiRequestSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	attempt, resp := tryDebugMultiRequestCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.NotEmpty(t, attempt.Error)
	assert.Nil(t, resp)
}

func TestTryDebugWorkflowCandidate_CaptureResponseError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		}, nil
	}}
	plg := compileTestPlugin(t, multiRequestWorkflowSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := tryDebugWorkflowCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.False(t, result.matched)
	assert.Contains(t, result.step.Summary, "read response failed")
}

func TestDebugWorkflow_MultiRequest_RenderCandidateError(t *testing.T) {
	spec := multiRequestWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	compiled.multiRequest.candidates = []*template{badTmpl}
	plg := &SearchPlugin{spec: compiled}

	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	_, _, err = debugWorkflowMultiRequest(ctx, &testHTTPClient{}, plg, evalCtx)
	require.Error(t, err)
}

func TestDebugMatchWorkflowItem_ItemVariableRenderError(t *testing.T) {
	w := &compiledSearchSelectWorkflow{
		selectors: []*compiledSelectorList{
			{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
		},
		itemVariables: map[string]*template{},
	}
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	w.itemVariables["bad"] = badTmpl
	results := map[string][]string{"link": {"/a"}}
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	_, _, err = debugMatchWorkflowItem(evalCtx, w, results, "", nil, 0)
	require.Error(t, err)
}

func TestDebugMatchAllWorkflowItems_ErrorPropagation(t *testing.T) {
	w := &compiledSearchSelectWorkflow{
		selectors: []*compiledSelectorList{
			{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
		},
		itemVariables: map[string]*template{},
	}
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	w.itemVariables["bad"] = badTmpl
	results := map[string][]string{"link": {"/a"}}
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	step := &WorkflowDebugStep{Items: make([]WorkflowSelectorItem, 0)}
	_, err = debugMatchAllWorkflowItems(evalCtx, w, results, "", nil, 1, step)
	require.Error(t, err)
}

func TestTryDebugWorkflowCandidate_BuildRequestError(t *testing.T) {
	spec := multiRequestWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.multiRequest.request.method = "INVALID\x00METHOD"
	plg := &SearchPlugin{spec: compiled}

	cli := &testHTTPClient{}
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := tryDebugWorkflowCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.False(t, result.matched)
	assert.Contains(t, result.step.Summary, "build request")
}

func TestTryDebugMultiRequestCandidate_BuildRequestError(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.multiRequest.request.method = "INVALID\x00METHOD"
	plg := &SearchPlugin{spec: compiled}

	cli := &testHTTPClient{}
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	attempt, resp := tryDebugMultiRequestCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.NotEmpty(t, attempt.Error)
	assert.Nil(t, resp)
}

func TestDebugScrape_MultiRequest_NotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><span>no match</span></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}"},
			Request:    &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{
				Mode:       "and",
				Conditions: []string{`selector_exists(xpath("//div[@class='found']"))`},
			},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
		}},
	}
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestDebugScrape_HTML_RequiredStringField(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"number": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="num"]/text()`}, Required: true},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.Nil(t, result.Meta)
}

func TestDebugWorkflow_FollowNextRequest_CaptureOK(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">link</a></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1>Title</h1></body></html>`), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.GreaterOrEqual(t, len(result.Steps), 2)
}

func TestDebugScrape_TwoStep_WorkflowHandled(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">link</a></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1 class="title">DetailTitle</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a/@href"}},
				Return:      "${item.link}",
				NextRequest: &RequestSpec{Method: "GET", Path: "${value}"},
			},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title":  {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
			"actors": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="actors"]/span/text()`}, Parser: ParserSpec{Kind: "string_list"}},
		}},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "DetailTitle", result.Meta.Title)
}

func TestDebugScrape_ScrapeFetch_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestDebugWorkflow_SearchSelect_ErrorInMatchCondition(t *testing.T) {
	spec := twoStepWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	compiled.workflow.itemVariables["bad"] = badTmpl

	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}

	htmlStr := `<html><body><a href="/detail/1">link</a></body></html>`
	baseResp := &HTTPResponseDebug{StatusCode: 200, Body: htmlStr}
	steps, err := debugWorkflowSearchSelect(ctx, &testHTTPClient{}, plg, evalCtx, baseResp)
	_ = steps
	require.Error(t, err)
}

func TestNormalizeStringSet_WithDuplicatesAndEmpty(t *testing.T) {
	result := normalizeStringSet([]string{"b", "", "a", " ", "b", "c"})
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestEqualNormalizedSet_DifferentLengths(t *testing.T) {
	assert.False(t, equalNormalizedSet([]string{"a"}, []string{"a", "b"}))
}

func TestEqualNormalizedSet_DifferentContent(t *testing.T) {
	assert.False(t, equalNormalizedSet([]string{"a", "b"}, []string{"a", "c"}))
}

func TestRenderCondition_NilCondition(t *testing.T) {
	assert.Empty(t, renderCondition(nil))
}

func TestRenderCondition_Named(t *testing.T) {
	cond := &compiledCondition{name: "contains"}
	assert.Equal(t, "contains", renderCondition(cond))
}

func TestDebugRequest_MultiRequest_InvalidMethodError(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.multiRequest.request.method = "INVALID\x00METHOD"
	plg := &SearchPlugin{spec: compiled}

	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	_, err = iterateMultiRequestCandidates(ctx, &testHTTPClient{}, plg, evalCtx)
	require.Error(t, err)
}

func TestDebugScrape_HTML_ListFieldWithTransforms(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body>
			<h1 class="title">T</h1>
			<div class="genres"><span> Action </span><span></span><span> Drama </span></div>
		</body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
				"genres": {
					Selector:   &SelectorSpec{Kind: "xpath", Expr: `//div[@class="genres"]/span/text()`},
					Transforms: []*TransformSpec{{Kind: "map_trim"}, {Kind: "remove_empty"}, {Kind: "dedupe"}},
					Parser:     ParserSpec{Kind: "string_list"},
				},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, []string{"Action", "Drama"}, result.Meta.Genres)
}

func TestDebugScrape_JSON_RequiredListFieldNotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"title":"T"}`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title":  {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
				"actors": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors[*]"}, Required: true, Parser: ParserSpec{Kind: "string_list"}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.Nil(t, result.Meta)
}

func TestDebugScrape_WithQueryAndHeaders(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "val", req.URL.Query().Get("q"))
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.Query = map[string]string{"q": "val"}
	spec.Request.Headers = map[string]string{"X-Custom": "hval"}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugScrape_WithPOSTAndFormBody(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, req.Method)
		body, _ := io.ReadAll(req.Body)
		assert.Contains(t, string(body), "query")
		req.Body = io.NopCloser(bytes.NewReader(body))
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.Method = "POST"
	spec.Request.Body = &RequestBodySpec{Kind: "form", Values: map[string]string{"query": "${number}"}}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugScrape_WithCookies(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		cookies := req.Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "session" && c.Value == "abc" {
				found = true
			}
		}
		assert.True(t, found)
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	spec.Request.Cookies = map[string]string{"session": "abc"}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugScrape_WithURLInsteadOfPath(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		assert.True(t, strings.HasPrefix(req.URL.String(), "https://other.com/"))
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", URL: "https://other.com/search/${number}"},
		Scrape:  simpleOneStepSpec("https://example.com").Scrape,
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

func TestDebugScrape_HTML_AllStringTransforms(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body>
			<h1 class="title">PRE-my title-SUF</h1>
			<div class="actors"><span>A</span></div>
		</body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`},
					Transforms: []*TransformSpec{
						{Kind: "trim"},
						{Kind: "trim_prefix", Value: "PRE-"},
						{Kind: "trim_suffix", Value: "-SUF"},
						{Kind: "replace", Old: " ", New: "_"},
						{Kind: "to_upper"},
					},
				},
				"actors": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="actors"]/span/text()`},
					Parser:   ParserSpec{Kind: "string_list"},
				},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "MY_TITLE", result.Meta.Title)
}

func TestCompileDraft_Success(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	result, err := CompileDraft(spec)
	require.NoError(t, err)
	assert.True(t, result.Summary.HasRequest)
	assert.Equal(t, "html", result.Summary.ScrapeFormat)
	assert.NotEmpty(t, result.YAML)
}

func TestCompileDraft_InvalidSpec(t *testing.T) {
	spec := &PluginSpec{Version: 1, Name: "bad"}
	_, err := CompileDraft(spec)
	require.Error(t, err)
}

func TestDebugScrapeDecodeFields_HTMLSuccess(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	result := &ScrapeDebugResult{Fields: map[string]FieldDebugResult{}}
	htmlBody := []byte(`<html><body><h1>My Title</h1></body></html>`)
	debugScrapeDecodeFields(context.Background(), plg, result, htmlBody)
	assert.Empty(t, result.Error)
}

func TestDebugScrapeDecodeFields_JSONSuccess(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	result := &ScrapeDebugResult{Fields: map[string]FieldDebugResult{}}
	jsonBody := []byte(`{"title":"My Title"}`)
	debugScrapeDecodeFields(context.Background(), plg, result, jsonBody)
	assert.Empty(t, result.Error)
}

func TestDebugScrapeDecodeFields_JSONInvalid(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	result := &ScrapeDebugResult{Fields: map[string]FieldDebugResult{}}
	debugScrapeDecodeFields(context.Background(), plg, result, []byte(`not-json`))
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "trace json fields failed")
}

func TestDebugRequest_SingleRequest_OKResponse(t *testing.T) {
	htmlBody := `<html><body><h1>Test</h1></body></html>`
	cli := &testHTTPClient{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(htmlBody)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		},
	}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotNil(t, result.Request)
	assert.NotNil(t, result.Response)
}

func TestDebugRequest_PrecheckNotMatched(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Precheck = &PrecheckSpec{NumberPatterns: []string{"^NOMATCH"}}
	_, err := DebugRequest(context.Background(), &testHTTPClient{}, spec, "ABC-123")
	require.ErrorIs(t, err, errPrecheckNotMatched)
}

func TestDebugRequest_CompileFails(t *testing.T) {
	spec := &PluginSpec{Version: 1, Name: "bad"}
	_, err := DebugRequest(context.Background(), &testHTTPClient{}, spec, "ABC-123")
	require.Error(t, err)
}

func TestTraceFieldJSON_ListField(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"actors": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors"}},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	var doc any
	_ = json.Unmarshal([]byte(`{"actors":["Alice","Bob"]}`), &doc)
	field := plg.fieldByName("actors")
	dbg, err := plg.traceFieldJSON(context.Background(), mv, doc, field)
	require.NoError(t, err)
	assert.True(t, dbg.Matched)
}

func TestTraceFieldJSON_StringFieldEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	var doc any
	_ = json.Unmarshal([]byte(`{"other":"value"}`), &doc)
	field := plg.fieldByName("title")
	dbg, err := plg.traceFieldJSON(context.Background(), mv, doc, field)
	require.NoError(t, err)
	assert.False(t, dbg.Matched)
}

func TestTraceAssignStringField_DateOnly(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "release_date", "2024-01-15", ParserSpec{Kind: "date_only"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTraceAssignStringField_DurationMmss(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "duration", "120:30", ParserSpec{Kind: "duration_mmss"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTraceAssignStringField_EmptyValue(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "title", "", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestTraceAssignStringField_UnknownParserErrors(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	_, err := traceAssignStringField(context.Background(), mv, "title", "Test", ParserSpec{Kind: "unknown_parser"})
	require.Error(t, err)
}

func TestTraceAssignStringField_StringKind(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "title", "Test", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "Test", result)
}

func TestTraceAssignStringField_NoParserKind(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "title", "Test", ParserSpec{})
	require.NoError(t, err)
	assert.Equal(t, "Test", result)
}

func TestTraceDecodeHTML_RequiredFieldNotMatched(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}, Required: true},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	node := helperParseHTMLForEditor(t, "<html><body><p>no title</p></body></html>")
	fields := map[string]FieldDebugResult{}
	mv, err := plg.traceDecodeHTML(context.Background(), node, fields)
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestTraceDecodeJSON_RequiredFieldNotMatched(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}, Required: true},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	fields := map[string]FieldDebugResult{}
	mv, err := plg.traceDecodeJSON(context.Background(), []byte(`{"other":"val"}`), fields)
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestDebugValidateMatchCount_ExpectCountMismatch(t *testing.T) {
	w := &compiledSearchSelectWorkflow{
		match: &compiledConditionGroup{expectCount: 1},
	}
	step := &WorkflowDebugStep{}
	matched := []*evalContext{{}, {}}
	err := debugValidateMatchCount(matched, w, 5, "link=5", 100, step)
	require.ErrorIs(t, err, errSearchSelectCountMismatch)
}

func TestDebugValidateMatchCount_NoMatch(t *testing.T) {
	w := &compiledSearchSelectWorkflow{}
	step := &WorkflowDebugStep{}
	err := debugValidateMatchCount([]*evalContext{}, w, 5, "link=5", 100, step)
	require.ErrorIs(t, err, errNoSearchSelectMatched)
}

func TestDebugValidateMatchCount_Success(t *testing.T) {
	w := &compiledSearchSelectWorkflow{}
	step := &WorkflowDebugStep{}
	err := debugValidateMatchCount([]*evalContext{{}}, w, 1, "link=1", 50, step)
	require.NoError(t, err)
	assert.Contains(t, step.Summary, "1/1 items matched")
}

func TestDebugCollectSelectors_CountMismatch(t *testing.T) {
	w := &compiledSearchSelectWorkflow{
		selectors: []*compiledSelectorList{
			{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
			{name: "title", compiledSelector: compiledSelector{kind: "xpath", expr: "//h2/text()"}},
		},
	}
	node := helperParseHTMLForEditor(t, `<html><body><a href="/a">A</a><a href="/b">B</a><h2>Only one</h2></body></html>`)
	_, _, _, err := debugCollectSelectors(node, w)
	require.ErrorIs(t, err, errSelectorCountMismatch)
}

func TestDebugWorkflowSearchSelect_NilBaseResponse(t *testing.T) {
	spec := twoStepWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	_, err = debugWorkflowSearchSelect(context.Background(), &testHTTPClient{}, plg, evalCtx, nil)
	require.ErrorIs(t, err, errMissingWorkflowBaseResponse)
}

func TestIterateMultiRequestCandidates_UniqueDedup(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	spec.MultiRequest.Unique = true
	spec.MultiRequest.Candidates = []string{"ABC-123", "ABC-123", "ABC-123"}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}

	callCount := 0
	cli := &testHTTPClient{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html><body></body></html>")),
				Header:     http.Header{},
			}, nil
		},
	}
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	_, _ = iterateMultiRequestCandidates(ctx, cli, plg, evalCtx)
	assert.Equal(t, 1, callCount)
}

func buildTestPluginFrom(t *testing.T, spec *PluginSpec) *SearchPlugin {
	t.Helper()
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	return &SearchPlugin{spec: compiled}
}

func helperParseHTMLForEditor(t *testing.T, s string) *html.Node {
	t.Helper()
	n, err := htmlquery.Parse(strings.NewReader(s))
	require.NoError(t, err)
	return n
}
