package yaml

import (
	"context"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

type workflowMultiRequestDebug struct {
	steps    []WorkflowDebugStep
	response *HTTPResponseDebug
}

type workflowCandidateResult struct {
	step     WorkflowDebugStep
	response *HTTPResponseDebug
	evalCtx  *evalContext
	matched  bool
}

func DebugWorkflow(
	ctx context.Context, cli client.IHTTPClient, raw *PluginSpec, number string,
) (*WorkflowDebugResult, error) {
	plg, err := newCompiledPlugin(raw)
	if err != nil {
		return nil, err
	}
	ctx = pluginapi.InitContainer(ctx)
	ctx = meta.SetNumberID(ctx, number)
	ok, err := plg.OnPrecheckRequest(ctx, strings.TrimSpace(number))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errPrecheckNotMatched
	}
	if plg.spec.workflow == nil && plg.spec.multiRequest == nil {
		return nil, errWorkflowNotConfigured
	}
	// 预先初始化 Steps 避免 nil 切片序列化成 null, 影响前端直接 .length / .map。
	result := &WorkflowDebugResult{Steps: []WorkflowDebugStep{}}
	host := selectedHost(nil, plg.spec.hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	evalCtx := &evalContext{number: number, host: host, vars: readVarsFromContext(ctx)}

	var baseResp *HTTPResponseDebug
	baseResp, evalCtx, result = debugWorkflowRequestPhase(ctx, cli, plg, evalCtx, result)
	if result.Error != "" || plg.spec.workflow == nil {
		return result, nil
	}
	steps, err := debugWorkflowSearchSelect(ctx, cli, plg, evalCtx, baseResp)
	result.Steps = append(result.Steps, steps...)
	if err != nil {
		result.Error = err.Error()
	}
	return result, nil
}

func debugWorkflowRequestPhase(
	ctx context.Context,
	cli client.IHTTPClient,
	plg *SearchPlugin,
	evalCtx *evalContext,
	result *WorkflowDebugResult,
) (*HTTPResponseDebug, *evalContext, *WorkflowDebugResult) {
	if plg.spec.multiRequest != nil {
		return debugWorkflowMultiRequestPhase(ctx, cli, plg, evalCtx, result)
	}
	baseResp, result := debugWorkflowSingleRequestPhase(ctx, cli, plg, evalCtx, result)
	return baseResp, evalCtx, result
}

func debugWorkflowMultiRequestPhase(
	ctx context.Context,
	cli client.IHTTPClient,
	plg *SearchPlugin,
	evalCtx *evalContext,
	result *WorkflowDebugResult,
) (*HTTPResponseDebug, *evalContext, *WorkflowDebugResult) {
	mr, finalCtx, err := debugWorkflowMultiRequest(ctx, cli, plg, evalCtx)
	if mr != nil {
		result.Steps = append(result.Steps, mr.steps...)
	}
	if err != nil {
		result.Error = fmt.Sprintf("multi_request failed: %s", err.Error())
		return nil, nil, result
	}
	return mr.response, finalCtx, result
}

func debugWorkflowSingleRequestPhase(
	ctx context.Context,
	cli client.IHTTPClient,
	plg *SearchPlugin,
	evalCtx *evalContext,
	result *WorkflowDebugResult,
) (*HTTPResponseDebug, *WorkflowDebugResult) {
	req, err := plg.buildRequest(ctx, plg.spec.request, evalCtx)
	if err != nil {
		result.Error = fmt.Sprintf("build request failed: %s", err.Error())
		return nil, result
	}
	rsp, err := cli.Do(req) //nolint:bodyclose // captureHTTPResponse closes the body
	if err != nil {
		result.Error = fmt.Sprintf("request failed: url=%s err=%s", req.URL.String(), err.Error())
		return nil, result
	}
	resp, err := captureHTTPResponse(rsp, plg.spec.request.decodeCharset)
	if err != nil {
		result.Error = fmt.Sprintf("read response failed: url=%s err=%s", req.URL.String(), err.Error())
		return nil, result
	}
	result.Steps = append(result.Steps, WorkflowDebugStep{
		Stage:    "request",
		Summary:  fmt.Sprintf("opened initial page, status=%d body_size=%d", resp.StatusCode, len(resp.Body)),
		Request:  ptrRequestDebug(requestDebug(req)),
		Response: resp,
	})
	if err := checkAcceptedStatus(plg.spec.request, resp.StatusCode); err != nil {
		result.Error = fmt.Sprintf("status check failed: url=%s %s", req.URL.String(), err.Error())
		return nil, result
	}
	return resp, result
}

func tryDebugWorkflowCandidate(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin, candidate string, evalCtx *evalContext,
) workflowCandidateResult {
	itemCtx := &evalContext{number: evalCtx.number, host: evalCtx.host, vars: evalCtx.vars, candidate: candidate}
	req, err := plg.buildRequest(ctx, plg.spec.multiRequest.request, itemCtx)
	if err != nil {
		return workflowCandidateResult{step: WorkflowDebugStep{
			Stage: "multi_request", Candidate: candidate,
			Summary: fmt.Sprintf("build request for candidate %q failed: %s", candidate, err.Error()),
		}}
	}
	step := WorkflowDebugStep{
		Stage: "multi_request", Candidate: candidate,
		Request: ptrRequestDebug(requestDebug(req)),
	}
	rsp, err := cli.Do(req) //nolint:bodyclose // captureHTTPResponse closes the body
	if err != nil {
		step.Summary = fmt.Sprintf("request failed: url=%s err=%s", req.URL.String(), err.Error())
		return workflowCandidateResult{step: step}
	}
	resp, err := captureHTTPResponse(rsp, plg.spec.multiRequest.request.decodeCharset)
	if err != nil {
		step.Summary = fmt.Sprintf("read response failed: url=%s err=%s", req.URL.String(), err.Error())
		return workflowCandidateResult{step: step}
	}
	step.Response = resp
	if err := checkAcceptedStatus(plg.spec.multiRequest.request, resp.StatusCode); err != nil {
		step.Summary = fmt.Sprintf("status=%d rejected: %s", resp.StatusCode, err.Error())
		return workflowCandidateResult{step: step}
	}
	node, err := htmlquery.Parse(strings.NewReader(resp.Body))
	if err != nil {
		step.Summary = fmt.Sprintf("parse html failed: %s", err.Error())
		return workflowCandidateResult{step: step}
	}
	itemCtx.body = resp.Body
	matched, err := plg.spec.multiRequest.successWhen.Eval(itemCtx, node)
	if err != nil {
		step.Summary = fmt.Sprintf("eval success_when failed: %s", err.Error())
		return workflowCandidateResult{step: step}
	}
	if !matched {
		step.Summary = fmt.Sprintf("success_when not matched, status=%d body_size=%d", resp.StatusCode, len(resp.Body))
		return workflowCandidateResult{step: step}
	}
	step.Summary = fmt.Sprintf("candidate matched, status=%d body_size=%d", resp.StatusCode, len(resp.Body))
	return workflowCandidateResult{step: step, response: resp, evalCtx: itemCtx, matched: true}
}

func debugWorkflowMultiRequest(
	ctx context.Context,
	cli client.IHTTPClient,
	plg *SearchPlugin,
	evalCtx *evalContext,
) (*workflowMultiRequestDebug, *evalContext, error) {
	seen := map[string]struct{}{}
	result := &workflowMultiRequestDebug{}
	for _, candidateTmpl := range plg.spec.multiRequest.candidates {
		candidate, err := candidateTmpl.Render(evalCtx)
		if err != nil {
			return result, nil, fmt.Errorf("render candidate template failed: %w", err)
		}
		if plg.spec.multiRequest.unique {
			if _, ok := seen[candidate]; ok {
				result.steps = append(result.steps, WorkflowDebugStep{
					Stage: "multi_request", Candidate: candidate,
					Summary: "skipped (duplicate candidate)",
				})
				continue
			}
			seen[candidate] = struct{}{}
		}
		cr := tryDebugWorkflowCandidate(ctx, cli, plg, candidate, evalCtx)
		result.steps = append(result.steps, cr.step)
		if cr.matched {
			result.response = cr.response
			return result, cr.evalCtx, nil
		}
	}
	return result, nil, fmt.Errorf("tried %d candidate(s): %w", len(seen), errNoMultiRequestMatched)
}

func debugCollectSelectors(node *html.Node, w *compiledSearchSelectWorkflow) (
	map[string][]string, []string, int, error,
) {
	results := make(map[string][]string, len(w.selectors))
	expectedLen := -1
	summary := make([]string, 0, len(w.selectors))
	for _, sel := range w.selectors {
		items := decoder.DecodeList(node, sel.expr)
		results[sel.name] = items
		summary = append(summary, fmt.Sprintf("%s=%d", sel.name, len(items)))
		if expectedLen == -1 {
			expectedLen = len(items)
			continue
		}
		if expectedLen != len(items) {
			return results, summary, expectedLen,
				fmt.Errorf("%w: %s", errSelectorCountMismatch, strings.Join(summary, ", "))
		}
	}
	return results, summary, expectedLen, nil
}

func debugMatchWorkflowItem(
	evalCtx *evalContext, w *compiledSearchSelectWorkflow,
	results map[string][]string, body string, node *html.Node, i int,
) (WorkflowSelectorItem, *evalContext, error) {
	itemCtx := &evalContext{
		number: evalCtx.number, host: evalCtx.host, body: body, vars: evalCtx.vars,
		item:          make(map[string]string, len(results)),
		itemVariables: make(map[string]string, len(w.itemVariables)),
	}
	itemDbg := WorkflowSelectorItem{Index: i, Item: make(map[string]string, len(results))}
	for name, lst := range results {
		itemCtx.item[name] = lst[i]
		itemDbg.Item[name] = lst[i]
	}
	for key, tmpl := range w.itemVariables {
		v, err := tmpl.Render(itemCtx)
		if err != nil {
			return itemDbg, nil, fmt.Errorf("item_variables render failed at index %d: %w", i, err)
		}
		itemCtx.itemVariables[key] = v
	}
	if len(itemCtx.itemVariables) != 0 {
		itemDbg.ItemVariables = itemCtx.itemVariables
	}
	matchPass := true
	if w.match != nil {
		itemDbg.MatchDetails = make([]WorkflowMatchDetail, 0, len(w.match.conditions))
		for _, cond := range w.match.conditions {
			ok, err := cond.Eval(itemCtx, node)
			if err != nil {
				return itemDbg, nil, fmt.Errorf("eval condition failed at index %d: %w", i, err)
			}
			itemDbg.MatchDetails = append(itemDbg.MatchDetails, WorkflowMatchDetail{
				Condition: renderCondition(cond),
				Pass:      ok,
			})
		}
		ok, err := w.match.Eval(itemCtx, node)
		if err != nil {
			return itemDbg, nil, fmt.Errorf("eval match failed at index %d: %w", i, err)
		}
		matchPass = ok
	}
	itemDbg.Matched = matchPass
	if matchPass {
		return itemDbg, itemCtx, nil
	}
	return itemDbg, nil, nil
}

func debugFollowNextRequest(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin,
	w *compiledSearchSelectWorkflow, matched []*evalContext,
) (string, *WorkflowDebugStep, error) {
	value, err := w.ret.Render(matched[0])
	if err != nil {
		return "", nil, fmt.Errorf("render return template failed: %w", err)
	}
	matched[0].value = value
	nextReq, err := plg.buildRequest(ctx, w.nextRequest, matched[0])
	if err != nil {
		return value, nil, fmt.Errorf("build next_request failed: selected_value=%s err=%w", value, err)
	}
	rsp, err := cli.Do(nextReq) //nolint:bodyclose // captureHTTPResponse closes the body
	if err != nil {
		return value, nil, fmt.Errorf("next_request failed: url=%s err=%w", nextReq.URL.String(), err)
	}
	resp, err := captureHTTPResponse(rsp, w.nextRequest.decodeCharset)
	if err != nil {
		return value, nil, fmt.Errorf("read next_request response failed: url=%s err=%w", nextReq.URL.String(), err)
	}
	if err := checkAcceptedStatus(w.nextRequest, resp.StatusCode); err != nil {
		nextStep := WorkflowDebugStep{
			Stage:    "next_request",
			Summary:  fmt.Sprintf("detail page status rejected: url=%s status=%d", nextReq.URL.String(), resp.StatusCode),
			Request:  ptrRequestDebug(requestDebug(nextReq)),
			Response: resp,
		}
		return value, &nextStep, fmt.Errorf("next_request status check failed: url=%s %w", nextReq.URL.String(), err)
	}
	step := WorkflowDebugStep{
		Stage:    "next_request",
		Summary:  fmt.Sprintf("opened detail page, status=%d body_size=%d", resp.StatusCode, len(resp.Body)),
		Request:  ptrRequestDebug(requestDebug(nextReq)),
		Response: resp,
	}
	return value, &step, nil
}

func debugMatchAllWorkflowItems(
	evalCtx *evalContext, w *compiledSearchSelectWorkflow, results map[string][]string,
	body string, node *html.Node, expectedLen int, step *WorkflowDebugStep,
) ([]*evalContext, error) {
	matched := make([]*evalContext, 0, expectedLen)
	for i := 0; i < expectedLen; i++ {
		itemDbg, itemCtx, err := debugMatchWorkflowItem(evalCtx, w, results, body, node, i)
		step.Items = append(step.Items, itemDbg)
		if err != nil {
			step.Summary = err.Error()
			return nil, err
		}
		if itemCtx != nil {
			matched = append(matched, itemCtx)
		}
	}
	return matched, nil
}

func debugValidateMatchCount(
	matched []*evalContext, w *compiledSearchSelectWorkflow, expectedLen int,
	summaryStr string, bodySize int, step *WorkflowDebugStep,
) error {
	if w.match != nil && w.match.expectCount > 0 && len(matched) != w.match.expectCount {
		step.Summary = fmt.Sprintf("matched %d/%d items (expect_count=%d), selector_counts=[%s], response_body_size=%d",
			len(matched), expectedLen, w.match.expectCount, summaryStr, bodySize)
		return fmt.Errorf(
			"got %d expect_count=%d total_items=%d selectors=[%s] body_size=%d: %w",
			len(matched), w.match.expectCount, expectedLen, summaryStr, bodySize, errSearchSelectCountMismatch,
		)
	}
	if len(matched) == 0 {
		step.Summary = fmt.Sprintf("0/%d items matched, selector_counts=[%s], response_body_size=%d",
			expectedLen, summaryStr, bodySize)
		return fmt.Errorf("total_items=%d selectors=[%s] body_size=%d: %w",
			expectedLen, summaryStr, bodySize, errNoSearchSelectMatched)
	}
	step.Summary = fmt.Sprintf("%d/%d items matched", len(matched), expectedLen)
	return nil
}

func debugWorkflowSearchSelect(
	ctx context.Context,
	cli client.IHTTPClient,
	plg *SearchPlugin,
	evalCtx *evalContext,
	baseResp *HTTPResponseDebug,
) ([]WorkflowDebugStep, error) {
	if baseResp == nil {
		return nil, errMissingWorkflowBaseResponse
	}
	node, err := htmlquery.Parse(strings.NewReader(baseResp.Body))
	if err != nil {
		return nil, fmt.Errorf("parse response body as HTML failed, body_size=%d: %w", len(baseResp.Body), err)
	}
	w := plg.spec.workflow
	results, selectorSummary, expectedLen, err := debugCollectSelectors(node, w)
	if err != nil {
		step := WorkflowDebugStep{
			Stage: "search_select", Selectors: results,
			Summary: fmt.Sprintf("selector count mismatch: %s", strings.Join(selectorSummary, ", ")),
		}
		return []WorkflowDebugStep{step}, err
	}
	step := WorkflowDebugStep{
		Stage: "search_select", Selectors: results,
		Items: make([]WorkflowSelectorItem, 0, max(expectedLen, 0)),
	}
	matched, err := debugMatchAllWorkflowItems(evalCtx, w, results, baseResp.Body, node, expectedLen, &step)
	if err != nil {
		return []WorkflowDebugStep{step}, err
	}
	summaryStr := strings.Join(selectorSummary, ", ")
	if err := debugValidateMatchCount(matched, w, expectedLen, summaryStr, len(baseResp.Body), &step); err != nil {
		return []WorkflowDebugStep{step}, err
	}
	value, nextStep, err := debugFollowNextRequest(ctx, cli, plg, w, matched)
	step.SelectedValue = value
	if nextStep != nil {
		return []WorkflowDebugStep{step, *nextStep}, err
	}
	if err != nil {
		step.Summary += fmt.Sprintf(", %s", err.Error())
		return []WorkflowDebugStep{step}, err
	}
	return []WorkflowDebugStep{step}, nil
}
