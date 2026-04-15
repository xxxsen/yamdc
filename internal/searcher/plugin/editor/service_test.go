package editor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/client"
	plugyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
)

func TestServiceCompile(t *testing.T) {
	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.Compile(context.Background(), oneStepDraft("https://fixture.example"))
	require.NoError(t, err)
	require.Contains(t, result.YAML, "version: 1")
	require.Equal(t, "html", result.Summary.ScrapeFormat)
	require.Equal(t, 2, result.Summary.FieldCount)
	require.True(t, result.Summary.HasRequest)
	require.False(t, result.Summary.HasMultiRequest)
}

func TestServiceRequestDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/search/ABC-123", r.URL.Path)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.RequestDebug(context.Background(), oneStepDraft(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, result.Request.Method)
	require.Equal(t, srv.URL+"/search/ABC-123", result.Request.URL)
	require.NotNil(t, result.Response)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Contains(t, result.Response.Body, "ok")
}

func TestServiceScrapeDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`
<html><body>
  <h1 class="title"> Sample Title </h1>
  <div class="actors"><span>Alice</span><span> Bob </span></div>
</body></html>`))
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.ScrapeDebug(context.Background(), oneStepDraft(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result.Meta)
	require.Equal(t, "Sample Title", result.Meta.Title)
	require.Equal(t, []string{"Alice", "Bob"}, result.Meta.Actors)
	require.Contains(t, result.Fields, "title")
	require.Contains(t, result.Fields, "actors")
	require.Equal(t, "Sample Title", result.Fields["title"].ParserResult)
}

func TestServiceWorkflowDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`<html><body><a class="item" href="/detail/1">ABC-123 title</a></body></html>`))
		case "/detail/1":
			_, _ = w.Write([]byte(`<html><body><h1 class="title">Target</h1></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.WorkflowDebug(context.Background(), twoStepDraft(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.Len(t, result.Steps, 3)
	require.Equal(t, "request", result.Steps[0].Stage)
	require.Equal(t, "search_select", result.Steps[1].Stage)
	require.Equal(t, "/detail/1", result.Steps[1].SelectedValue)
	require.Equal(t, "next_request", result.Steps[2].Stage)
}

func TestServiceWorkflowDebugWithPrecheckVariablesAndNextRequestURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`<html><body><a class="item" href="/detail/1">abc-123 title</a></body></html>`))
		case "/detail/1":
			_, _ = w.Write([]byte(`<html><body><h1 class="title">Target</h1></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.WorkflowDebug(context.Background(), twoStepDraftWithURLAndPrecheckVars(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.Len(t, result.Steps, 3)
	require.Equal(t, "search_select", result.Steps[1].Stage)
	require.Equal(t, "/detail/1", result.Steps[1].SelectedValue)
	require.Equal(t, "next_request", result.Steps[2].Stage)
	require.Equal(t, srv.URL+"/detail/1", result.Steps[2].Request.URL)
}

func TestServiceWorkflowDebugRejectsInitialNonAcceptedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<html><body><a class="item" href="/detail/1">ABC-123 title</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.WorkflowDebug(context.Background(), twoStepDraft(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Error, "status code")
	require.Contains(t, result.Error, "404")
	require.Len(t, result.Steps, 1)
	require.Equal(t, "request", result.Steps[0].Stage)
	require.Equal(t, http.StatusNotFound, result.Steps[0].Response.StatusCode)
}

func TestServiceWorkflowDebugRejectsNextRequestNonAcceptedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`<html><body><a class="item" href="/detail/1">ABC-123 title</a></body></html>`))
		case "/detail/1":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<html><body><h1 class="title">Target</h1></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.WorkflowDebug(context.Background(), twoStepDraft(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Error, "status code")
	require.Contains(t, result.Error, "404")
	require.Len(t, result.Steps, 3)
	require.Equal(t, "next_request", result.Steps[2].Stage)
	require.Equal(t, http.StatusNotFound, result.Steps[2].Response.StatusCode)
}

func TestServiceWorkflowDebugReturnsStepDetailsWhenSearchSelectMatchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`<html><body><a class="item" href="/detail/1">OTHER-999 title</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.WorkflowDebug(context.Background(), twoStepDraft(srv.URL), "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Error, "search_select matched count mismatch")
	require.Len(t, result.Steps, 2)
	require.Equal(t, "request", result.Steps[0].Stage)
	require.Equal(t, "search_select", result.Steps[1].Stage)
	require.NotEmpty(t, result.Steps[1].Selectors)
	require.Len(t, result.Steps[1].Items, 1)
	require.False(t, result.Steps[1].Items[0].Matched)
}

func TestServiceCaseDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1 class="title"> Sample Title </h1><div class="actors"><span>Alice</span></div></body></html>`))
	}))
	defer srv.Close()

	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.CaseDebug(context.Background(), oneStepDraft(srv.URL), plugyaml.CaseSpec{
		Name:  "case-1",
		Input: "ABC-123",
		Output: plugyaml.CaseOutput{
			Title:    "Sample Title",
			ActorSet: []string{"Alice"},
			Status:   "success",
		},
	})
	require.NoError(t, err)
	require.True(t, result.Pass)
	require.NotNil(t, result.Meta)
}

