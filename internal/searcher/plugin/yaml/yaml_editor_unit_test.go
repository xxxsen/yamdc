package yaml

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

type testHTTPClient struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (c *testHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.roundTrip != nil {
		return c.roundTrip(req)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func makeResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Header:     make(http.Header),
	}
}

// --- DebugCase ---

func TestDebugCase_Success(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">MyTitle</h1><div class="actors"><span>Alice</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:    "MyTitle",
			ActorSet: []string{"Alice"},
			TagSet:   nil,
			Status:   "success",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_ExpectError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Status: "error",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_UnexpectedError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Status: "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

func TestDebugCase_TitleMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">Wrong</h1></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:  "Expected",
			Status: "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

func TestDebugCase_TagSetMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="genres"><span>Action</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpecWithGenres("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:  "T",
			TagSet: []string{"Drama"},
			Status: "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

func TestDebugCase_StatusNotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body></body></html>`), nil
	}}
	spec := simpleOneStepSpecRequired("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:   "test",
		Input:  "ABC-123",
		Output: CaseOutput{Status: "not_found"},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_StatusMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body></body></html>`), nil
	}}
	spec := simpleOneStepSpecRequired("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:   "test",
		Input:  "ABC-123",
		Output: CaseOutput{Status: "success"},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

// --- DebugRequest with multi_request ---

func TestDebugRequest_MultiRequest(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/ABC-123") {
			return makeResponse(200, `<html><body><div class="found">ok</div></body></html>`), nil
		}
		return makeResponse(404, "not found"), nil
	}}
	spec := multiRequestSpec("https://example.com")
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ABC-123", result.Candidate)
}

func TestDebugRequest_MultiRequest_NoMatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := multiRequestSpec("https://example.com")
	_, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugWorkflow with multi_request ---

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

func TestDebugScrape_JSON(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"title": "MyTitle", "actors": ["A", "B"]}`), nil
	}}
	spec := jsonScrapeSpec("https://example.com")
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "MyTitle", result.Meta.Title)
}

// --- requestDebug with body ---

func TestRequestDebug_WithBody(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", bytes.NewReader([]byte("body-data")))
	dbg := requestDebug(req)
	assert.Equal(t, "body-data", dbg.Body)
}

// --- traceAssignStringField ---

func TestTraceAssignStringField(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "title", "MyTitle", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "MyTitle", result)

	mv2 := &model.MovieMeta{}
	result, err = traceAssignStringField(ctx, mv2, "title", "", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "", result)

	mv3 := &model.MovieMeta{}
	result, err = traceAssignStringField(ctx, mv3, "release_date", "2024-01-02", ParserSpec{Kind: "date_only"})
	require.NoError(t, err)
	assert.NotZero(t, result)

	mv4 := &model.MovieMeta{}
	result, err = traceAssignStringField(ctx, mv4, "duration", "120分钟", ParserSpec{Kind: "duration_default"})
	require.NoError(t, err)
	assert.NotZero(t, result)

	mv5 := &model.MovieMeta{}
	_, err = traceAssignStringField(ctx, mv5, "title", "T", ParserSpec{Kind: "unknown_custom"})
	require.Error(t, err)
}

// --- traceStringTransforms / traceListTransforms ---

func TestTraceStringTransforms(t *testing.T) {
	var steps []TransformStep
	result := traceStringTransforms(" abc ", []*TransformSpec{{Kind: "trim"}}, &steps)
	assert.Equal(t, "abc", result)
	assert.Len(t, steps, 1)
}

func TestTraceListTransforms(t *testing.T) {
	var steps []TransformStep
	result := traceListTransforms([]string{" a ", " b "}, []*TransformSpec{{Kind: "map_trim"}}, &steps)
	assert.Equal(t, []string{"a", "b"}, result)
	assert.Len(t, steps, 1)
}

// --- captureHTTPResponse ---

func TestCaptureHTTPResponse(t *testing.T) {
	rsp := makeResponse(200, "body-text")
	defer func() { _ = rsp.Body.Close() }()
	resp, err := captureHTTPResponse(rsp, "")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "body-text", resp.Body)
}

// --- cloneHeader ---

func TestCloneHeader(t *testing.T) {
	h := http.Header{"X-Key": {"v1", "v2"}}
	c := cloneHeader(h)
	assert.Equal(t, []string{"v1", "v2"}, c["X-Key"])
}

// --- helpers ---

func simpleOneStepSpec(host string) *PluginSpec {
	return &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{host},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector:   &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`},
					Transforms: []*TransformSpec{{Kind: "trim"}},
					Parser:     ParserSpec{Kind: "string"},
				},
				"actors": {
					Selector:   &SelectorSpec{Kind: "xpath", Expr: `//div[@class="actors"]/span/text()`, Multi: true},
					Transforms: []*TransformSpec{{Kind: "map_trim"}, {Kind: "remove_empty"}},
					Parser:     ParserSpec{Kind: "string_list"},
				},
			},
		},
	}
}

