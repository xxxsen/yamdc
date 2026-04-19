package yaml

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"gopkg.in/yaml.v3"

	"github.com/xxxsen/yamdc/internal/client"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

var (
	errPrecheckNotMatched           = errors.New("precheck did not match current plugin")
	errResponseTreatedAsNotFound    = errors.New("response is treated as not found")
	errWorkflowNotConfigured        = errors.New("workflow is not configured")
	errNoMultiRequestCandidateTried = errors.New("no multi_request candidate tried")
	errMissingWorkflowBaseResponse  = errors.New("missing workflow base response")
)

func CompileDraft(raw *PluginSpec) (*CompileResult, error) {
	spec, err := compilePlugin(raw)
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml failed, err:%w", err)
	}
	return &CompileResult{
		YAML: string(data),
		Summary: CompileSummary{
			HasRequest:      spec.request != nil,
			HasMultiRequest: spec.multiRequest != nil,
			HasWorkflow:     spec.workflow != nil,
			ScrapeFormat:    spec.scrape.format,
			FieldCount:      len(spec.scrape.fields),
		},
	}, nil
}

func DebugRequest(
	ctx context.Context,
	cli client.IHTTPClient,
	raw *PluginSpec,
	number string,
) (*RequestDebugResult, error) {
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
	if plg.spec.request != nil {
		req, err := plg.OnMakeHTTPRequest(ctx, number)
		if err != nil {
			return nil, err
		}
		rsp, err := cli.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute request: %w", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		resp, err := captureHTTPResponse(rsp, plg.spec.request.decodeCharset)
		if err != nil {
			return nil, err
		}
		return &RequestDebugResult{
			Request:  requestDebug(req),
			Response: resp,
		}, nil
	}
	return debugMultiRequest(ctx, cli, plg, number)
}

func newCompiledPlugin(raw *PluginSpec) (*SearchPlugin, error) {
	spec, err := compilePlugin(raw)
	if err != nil {
		return nil, err
	}
	return &SearchPlugin{spec: spec}, nil
}

func tryDebugMultiRequestCandidate(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin, candidate string, evalCtx *evalContext,
) (RequestDebugAttempt, *HTTPResponseDebug) {
	itemCtx := &evalContext{number: evalCtx.number, host: evalCtx.host, vars: evalCtx.vars, candidate: candidate}
	req, err := plg.buildRequest(ctx, plg.spec.multiRequest.request, itemCtx)
	if err != nil {
		return RequestDebugAttempt{Candidate: candidate, Error: err.Error()}, nil
	}
	attempt := RequestDebugAttempt{Candidate: candidate, Request: requestDebug(req)}
	rsp, err := cli.Do(req) //nolint:bodyclose // captureHTTPResponse closes the body
	if err != nil {
		attempt.Error = err.Error()
		return attempt, nil
	}
	resp, err := captureHTTPResponse(rsp, plg.spec.multiRequest.request.decodeCharset)
	if err != nil {
		attempt.Error = err.Error()
		return attempt, nil
	}
	attempt.Response = resp
	if err := checkAcceptedStatus(plg.spec.multiRequest.request, resp.StatusCode); err != nil {
		attempt.Error = err.Error()
		return attempt, nil
	}
	node, err := htmlquery.Parse(strings.NewReader(resp.Body))
	if err != nil {
		attempt.Error = err.Error()
		return attempt, nil
	}
	itemCtx.body = resp.Body
	ok, err := plg.spec.multiRequest.successWhen.Eval(itemCtx, node)
	if err != nil {
		attempt.Error = err.Error()
		return attempt, nil
	}
	attempt.Matched = ok
	return attempt, resp
}

func iterateMultiRequestCandidates(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin, evalCtx *evalContext,
) (*RequestDebugResult, error) {
	seen := map[string]struct{}{}
	out := &RequestDebugResult{}
	for _, candidateTmpl := range plg.spec.multiRequest.candidates {
		candidate, err := candidateTmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		if plg.spec.multiRequest.unique {
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
		}
		attempt, resp := tryDebugMultiRequestCandidate(ctx, cli, plg, candidate, evalCtx)
		out.Attempts = append(out.Attempts, attempt)
		if attempt.Matched {
			out.Candidate = candidate
			out.Request = attempt.Request
			out.Response = resp
			return out, nil
		}
	}
	if len(out.Attempts) == 0 {
		return nil, errNoMultiRequestCandidateTried
	}
	return out, errNoMultiRequestMatched
}

func debugMultiRequest(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin, number string,
) (*RequestDebugResult, error) {
	host := selectedHost(nil, plg.spec.hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	evalCtx := &evalContext{number: number, host: host, vars: readVarsFromContext(ctx)}
	return iterateMultiRequestCandidates(ctx, cli, plg, evalCtx)
}