func TestServiceImportYAML(t *testing.T) {
	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.ImportYAML(context.Background(), `
version: 1
name: fixture
type: one-step
hosts:
  - https://fixture.example
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
`)
	require.NoError(t, err)
	require.Equal(t, "fixture", result.Name)
	require.Equal(t, "one-step", result.Type)
	require.Equal(t, "html", result.Scrape.Format)
}

func TestServiceImportYAMLPreservesPrecheckVariablesAndURLRequests(t *testing.T) {
	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.ImportYAML(context.Background(), `
version: 1
name: fixture-two-step
type: two-step
hosts:
  - https://fixture.example
precheck:
  variables:
    clean_number: ${clean_number(${number})}
request:
  method: GET
  path: /search
workflow:
  search_select:
    selectors:
      - name: read_link
        kind: xpath
        expr: //a/@href
    match:
      mode: and
      conditions:
        - contains("${vars.clean_number}", "${number}")
    return: ${item.read_link}
    next_request:
      method: GET
      url: ${build_url(${host}, ${value})}
      accept_status_codes: [200]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true
`)
	require.NoError(t, err)
	require.Equal(t, "${clean_number(${number})}", result.Precheck.Variables["clean_number"])
	require.Equal(t, "${build_url(${host}, ${value})}", result.Workflow.SearchSelect.NextRequest.URL)
	require.Empty(t, result.Workflow.SearchSelect.NextRequest.Path)
}

func TestServiceCompilePreservesPrecheckVariablesAndURLRequests(t *testing.T) {
	svc, err := NewService(client.MustNewClient())
	require.NoError(t, err)
	result, err := svc.Compile(context.Background(), twoStepDraftWithURLAndPrecheckVars("https://fixture.example"))
	require.NoError(t, err)
	require.Contains(t, result.YAML, "clean_number: ${clean_number(${number})}")
	require.Contains(t, result.YAML, "url: ${build_url(${host}, ${value})}")
}

func oneStepDraft(host string) *plugyaml.PluginSpec {
	return &plugyaml.PluginSpec{
		Version: 1,
		Name:    "fixture",
		Type:    "one-step",
		Hosts:   []string{host},
		Request: &plugyaml.RequestSpec{
			Method: "GET",
			Path:   "/search/${number}",
		},
		Scrape: &plugyaml.ScrapeSpec{
			Format: "html",
			Fields: map[string]*plugyaml.FieldSpec{
				"title": {
					Selector: &plugyaml.SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`},
					Transforms: []*plugyaml.TransformSpec{
						{Kind: "trim"},
					},
					Parser:   plugyaml.ParserSpec{Kind: "string"},
					Required: true,
				},
				"actors": {
					Selector: &plugyaml.SelectorSpec{Kind: "xpath", Expr: `//div[@class="actors"]/span/text()`, Multi: true},
					Transforms: []*plugyaml.TransformSpec{
						{Kind: "map_trim"},
						{Kind: "remove_empty"},
					},
					Parser:   plugyaml.ParserSpec{Kind: "string_list"},
					Required: true,
				},
			},
		},
		Postprocess: &plugyaml.PostprocessSpec{
			Defaults: &plugyaml.DefaultsSpec{TitleLang: strings.ToLower("EN")},
		},
	}
}