func simpleOneStepSpecWithGenres(host string) *PluginSpec {
	spec := simpleOneStepSpec(host)
	spec.Scrape.Fields["genres"] = &FieldSpec{
		Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="genres"]/span/text()`, Multi: true},
		Parser:   ParserSpec{Kind: "string_list"},
	}
	return spec
}

func simpleOneStepSpecRequired(host string) *PluginSpec {
	spec := simpleOneStepSpec(host)
	spec.Scrape.Fields["title"].Required = true
	return spec
}

func multiRequestSpec(host string) *PluginSpec { //nolint:unparam
	return &PluginSpec{
		Version: 1,
		Name:    "test-multi",
		Type:    "one-step",
		Hosts:   []string{host},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}", "${to_lower(${number})}"},
			Unique:     true,
			Request:    &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{
				Mode:       "and",
				Conditions: []string{`selector_exists(xpath("//div[@class='found']"))`},
			},
		},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="found"]/text()`},
					Parser:   ParserSpec{Kind: "string"},
				},
			},
		},
	}
}

func multiRequestWorkflowSpec(host string) *PluginSpec { //nolint:unparam
	return &PluginSpec{
		Version: 1,
		Name:    "test-multi-wf",
		Type:    "two-step",
		Hosts:   []string{host},
		MultiRequest: &MultiRequestSpec{
			Candidates: []string{"${number}", "${to_lower(${number})}"},
			Unique:     true,
			Request:    &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{
				Mode:       "and",
				Conditions: []string{`selector_exists(xpath("//div[@class='found']"))`},
			},
		},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
				},
				Return:      "${item.link}",
				NextRequest: &RequestSpec{Method: "GET", Path: "${value}"},
			},
		},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="found"]/text()`},
					Parser:   ParserSpec{Kind: "string"},
				},
			},
		},
	}
}

func jsonScrapeSpec(host string) *PluginSpec {
	return &PluginSpec{
		Version: 1,
		Name:    "test-json",
		Type:    "one-step",
		Hosts:   []string{host},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"},
					Parser:   ParserSpec{Kind: "string"},
				},
				"actors": {
					Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors[*]", Multi: true},
					Parser:   ParserSpec{Kind: "string_list"},
				},
			},
		},
	}
}

var _ = model.MovieMeta{}

// --- DebugWorkflow with match conditions ---

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

func TestDebugRequest_SingleRequest_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugScrape with response treated as not found ---

func TestDebugScrape_NotFoundResponse(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugWorkflow multi_request with workflow - all candidates fail ---

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

func TestDebugScrape_JSON_WithListFields(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"title":"T","actors":["A","B"],"genres":["Action"]}`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title":  {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
				"actors": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors[*]"}, Parser: ParserSpec{Kind: "string_list"}},
				"genres": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.genres[*]"}, Parser: ParserSpec{Kind: "string_list"}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, []string{"A", "B"}, result.Meta.Actors)
	assert.Equal(t, []string{"Action"}, result.Meta.Genres)
}

// --- DebugScrape with JSON required not found ---

