package yaml

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestDebugWorkflow_MultiRequest(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/search/ABC-123") {
			return makeResponse(200, `<html><body><div class="found">ok</div></body></html>`), nil
		}
		return makeResponse(404, "not found"), nil
	}}
	spec := multiRequestWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDebugWorkflow_NotConfigured(t *testing.T) {
	cli := &testHTTPClient{}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugScrape with JSON format ---

func TestDebugWorkflow_WithMatch(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body>
				<a href="/video/1">video link</a>
				<a href="/image/2">image link</a>
			</body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1>Detail</h1></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
					{Name: "text", Kind: "xpath", Expr: "//a/text()"},
				},
				ItemVariables: map[string]string{"slug": "${item.link}"},
				Match: &ConditionGroupSpec{
					Mode:       "and",
					Conditions: []string{`contains("${item.text}", "video")`},
				},
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
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
}

func TestDebugWorkflow_MatchExpectCount(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><a href="/1">video A</a><a href="/2">video B</a></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
					{Name: "text", Kind: "xpath", Expr: "//a/text()"},
				},
				Match: &ConditionGroupSpec{
					Mode:        "and",
					Conditions:  []string{`contains("${item.text}", "video")`},
					ExpectCount: 5,
				},
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

func TestDebugWorkflow_NextRequestFailed(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">link</a></body></html>`), nil
		}
		return makeResponse(404, "not found"), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugRequest with single request ---

func TestDebugWorkflow_MultiRequest_AllFail(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := multiRequestWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugScrape JSON with list field (genres) ---

func TestDebugWorkflow_SingleRequest(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">link</a></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1>Detail Page</h1></body></html>`), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.GreaterOrEqual(t, len(result.Steps), 2)
}

func TestDebugWorkflow_SingleRequest_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(500, "error"), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

func TestDebugWorkflow_SingleRequest_NoMatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body></body></html>`), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugWorkflow with multi_request + workflow ---

func TestDebugWorkflow_MultiRequest_Success(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		callCount++
		if strings.Contains(req.URL.Path, "/search/") {
			return makeResponse(200, `<html><body><div class="found">ok</div><a href="/detail/1">link</a></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1>Detail</h1></body></html>`), nil
	}}
	spec := multiRequestWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
}

// --- DebugCase with ActorSet match ---

func TestDebugWorkflowSingleRequestPhase_RequestError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	evalCtx := &evalContext{number: "ABC", host: "https://example.com"}
	result := &WorkflowDebugResult{}
	_, updatedResult := debugWorkflowSingleRequestPhase(context.Background(), cli, plg, evalCtx, result)
	assert.NotEmpty(t, updatedResult.Error)
}

// --- debugCollectSelectors mismatch ---

func TestDebugCollectSelectors_Mismatch(t *testing.T) {
	body := `<html><body><a href="1">x</a><a href="2">y</a><span class="t">only-one</span></body></html>`
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
					{Name: "title", Kind: "xpath", Expr: "//span[@class='t']/text()"},
				},
				Return:      "${item.link}",
				NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
			},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, body), nil
	}}
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugScrape with postprocess ---

func TestDebugWorkflow_MultiRequest_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
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
	assert.Contains(t, result.Error, "multi_request failed")
}

func TestDebugWorkflow_MultiRequest_StatusRejected(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, `<html><body>not found</body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}"},
			Request: &RequestSpec{
				Method:            "GET",
				Path:              "/search/${candidate}",
				AcceptStatusCodes: []int{200},
			},
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

func TestDebugWorkflow_MultiRequest_SuccessWhenNotMatched(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><span>no match</span></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}"},
			Unique:     true,
			Request:    &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{
				Mode:       "and",
				Conditions: []string{`selector_exists(xpath("//div[@class='found']"))`},
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

func TestDebugWorkflow_MultiRequest_DuplicateCandidate(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		return makeResponse(200, `<html><body><div class="found">ok</div><a href="/detail">link</a><h1>Title</h1></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}", "${number}"},
			Unique:     true,
			Request:    &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{
				Mode:       "and",
				Conditions: []string{`selector_exists(xpath("//div[@class='found']"))`},
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
	found := false
	for _, step := range result.Steps {
		if strings.Contains(step.Summary, "duplicate") || strings.Contains(step.Summary, "candidate matched") {
			found = true
		}
	}
	assert.True(t, found || result.Error == "")
}

// --- DebugWorkflow: search_select followNextRequest status error ---

func TestDebugWorkflow_NextRequestStatusRejected(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">ABC-123</a></body></html>`), nil
		}
		return makeResponse(403, `<html><body>forbidden</body></html>`), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	spec.Workflow.SearchSelect.NextRequest.AcceptStatusCodes = []int{200}
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugWorkflow: search_select with item_variables ---

func TestDebugWorkflow_WithItemVariables(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/abc-123">ABC-123</a><span class="text">ABC-123</span></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1>Detail Title</h1></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
					{Name: "text", Kind: "xpath", Expr: "//span[@class='text']/text()"},
				},
				ItemVariables: map[string]string{"clean": "${to_upper(${item.text})}"},
				Match: &ConditionGroupSpec{
					Mode:       "and",
					Conditions: []string{`contains("${item.text}", "${number}")`},
				},
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
	assert.Empty(t, result.Error)
}

