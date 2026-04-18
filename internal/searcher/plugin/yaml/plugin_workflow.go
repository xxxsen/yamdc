package yaml

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"golang.org/x/net/html"
)

func (w *compiledSearchSelectWorkflow) handleRequest(
	ctx context.Context,
	plg *SearchPlugin,
	invoker pluginapi.HTTPInvoker,
	req *http.Request,
	evalCtx *evalContext,
) (*http.Response, error) {
	rsp, err := invoker(ctx, req)
	if err != nil {
		return nil, err
	}
	return w.handleResponse(ctx, plg, invoker, rsp, evalCtx, plg.spec.request.decodeCharset)
}

func checkBaseResponseStatus(plg *SearchPlugin, statusCode int) error {
	if pReq := plg.spec.request; pReq != nil {
		return checkAcceptedStatus(pReq, statusCode)
	}
	if pReq := plg.spec.multiRequest; pReq != nil {
		return checkAcceptedStatus(pReq.request, statusCode)
	}
	return nil
}

func collectSelectorResults(node *html.Node, selectors []*compiledSelectorList) (map[string][]string, int, error) {
	results := make(map[string][]string, len(selectors))
	expectedLen := -1
	for _, sel := range selectors {
		items := decoder.DecodeList(node, sel.expr)
		results[sel.name] = items
		if expectedLen == -1 {
			expectedLen = len(items)
			continue
		}
		if expectedLen != len(items) {
			return results, expectedLen, errSelectorCountMismatch
		}
	}
	return results, expectedLen, nil
}

func (w *compiledSearchSelectWorkflow) matchItems(
	evalCtx *evalContext, results map[string][]string, body string, node *html.Node, expectedLen int,
) ([]*evalContext, error) {
	matched := make([]*evalContext, 0, expectedLen)
	for i := 0; i < expectedLen; i++ {
		itemCtx := &evalContext{
			number:        evalCtx.number,
			host:          evalCtx.host,
			body:          body,
			vars:          evalCtx.vars,
			item:          make(map[string]string, len(results)),
			itemVariables: make(map[string]string, len(w.itemVariables)),
		}
		for name, lst := range results {
			itemCtx.item[name] = lst[i]
		}
		for key, tmpl := range w.itemVariables {
			v, err := tmpl.Render(itemCtx)
			if err != nil {
				return nil, err
			}
			itemCtx.itemVariables[key] = v
		}
		ok, err := w.match.Eval(itemCtx, node)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, itemCtx)
		}
	}
	return matched, nil
}

func (w *compiledSearchSelectWorkflow) followNextRequest(
	ctx context.Context, plg *SearchPlugin, invoker pluginapi.HTTPInvoker, itemCtx *evalContext,
) (*http.Response, error) {
	value, err := w.ret.Render(itemCtx)
	if err != nil {
		return nil, err
	}
	itemCtx.value = value
	nextReq, err := plg.buildRequest(ctx, w.nextRequest, itemCtx)
	if err != nil {
		return nil, err
	}
	pluginapi.SetContainerValue(ctx, ctxKeyRequestPath, nextReq.URL.String())
	nextRsp, err := invoker(ctx, nextReq)
	if err != nil {
		return nil, err
	}
	if err := checkAcceptedStatus(w.nextRequest, nextRsp.StatusCode); err != nil {
		_ = nextRsp.Body.Close()
		return nil, err
	}
	pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, nextReq.URL.String())
	return nextRsp, nil
}

func (w *compiledSearchSelectWorkflow) handleResponse(
	ctx context.Context,
	plg *SearchPlugin,
	invoker pluginapi.HTTPInvoker,
	rsp *http.Response,
	evalCtx *evalContext,
	decodeCharset string,
) (*http.Response, error) {
	body, node, err := readResponseBody(rsp, decodeCharset)
	if err != nil {
		return nil, err
	}
	if err := checkBaseResponseStatus(plg, rsp.StatusCode); err != nil {
		return nil, err
	}
	results, expectedLen, err := collectSelectorResults(node, w.selectors)
	if err != nil {
		return nil, err
	}
	matched, err := w.matchItems(evalCtx, results, body, node, expectedLen)
	if err != nil {
		return nil, err
	}
	if w.match != nil && w.match.expectCount > 0 && len(matched) != w.match.expectCount {
		return nil, fmt.Errorf("got %d expect %d: %w", len(matched), w.match.expectCount, errSearchSelectCountMismatch)
	}
	if len(matched) == 0 {
		return nil, errNoSearchSelectMatched
	}
	return w.followNextRequest(ctx, plg, invoker, matched[0])
}

func (w *compiledMultiRequest) handle(
	ctx context.Context,
	plg *SearchPlugin,
	invoker pluginapi.HTTPInvoker,
	evalCtx *evalContext,
) (*http.Response, error) {
	seen := map[string]struct{}{}
	for _, candidateTmpl := range w.candidates {
		candidate, err := candidateTmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		if w.unique {
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
		}
		itemCtx := &evalContext{number: evalCtx.number, host: evalCtx.host, vars: evalCtx.vars, candidate: candidate}
		targetReq, err := plg.buildRequest(ctx, w.request, itemCtx)
		if err != nil {
			return nil, err
		}
		rsp, err := invoker(ctx, targetReq)
		if err != nil {
			return nil, err
		}
		if err := checkAcceptedStatus(w.request, rsp.StatusCode); err != nil {
			_ = rsp.Body.Close()
			continue
		}
		body, node, err := readResponseBody(rsp, w.request.decodeCharset)
		if err != nil {
			return nil, err
		}
		itemCtx.body = body
		ok, err := w.successWhen.Eval(itemCtx, node)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, targetReq.URL.String())
		rsp.Body = io.NopCloser(bytes.NewReader([]byte(body)))
		return rsp, nil
	}
	return nil, errNoMultiRequestMatched
}