func TestDebugScrape_JSON_RequiredNotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `{"other":"val"}`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/api/${number}"},
		Scrape: &ScrapeSpec{
			Format: "json",
			Fields: map[string]*FieldSpec{
				"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}, Required: true},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.Nil(t, result.Meta)
}

// --- DebugScrape HTML with required list field not found ---

func TestDebugScrape_HTML_RequiredListNotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title":  {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
				"actors": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="actors"]/span/text()`}, Parser: ParserSpec{Kind: "string_list"}, Required: true},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	assert.Nil(t, result.Meta)
}

// --- DebugScrape with string transforms ---

func TestDebugScrape_HTML_WithTransforms(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">  My Title  </h1><span class="date">2024-01-15</span><span class="dur">120分</span></body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector:   &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`},
					Transforms: []*TransformSpec{{Kind: "trim"}},
				},
				"release_date": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="date"]/text()`},
					Parser:   ParserSpec{Kind: "date_only"},
				},
				"duration": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="dur"]/text()`},
					Parser:   ParserSpec{Kind: "duration_default"},
				},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "My Title", result.Meta.Title)
}

// --- DebugRequest precheck failure ---

func TestDebugRequest_PrecheckFails(t *testing.T) {
	cli := &testHTTPClient{}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:    []string{"https://example.com"},
		Precheck: &PrecheckSpec{NumberPatterns: []string{`^NOPE$`}},
		Request:  &RequestSpec{Method: "GET", Path: "/"},
		Scrape:   &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	_, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- CompileDraft with multi_request and workflow ---

func TestCompileDraft_MultiRequest(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	result, err := CompileDraft(spec)
	require.NoError(t, err)
	assert.True(t, result.Summary.HasMultiRequest)
	assert.False(t, result.Summary.HasWorkflow)
}

func TestCompileDraft_Workflow(t *testing.T) {
	spec := twoStepWorkflowSpec("https://example.com")
	result, err := CompileDraft(spec)
	require.NoError(t, err)
	assert.True(t, result.Summary.HasWorkflow)
}

// --- DebugScrape HTML with all field types ---

func TestDebugScrape_HTML_AllFieldTypes(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body>
			<h1 class="title">Title</h1>
			<div class="actors"><span> A </span><span> B </span></div>
			<div class="genres"><span>Genre1</span><span>Genre2</span></div>
			<div class="images"><img src="img1.jpg"/><img src="img2.jpg"/></div>
			<img class="cover" src="cover.jpg"/>
			<img class="poster" src="poster.jpg"/>
			<span class="num">NUM-1</span>
			<div class="plot">Plot text</div>
			<span class="studio">Studio</span>
			<span class="label">Label</span>
			<span class="director">Dir</span>
			<span class="series">Series</span>
		</body></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title":         {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
				"actors":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="actors"]/span/text()`}, Transforms: []*TransformSpec{{Kind: "map_trim"}, {Kind: "remove_empty"}}, Parser: ParserSpec{Kind: "string_list"}},
				"genres":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="genres"]/span/text()`}, Parser: ParserSpec{Kind: "string_list"}},
				"sample_images": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="images"]/img/@src`}, Parser: ParserSpec{Kind: "string_list"}},
				"cover":         {Selector: &SelectorSpec{Kind: "xpath", Expr: `//img[@class="cover"]/@src`}},
				"poster":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//img[@class="poster"]/@src`}},
				"number":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="num"]/text()`}},
				"plot":          {Selector: &SelectorSpec{Kind: "xpath", Expr: `//div[@class="plot"]/text()`}},
				"studio":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="studio"]/text()`}},
				"label":         {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="label"]/text()`}},
				"director":      {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="director"]/text()`}},
				"series":        {Selector: &SelectorSpec{Kind: "xpath", Expr: `//span[@class="series"]/text()`}},
			},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "Title", result.Meta.Title)
	assert.Equal(t, []string{"A", "B"}, result.Meta.Actors)
	assert.Equal(t, []string{"Genre1", "Genre2"}, result.Meta.Genres)
	assert.Equal(t, "cover.jpg", result.Meta.Cover.Name)
}

