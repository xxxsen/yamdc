package yaml

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	"golang.org/x/net/html"
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

func mustCompilePlugin(t *testing.T, yamlStr string) *SearchPlugin {
	t.Helper()
	plg, err := NewFromBytes([]byte(yamlStr))
	require.NoError(t, err)
	return plg.(*SearchPlugin)
}

func minimalOneStepYAML() string {
	return `
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
}

// --- nopCloser helper ---

func nopCloser(data []byte) io.ReadCloser {
	return io.NopCloser(strings.NewReader(string(data)))
}

func parseHTML(s string) (*html.Node, error) {
	return html.Parse(strings.NewReader(s))
}

func buildTestPlugin(t *testing.T, spec *PluginSpec) *SearchPlugin {
	t.Helper()
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	return &SearchPlugin{spec: compiled}
}

func mustCompileTemplate(t *testing.T, raw string) *template {
	t.Helper()
	tmpl, err := compileTemplate(raw)
	require.NoError(t, err)
	return tmpl
}

func helperParseHTMLStr(t *testing.T, s string) *html.Node {
	t.Helper()
	n, err := htmlquery.Parse(strings.NewReader(s))
	require.NoError(t, err)
	return n
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
