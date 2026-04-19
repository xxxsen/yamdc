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
)

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

func TestDebugScrape_NotFoundResponse(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

// --- DebugWorkflow multi_request with workflow - all candidates fail ---

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

func TestDebugScrape_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
	require.Error(t, err)
}

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
		return nil, errors.New("network error")
	}}
	spec := simpleOneStepSpec("https://example.com")
	_, err := DebugScrape(context.Background(), cli, spec, "ABC-123")
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