// --- DebugScrape HTML ---

func TestDebugScrape_HTML(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">HtmlTitle</h1><div class="actors"><span>A</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "HtmlTitle", result.Meta.Title)
}

func TestDebugScrape_PrecheckFails(t *testing.T) {
	cli := &testHTTPClient{}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Precheck: &PrecheckSpec{
			NumberPatterns: []string{`^MATCH-\d+$`},
		},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := DebugScrape(context.Background(), cli, spec, "NOMATCH")
	require.Error(t, err)
}

func TestDebugScrape_UnsupportedFormat(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	compiled.scrape.format = "xml"
	result := &ScrapeDebugResult{Fields: map[string]FieldDebugResult{}}
	debugScrapeDecodeFields(context.Background(), &SearchPlugin{spec: compiled}, result, []byte("data"))
	assert.Contains(t, result.Error, "unsupported")
}

// --- DebugRequest with single request ---

func TestDebugRequest_SingleRequest(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body>ok</body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Request.URL)
}

// --- DebugWorkflow with single request (two-step) ---

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

func TestDebugCase_ActorSetMatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>Alice</span><span>Bob</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:    "T",
			ActorSet: []string{"Bob", "Alice"},
			Status:   "success",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_ActorSetMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>Alice</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:    "T",
			ActorSet: []string{"Charlie"},
			Status:   "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

// --- debugScrapeDecodeFields with JSON error ---

func TestDebugScrapeDecodeFields_JSONError(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape:  &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	result := &ScrapeDebugResult{Fields: map[string]FieldDebugResult{}}
	debugScrapeDecodeFields(context.Background(), plg, result, []byte("not-json"))
	assert.Contains(t, result.Error, "json")
}

// --- debugWorkflowSingleRequestPhase errors ---

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

func TestDebugScrape_WithPostprocess(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><head><title>OrigTitle</title></head></html>`), nil
	}}
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
		Postprocess: &PostprocessSpec{
			Defaults: &DefaultsSpec{TitleLang: "ja"},
		},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
}

// --- DebugWorkflow: tryDebugWorkflowCandidate error paths ---

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

func TestDebugRequest_MultiRequest_HTTPError(t *testing.T) {
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
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}},
		}},
	}
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Attempts, 1)
	assert.NotEmpty(t, result.Attempts[0].Error)
}

func TestDebugRequest_MultiRequest_StatusRejected(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(500, "error"), nil
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
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}},
		}},
	}
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
	require.NotNil(t, result)
}

func TestDebugRequest_MultiRequest_DuplicateSkip(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><div>ok</div></body></html>`), nil
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
				Conditions: []string{`selector_exists(xpath("//div"))`},
			},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}},
		}},
	}
	result, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Attempts, 1)
	assert.True(t, result.Attempts[0].Matched)
}

// --- DebugWorkflow: singleRequestPhase status check failed ---

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

func TestDebugScrape_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestDebugWorkflow_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := DebugWorkflow(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestDebugRequest_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestDebugCase_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{Input: "ABC-123"})
	require.NoError(t, err)
	assert.False(t, result.Pass)
	assert.NotEmpty(t, result.Errmsg)
}

func TestDebugCase_CompileError_ExpectError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{Input: "ABC-123", Output: CaseOutput{Status: "error"}})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestCompileDraft_CompileError(t *testing.T) {
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := CompileDraft(spec)
	require.Error(t, err)
}

// --- captureHTTPResponse with decode error ---

func TestCaptureHTTPResponse_DecodeError(t *testing.T) {
	rsp := makeResponse(200, "hello")
	defer func() { _ = rsp.Body.Close() }()
	_, err := captureHTTPResponse(rsp, "unknown-charset")
	require.Error(t, err)
}

// --- previewBody truncation ---

func TestPreviewBody_Truncation(t *testing.T) {
	body := strings.Repeat("a", 5000)
	result := previewBody(body)
	assert.Len(t, result, 4000)
}