// --- DebugWorkflow: search_select no match (zero items matched condition) ---

func TestDebugWorkflow_SearchSelect_NoMatchedItems(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/a">NOPE</a><span class="t">NOPE</span></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1>Detail</h1></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
					{Name: "text", Kind: "xpath", Expr: "//span[@class='t']/text()"},
				},
				Match: &ConditionGroupSpec{
					Mode:       "and",
					Conditions: []string{`contains("${item.text}", "DOESNOTEXIST")`},
				},
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

// --- DebugRequest: tryDebugMultiRequestCandidate error paths ---

func TestDebugWorkflow_SingleRequest_StatusCheckFailed(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(500, "error"), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}", AcceptStatusCodes: []int{200}},
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
	assert.Contains(t, result.Error, "status check failed")
}

// --- newCompiledPlugin compile error ---

func TestDebugWorkflow_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestDebugWorkflow_NilBaseResp(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("always fail")
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
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}},
		}},
	}
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugWorkflow: expect_count validation ---

func TestDebugWorkflow_ExpectCountMismatch(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		return makeResponse(200, `<html><body><a href="/a">X</a><a href="/b">Y</a></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
				},
				Match: &ConditionGroupSpec{
					Mode:        "and",
					Conditions:  []string{`contains("${item.link}", "/")`},
					ExpectCount: 1,
				},
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

// --- DebugRequest nextRequest follow via workflow ---

func TestDebugWorkflow_FollowNextRequest_HTTPError(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">ABC-123</a></body></html>`), nil
		}
		return nil, fmt.Errorf("detail page unreachable")
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

// --- DebugScrape via two-step (exercises debugScrapeFetch with handleRequest) ---

func TestDebugWorkflow_PrecheckNotMatched(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:    []string{"https://example.com"},
		Precheck: &PrecheckSpec{NumberPatterns: []string{`^ONLY-\d+$`}},
		Request:  &RequestSpec{Method: "GET", Path: "/search/${number}"},
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
	_, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- helpers ---

func TestTryDebugWorkflowCandidate_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	}}
	plg := compileTestPlugin(t, multiRequestWorkflowSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := tryDebugWorkflowCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.False(t, result.matched)
	assert.Contains(t, result.step.Summary, "request failed")
}

func TestTryDebugWorkflowCandidate_StatusRejected(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(500, "error"), nil
	}}
	spec := multiRequestWorkflowSpec("https://example.com")
	spec.MultiRequest.Request.AcceptStatusCodes = []int{200}
	plg := compileTestPlugin(t, spec)
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := tryDebugWorkflowCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.False(t, result.matched)
	assert.Contains(t, result.step.Summary, "rejected")
}

func TestTryDebugWorkflowCandidate_SuccessWhenNotMatched(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><span>no match</span></body></html>`), nil
	}}
	plg := compileTestPlugin(t, multiRequestWorkflowSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := tryDebugWorkflowCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.False(t, result.matched)
	assert.Contains(t, result.step.Summary, "not matched")
}

func TestTryDebugWorkflowCandidate_Success(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><div class="found">ok</div></body></html>`), nil
	}}
	plg := compileTestPlugin(t, multiRequestWorkflowSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := tryDebugWorkflowCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.True(t, result.matched)
	assert.Contains(t, result.step.Summary, "candidate matched")
}

// --- captureHTTPResponse: error reading body ---

func TestDebugWorkflowSingleRequestPhase_CaptureError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		}, nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	plg := compileTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	result := &WorkflowDebugResult{}
	resp, result := debugWorkflowSingleRequestPhase(ctx, cli, plg, evalCtx, result)
	assert.Nil(t, resp)
	assert.NotEmpty(t, result.Error)
}

// --- debugFollowNextRequest: cli.Do error ---

func TestDebugFollowNextRequest_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
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

// --- debugFollowNextRequest: status check error ---

func TestDebugFollowNextRequest_StatusRejected(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(500, "error"), nil
	}}
	spec := twoStepWorkflowSpec("https://example.com")
	spec.Workflow.SearchSelect.NextRequest.AcceptStatusCodes = []int{200}
	plg := compileTestPlugin(t, spec)
	matched := []*evalContext{{
		number: "ABC-123", host: "https://example.com",
		item: map[string]string{"link": "/detail/1"},
	}}
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	value, step, err := debugFollowNextRequest(ctx, cli, plg, plg.spec.workflow, matched)
	require.Error(t, err)
	assert.NotEmpty(t, value)
	assert.NotNil(t, step)
}

// --- debugFollowNextRequest: success ---

func TestDebugFollowNextRequest_Success(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1>Title</h1></body></html>`), nil
	}}
	plg := compileTestPlugin(t, twoStepWorkflowSpec("https://example.com"))
	matched := []*evalContext{{
		number: "ABC-123", host: "https://example.com",
		item: map[string]string{"link": "/detail/1"},
	}}
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	value, step, err := debugFollowNextRequest(ctx, cli, plg, plg.spec.workflow, matched)
	require.NoError(t, err)
	assert.NotEmpty(t, value)
	assert.NotNil(t, step)
}

// --- fieldByName: no match ---

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
	_, _, _, err := debugCollectSelectors(node, w) //nolint:dogsled
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
