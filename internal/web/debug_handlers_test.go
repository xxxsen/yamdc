package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/searcher"
	plugineditor "github.com/xxxsen/yamdc/internal/searcher/plugin/editor"
	plugyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestHandleMovieIDCleanerExplain(t *testing.T) {
	tests := []struct {
		name     string
		cleaner  movieidcleaner.Cleaner
		body     string
		wantCode int
	}{
		{"nil cleaner", nil, `{"input":"abc"}`, errCodeMovieIDCleanerUnavailable},
		{"invalid json", &stubCleaner{}, `{bad`, errCodeInvalidJSONBody},
		{"empty input", &stubCleaner{}, `{"input":""}`, errCodeInputRequired},
		{"whitespace input", &stubCleaner{}, `{"input":"  "}`, errCodeInputRequired},
		{"explain error", &stubCleaner{explainErr: fmt.Errorf("boom")}, `{"input":"abc"}`, errCodeMovieIDCleanerExplainFailed},
		{"success", &stubCleaner{explainResult: &movieidcleaner.ExplainResult{
			Input: "abc",
			Final: &movieidcleaner.Result{NumberID: "ABC-123", Status: "success"},
		}}, `{"input":"abc"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &API{cleaner: tt.cleaner}
			c, rec := newGinContext(http.MethodPost, "/api/debug/movieid-cleaner/explain", strings.NewReader(tt.body))
			api.handleMovieIDCleanerExplain(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleSearcherDebugPlugins(t *testing.T) {
	tests := []struct {
		name     string
		debugger *API
		wantCode int
	}{
		{"nil debugger", &API{}, errCodeSearcherDebuggerUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodGet, "/api/debug/searcher/plugins", nil)
			tt.debugger.handleSearcherDebugPlugins(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleSearcherDebugSearch(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		body     string
		wantCode int
	}{
		{"nil debugger", &API{}, `{"input":"abc"}`, errCodeSearcherDebuggerUnavailable},
		{"invalid json", &API{debugger: searcher.NewDebugger(client.MustNewClient(), store.NewMemStorage(), movieidcleaner.NewPassthroughCleaner(), nil, nil)}, `{bad`, errCodeInvalidJSONBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/debug/searcher/search", strings.NewReader(tt.body))
			tt.api.handleSearcherDebugSearch(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleSearcherDebugPluginsWithDebugger(t *testing.T) {
	debugger := searcher.NewDebugger(client.MustNewClient(), store.NewMemStorage(), movieidcleaner.NewPassthroughCleaner(), nil, nil)
	api := &API{debugger: debugger}
	c, rec := newGinContext(http.MethodGet, "/api/debug/searcher/plugins", nil)
	api.handleSearcherDebugPlugins(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleSearcherDebugSearchWithDebugger(t *testing.T) {
	debugger := searcher.NewDebugger(client.MustNewClient(), store.NewMemStorage(), movieidcleaner.NewPassthroughCleaner(), nil, nil)
	api := &API{debugger: debugger}
	c, rec := newGinContext(http.MethodPost, "/api/debug/searcher/search", strings.NewReader(`{"input":"ABC-123"}`))
	api.handleSearcherDebugSearch(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandlePluginEditorCompile(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)

	tests := []struct {
		name     string
		api      *API
		body     string
		wantCode int
	}{
		{"nil editor", &API{}, `{"draft":{}}`, errCodePluginEditorUnavailable},
		{"invalid json", &API{editor: editorSvc}, `{bad`, errCodeInvalidJSONBody},
		{"nil draft", &API{editor: editorSvc}, `{}`, errCodeInputRequired},
		{"success", &API{editor: editorSvc}, `{
			"draft":{"version":1,"name":"fixture","type":"one-step","hosts":["https://fixture.example"],"request":{"method":"GET","path":"/search/${number}"},"scrape":{"format":"html","fields":{"title":{"selector":{"kind":"xpath","expr":"//title/text()"},"parser":"string","required":true}}}}
		}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/compile", strings.NewReader(tt.body))
			tt.api.handlePluginEditorCompile(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandlePluginEditorImport(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)

	tests := []struct {
		name     string
		api      *API
		body     string
		wantCode int
	}{
		{"nil editor", &API{}, `{"yaml":"x"}`, errCodePluginEditorUnavailable},
		{"invalid json", &API{editor: editorSvc}, `{bad`, errCodeInvalidJSONBody},
		{"empty yaml", &API{editor: editorSvc}, `{"yaml":""}`, errCodeInputRequired},
		{"whitespace yaml", &API{editor: editorSvc}, `{"yaml":"  "}`, errCodeInputRequired},
		{"success", &API{editor: editorSvc}, `{"yaml":"version: 1\nname: fixture\ntype: one-step\nhosts:\n  - https://fixture.example\nrequest:\n  method: GET\n  path: /search/${number}\nscrape:\n  format: html\n  fields:\n    title:\n      selector:\n        kind: xpath\n        expr: //title/text()\n      parser: string\n      required: true\n"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/import", strings.NewReader(tt.body))
			tt.api.handlePluginEditorImport(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandlePluginEditorDraftNumberOp(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)

	tests := []struct {
		name     string
		api      *API
		body     string
		wantCode int
	}{
		{"nil editor", &API{}, `{"draft":{},"number":"ABC"}`, errCodePluginEditorUnavailable},
		{"invalid json", &API{editor: editorSvc}, `{bad`, errCodeInvalidJSONBody},
		{"nil draft", &API{editor: editorSvc}, `{"number":"ABC"}`, errCodeInputRequired},
		{"empty number", &API{editor: editorSvc}, `{"draft":{"version":1},"number":""}`, errCodeInputRequired},
		{"whitespace number", &API{editor: editorSvc}, `{"draft":{"version":1},"number":"  "}`, errCodeInputRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/request", strings.NewReader(tt.body))
			tt.api.handlePluginEditorDraftNumberOp(c, "request", errCodePluginEditorRequestFailed, func(_ context.Context, _ *plugyaml.PluginSpec, _ string) (interface{}, error) {
				return nil, nil //nolint:nilnil
			})
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandlePluginEditorRequest(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/request", strings.NewReader(`{"draft":{}}`))
	api.handlePluginEditorRequest(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodePluginEditorUnavailable, resp.Code)
}

func TestHandlePluginEditorScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1 class="title"> Sample Title </h1></body></html>`))
	}))
	defer srv.Close()

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	body := fmt.Sprintf(`{"number":"ABC-123","draft":{"version":1,"name":"fixture","type":"one-step","hosts":["%s"],"request":{"method":"GET","path":"/search/${number}"},"scrape":{"format":"html","fields":{"title":{"selector":{"kind":"xpath","expr":"//h1[@class='title']/text()"},"transforms":[{"kind":"trim"}],"parser":"string","required":true}}}}}`, srv.URL)
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/scrape", strings.NewReader(body))
	api.handlePluginEditorScrape(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandlePluginEditorWorkflow(t *testing.T) {
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

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	body := fmt.Sprintf(`{"number":"ABC-123","draft":{"version":1,"name":"fixture-two-step","type":"two-step","hosts":["%s"],"request":{"method":"GET","path":"/search"},"workflow":{"search_select":{"selectors":[{"name":"read_link","kind":"xpath","expr":"//a[@class='item']/@href"},{"name":"read_title","kind":"xpath","expr":"//a[@class='item']/text()"}],"match":{"mode":"and","conditions":["contains(\"${item.read_title}\", \"${number}\")"],"expect_count":1},"return":"${item.read_link}","next_request":{"method":"GET","path":"${build_url(${host}, ${value})}"}}},"scrape":{"format":"html","fields":{"title":{"selector":{"kind":"xpath","expr":"//h1/text()"},"parser":"string","required":true}}}}}`, srv.URL)
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/workflow", strings.NewReader(body))
	api.handlePluginEditorWorkflow(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandlePluginEditorCase(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)

	tests := []struct {
		name     string
		api      *API
		body     string
		wantCode int
	}{
		{"nil editor", &API{}, `{"draft":{},"case":{}}`, errCodePluginEditorUnavailable},
		{"invalid json", &API{editor: editorSvc}, `{bad`, errCodeInvalidJSONBody},
		{"nil draft", &API{editor: editorSvc}, `{"case":{"name":"c1","input":"x","output":{}}}`, errCodeInputRequired},
		{"nil case", &API{editor: editorSvc}, `{"draft":{"version":1}}`, errCodeInputRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/case", strings.NewReader(tt.body))
			tt.api.handlePluginEditorCase(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandlePluginEditorCaseSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1 class="title"> Sample Title </h1><div class="actors"><span>Alice</span></div></body></html>`))
	}))
	defer srv.Close()

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	body := fmt.Sprintf(`{"draft":{"version":1,"name":"fixture","type":"one-step","hosts":["%s"],"request":{"method":"GET","path":"/search/${number}"},"scrape":{"format":"html","fields":{"title":{"selector":{"kind":"xpath","expr":"//h1[@class='title']/text()"},"transforms":[{"kind":"trim"}],"parser":"string","required":true},"actors":{"selector":{"kind":"xpath","expr":"//div[@class='actors']/span/text()","multi":true},"parser":"string_list","required":false}}}},"case":{"name":"case-1","input":"ABC-123","output":{"title":"Sample Title","actor_set":["Alice"],"status":"success"}}}`, srv.URL)
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/case", strings.NewReader(body))
	api.handlePluginEditorCase(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleHandlerDebugHandlers(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		wantCode int
	}{
		{"nil handlers", &API{}, errCodeHandlerDebuggerUnavailable},
		{"with handlers", &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), movieidcleaner.NewPassthroughCleaner(), []string{}, map[string]phandler.DebugHandlerOption{})}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodGet, "/api/debug/handlers", nil)
			tt.api.handleHandlerDebugHandlers(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleHandlerDebugRun(t *testing.T) {
	tests := []struct {
		name     string
		api      *API
		body     string
		wantCode int
	}{
		{"nil handlers", &API{}, `{}`, errCodeHandlerDebuggerUnavailable},
		{"invalid json", &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), movieidcleaner.NewPassthroughCleaner(), []string{"number_title"}, map[string]phandler.DebugHandlerOption{})}, `{bad`, errCodeInvalidJSONBody},
		{"success", &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), movieidcleaner.NewPassthroughCleaner(), []string{"number_title"}, map[string]phandler.DebugHandlerOption{})}, `{"mode":"single","handler_id":"number_title","meta":{"number":"ABC-123","title":"sample title"}}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newGinContext(http.MethodPost, "/api/debug/handler/run", strings.NewReader(tt.body))
			tt.api.handleHandlerDebugRun(c)
			resp := decodeResponse(t, rec)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestHandleHandlerDebugRunChain(t *testing.T) {
	phandler.Register("test_chain_fail_v2", func(_ interface{}, _ appdeps.Runtime) (phandler.IHandler, error) {
		return testHandlerFn(func(_ context.Context, fc *model.FileContext) error {
			fc.Meta.Title += "-failed"
			return fmt.Errorf("boom")
		}), nil
	})
	phandler.Register("test_chain_ok_v2", func(_ interface{}, _ appdeps.Runtime) (phandler.IHandler, error) {
		return testHandlerFn(func(_ context.Context, fc *model.FileContext) error {
			fc.Meta.Title += "-ok"
			return nil
		}), nil
	})

	api := &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), movieidcleaner.NewPassthroughCleaner(), []string{"test_chain_fail_v2", "test_chain_ok_v2"}, map[string]phandler.DebugHandlerOption{})}
	c, rec := newGinContext(http.MethodPost, "/api/debug/handler/run", strings.NewReader(`{"mode":"chain","handler_ids":["test_chain_fail_v2","test_chain_ok_v2"],"meta":{"number":"ABC-123","title":"sample title"}}`))
	api.handleHandlerDebugRun(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleSearcherDebugSearchError(t *testing.T) {
	debugger := searcher.NewDebugger(client.MustNewClient(), store.NewMemStorage(), movieidcleaner.NewPassthroughCleaner(), nil, nil)
	api := &API{debugger: debugger}
	c, rec := newGinContext(http.MethodPost, "/api/debug/searcher/search", strings.NewReader(`{"input":"ABC-123","plugins":["nonexistent_plugin"]}`))
	api.handleSearcherDebugSearch(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeSearcherDebugSearchFailed, resp.Code)
}

func TestHandlePluginEditorCompileError(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/compile", strings.NewReader(`{"draft":{"version":999}}`))
	api.handlePluginEditorCompile(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodePluginEditorCompileFailed, resp.Code)
}

func TestHandlePluginEditorImportError(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/import", strings.NewReader(`{"yaml":"invalid yaml: [unclosed"}`))
	api.handlePluginEditorImport(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodePluginEditorImportFailed, resp.Code)
}

func TestHandlePluginEditorDraftNumberOpError(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	c, rec := newGinContext(http.MethodPost, "/test", strings.NewReader(`{"draft":{"version":1},"number":"ABC-123"}`))
	api.handlePluginEditorDraftNumberOp(c, "test_op", errCodePluginEditorRequestFailed, func(_ context.Context, _ *plugyaml.PluginSpec, _ string) (interface{}, error) {
		return nil, fmt.Errorf("op failed")
	})
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodePluginEditorRequestFailed, resp.Code)
}

func TestHandlePluginEditorDraftNumberOpSuccess(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	c, rec := newGinContext(http.MethodPost, "/test", strings.NewReader(`{"draft":{"version":1},"number":"ABC-123"}`))
	api.handlePluginEditorDraftNumberOp(c, "test_op", errCodePluginEditorRequestFailed, func(_ context.Context, _ *plugyaml.PluginSpec, _ string) (interface{}, error) {
		return map[string]string{"ok": "true"}, nil
	})
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandlePluginEditorCaseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>Wrong</h1></body></html>`))
	}))
	defer srv.Close()

	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	body := fmt.Sprintf(`{
		"draft":{"version":1,"name":"bad","type":"one-step","hosts":["%s"],"request":{"method":"GET","path":"/${number}"},"scrape":{"format":"html","fields":{"title":{"selector":{"kind":"xpath","expr":"//h1/text()"},"parser":"string","required":true}}}},
		"case":{"name":"c1","input":"X","output":{"title":"Expected","status":"success"}}
	}`, srv.URL)
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/case", strings.NewReader(body))
	api.handlePluginEditorCase(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
}

func TestHandleHandlerDebugRunError(t *testing.T) {
	api := &API{handlers: phandler.NewDebugger(phandlerDebugRuntime(), movieidcleaner.NewPassthroughCleaner(), []string{"number_title"}, map[string]phandler.DebugHandlerOption{})}
	c, rec := newGinContext(http.MethodPost, "/api/debug/handler/run", strings.NewReader(`{"mode":"single","handler_id":"nonexistent_handler","meta":{"number":"X"}}`))
	api.handleHandlerDebugRun(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, errCodeHandlerDebugRunFailed, resp.Code)
}

func TestHandlePluginEditorRequestWithEditor(t *testing.T) {
	editorSvc, err := plugineditor.NewService(client.MustNewClient())
	require.NoError(t, err)
	api := &API{editor: editorSvc}
	c, rec := newGinContext(http.MethodPost, "/api/debug/plugin-editor/request", strings.NewReader(`{"number":"ABC","draft":{"version":1,"name":"x","type":"one-step","hosts":["http://localhost:1"],"request":{"method":"GET","path":"/"},"scrape":{"format":"html","fields":{"title":{"selector":{"kind":"xpath","expr":"//x"},"parser":"string"}}}}}`))
	api.handlePluginEditorRequest(c)
	resp := decodeResponse(t, rec)
	assert.NotEqual(t, errCodePluginEditorUnavailable, resp.Code)
}