// --- debugWorkflowSearchSelect nil baseResp ---

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

func TestDebugScrape_TwoStep(t *testing.T) {
	callCount := 0
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return makeResponse(200, `<html><body><a href="/detail/1">link</a></body></html>`), nil
		}
		return makeResponse(200, `<html><body><h1 class="title">DetailTitle</h1></body></html>`), nil
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
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`}},
		}},
	}
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "DetailTitle", result.Meta.Title)
}

// --- DebugScrape with OnPrecheckResponse returning not found ---

func TestDebugScrape_PrecheckResponseNotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, `<html><body>not found</body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugScrape: multiRequest path through debugScrapeFetch ---

func TestDebugScrape_MultiRequest(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><div class="found">ok</div><h1 class="title">MRTitle</h1></body></html>`), nil
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
	result, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	assert.Equal(t, "MRTitle", result.Meta.Title)
}

// --- DebugScrape with OnPrecheckResponse not_found_status_codes ---

func TestDebugScrape_NotFoundStatusCode(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(302, "redirect"), nil
	}}
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}", NotFoundStatusCodes: []int{302}},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugWorkflow: precheck not matched ---

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

func twoStepWorkflowSpec(host string) *PluginSpec { //nolint:unparam
	return &PluginSpec{
		Version: 1,
		Name:    "test-twostep",
		Type:    "two-step",
		Hosts:   []string{host},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
				},
				Return:      "${item.link}",
				NextRequest: &RequestSpec{Method: "GET", Path: "${value}"},
			},
		},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{Kind: "xpath", Expr: `//h1/text()`},
					Parser:   ParserSpec{Kind: "string"},
				},
			},
		},
	}
}

// --- tryDebugMultiRequestCandidate: direct tests for error paths ---

func compileTestPlugin(t *testing.T, spec *PluginSpec) *SearchPlugin {
	t.Helper()
	plg, err := newCompiledPlugin(spec)
	require.NoError(t, err)
	return plg
}

func TestTryDebugMultiRequestCandidate_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
	}}
	plg := compileTestPlugin(t, multiRequestSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	attempt, resp := tryDebugMultiRequestCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.NotEmpty(t, attempt.Error)
	assert.Nil(t, resp)
}

func TestTryDebugMultiRequestCandidate_StatusRejected(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(500, "error"), nil
	}}
	spec := multiRequestSpec("https://example.com")
	spec.MultiRequest.Request.AcceptStatusCodes = []int{200}
	plg := compileTestPlugin(t, spec)
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	attempt, resp := tryDebugMultiRequestCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.NotEmpty(t, attempt.Error)
	assert.Nil(t, resp)
}

func TestTryDebugMultiRequestCandidate_SuccessWhenNotMatched(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><span>no match</span></body></html>`), nil
	}}
	plg := compileTestPlugin(t, multiRequestSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	attempt, _ := tryDebugMultiRequestCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.False(t, attempt.Matched)
}

func TestTryDebugMultiRequestCandidate_Success(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><div class="found">ok</div></body></html>`), nil
	}}
	plg := compileTestPlugin(t, multiRequestSpec("https://example.com"))
	evalCtx := &evalContext{number: "ABC-123", host: "https://example.com"}
	attempt, resp := tryDebugMultiRequestCandidate(context.Background(), cli, plg, "ABC-123", evalCtx)
	assert.True(t, attempt.Matched)
	assert.NotNil(t, resp)
}

// --- tryDebugWorkflowCandidate: direct tests ---

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

func TestCaptureHTTPResponse_ReadError(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(&errorReader{}),
		Header:     make(http.Header),
	}
	_, err := captureHTTPResponse(rsp, "")
	require.Error(t, err)
}

type errorReader struct{}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

// --- debugWorkflowSingleRequestPhase: captureHTTPResponse error ---

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

func TestFieldByName_NoMatch(t *testing.T) {
	plg := compileTestPlugin(t, simpleOneStepSpec("https://example.com"))
	f := plg.fieldByName("nonexistent_field")
	assert.Nil(t, f)
}
