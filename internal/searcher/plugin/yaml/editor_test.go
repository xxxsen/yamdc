package yaml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestEditorContextNumberPropagation(t *testing.T) {
	// mock yaml plugin spec
	rawSpec := &PluginSpec{
		Version: 1,
		Name:    "test_propagation",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method:            "GET",
			Path:              "/search/${number}", // Expecting the context number to be populated here
			AcceptStatusCodes: []int{200},
			Response: &ResponseSpec{
				DecodeCharset: "utf-8",
			},
		},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
				},
				Return: "${item.link}",
				NextRequest: &RequestSpec{
					Method:            "GET",
					Path:              "/detail/${number}",
					AcceptStatusCodes: []int{200},
				},
			},
		},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{
						Kind: "xpath",
						Expr: "//title/text()",
					},
					Parser: ParserSpec{
						Kind: "string",
					},
				},
			},
		},
	}

	cli := &testHTTPClient{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			body := "<html><title>Test: " + req.URL.Path + "</title><body><a href=\"/link\">link</a></body></html>"
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte(body))),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		},
	}

	numberToTest := "TEST-A-123"

	t.Run("DebugRequest", func(t *testing.T) {
		res, err := DebugRequest(context.Background(), cli, rawSpec, numberToTest)
		if err != nil {
			t.Fatalf("DebugRequest error: %v", err)
		}
		if res.Request.URL != "https://example.com/search/TEST-A-123" {
			t.Errorf("expected URL https://example.com/search/TEST-A-123, got: %s", res.Request.URL)
		}
	})

	t.Run("DebugScrape", func(t *testing.T) {
		res, err := DebugScrape(context.Background(), cli, rawSpec, numberToTest)
		if err != nil {
			t.Fatalf("DebugScrape error: %v", err)
		}
		if res.Error != "" {
			t.Fatalf("DebugScrape returned error field: %s", res.Error)
		}
		if res.Request.URL != "https://example.com/detail/TEST-A-123" {
			t.Errorf("expected URL https://example.com/detail/TEST-A-123, got: %s", res.Request.URL)
		}
		// Confirm title field captured the populated URL
		f, ok := res.Fields["title"]
		if !ok || len(f.SelectorValues) == 0 || f.SelectorValues[0] != "Test: /detail/TEST-A-123" {
			t.Errorf("expected title 'Test: /detail/TEST-A-123', got: %+v", f)
		}
	})

	t.Run("DebugWorkflow", func(t *testing.T) {
		res, err := DebugWorkflow(context.Background(), cli, rawSpec, numberToTest)
		if err != nil {
			t.Fatalf("DebugWorkflow error: %v", err)
		}
		if res.Error != "" {
			t.Fatalf("DebugWorkflow returned error field: %s", res.Error)
		}
		if len(res.Steps) == 0 {
			t.Fatalf("DebugWorkflow returned 0 steps")
		}
		req := res.Steps[0].Request
		if req == nil {
			t.Fatalf("Step 0 request is nil")
		}
		if req.URL != "https://example.com/search/TEST-A-123" {
			t.Errorf("expected URL https://example.com/search/TEST-A-123, got: %s", req.URL)
		}
	})
}

func TestCompileDraft(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	result, err := CompileDraft(spec)
	require.NoError(t, err)
	require.Contains(t, result.YAML, "version: 1")
	assert.True(t, result.Summary.HasRequest)
}

func TestCompileDraft_Error(t *testing.T) {
	_, err := CompileDraft(&PluginSpec{Version: 0})
	require.Error(t, err)
}

// --- helpers ---

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

func TestDebugRequest_SingleRequest_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugScrape with response treated as not found ---

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

func TestDebugRequest_MultiRequest_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
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

func TestDebugRequest_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := DebugRequest(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

func TestCompileDraft_CompileError(t *testing.T) {
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := CompileDraft(spec)
	require.Error(t, err)
}

// --- captureHTTPResponse with decode error ---

func TestTryDebugMultiRequestCandidate_HTTPError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
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

func TestDebugRequest_SingleRequest_OKResponse(t *testing.T) {
	htmlBody := `<html><body><h1>Test</h1></body></html>`
	cli := &testHTTPClient{
		roundTrip: func(_ *http.Request) (*http.Response, error) {
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

func TestIterateMultiRequestCandidates_UniqueDedup(t *testing.T) {
	spec := multiRequestSpec("https://example.com")
	spec.MultiRequest.Unique = true
	spec.MultiRequest.Candidates = []string{"ABC-123", "ABC-123", "ABC-123"}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}

	callCount := 0
	cli := &testHTTPClient{
		roundTrip: func(_ *http.Request) (*http.Response, error) {
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