func twoStepDraft(host string) *plugyaml.PluginSpec {
	return &plugyaml.PluginSpec{
		Version: 1,
		Name:    "fixture-two-step",
		Type:    "two-step",
		Hosts:   []string{host},
		Request: &plugyaml.RequestSpec{
			Method: "GET",
			Path:   "/search",
		},
		Workflow: &plugyaml.WorkflowSpec{
			SearchSelect: &plugyaml.SearchSelectWorkflowSpec{
				Selectors: []*plugyaml.SelectorListSpec{
					{Name: "read_link", Kind: "xpath", Expr: `//a[@class="item"]/@href`},
					{Name: "read_title", Kind: "xpath", Expr: `//a[@class="item"]/text()`},
				},
				Match: &plugyaml.ConditionGroupSpec{
					Mode:        "and",
					Conditions:  []string{`contains("${item.read_title}", "${number}")`},
					ExpectCount: 1,
				},
				Return:      `${item.read_link}`,
				NextRequest: &plugyaml.RequestSpec{Method: "GET", Path: `${build_url(${host}, ${value})}`},
			},
		},
		Scrape: &plugyaml.ScrapeSpec{
			Format: "html",
			Fields: map[string]*plugyaml.FieldSpec{
				"title": {
					Selector: &plugyaml.SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`},
					Transforms: []*plugyaml.TransformSpec{
						{Kind: "trim"},
					},
					Parser:   plugyaml.ParserSpec{Kind: "string"},
					Required: true,
				},
			},
		},
	}
}

func twoStepDraftWithURLAndPrecheckVars(host string) *plugyaml.PluginSpec {
	return &plugyaml.PluginSpec{
		Version: 1,
		Name:    "fixture-two-step-url",
		Type:    "two-step",
		Hosts:   []string{host},
		Precheck: &plugyaml.PrecheckSpec{
			Variables: map[string]string{
				"clean_number": `${clean_number(${number})}`,
			},
		},
		Request: &plugyaml.RequestSpec{
			Method: "GET",
			Path:   "/search",
		},
		Workflow: &plugyaml.WorkflowSpec{
			SearchSelect: &plugyaml.SearchSelectWorkflowSpec{
				Selectors: []*plugyaml.SelectorListSpec{
					{Name: "read_link", Kind: "xpath", Expr: `//a[@class="item"]/@href`},
					{Name: "read_title", Kind: "xpath", Expr: `//a[@class="item"]/text()`},
				},
				ItemVariables: map[string]string{
					"clean_title": `${clean_number(${to_upper(${item.read_title})})}`,
				},
				Match: &plugyaml.ConditionGroupSpec{
					Mode:        "and",
					Conditions:  []string{`contains("${item_variables.clean_title}", "${vars.clean_number}")`},
					ExpectCount: 1,
				},
				Return: `${item.read_link}`,
				NextRequest: &plugyaml.RequestSpec{
					Method: "GET",
					URL:    `${build_url(${host}, ${value})}`,
				},
			},
		},
		Scrape: &plugyaml.ScrapeSpec{
			Format: "html",
			Fields: map[string]*plugyaml.FieldSpec{
				"title": {
					Selector: &plugyaml.SelectorSpec{Kind: "xpath", Expr: `//h1[@class="title"]/text()`},
					Transforms: []*plugyaml.TransformSpec{
						{Kind: "trim"},
					},
					Parser:   plugyaml.ParserSpec{Kind: "string"},
					Required: true,
				},
			},
		},
	}
}
