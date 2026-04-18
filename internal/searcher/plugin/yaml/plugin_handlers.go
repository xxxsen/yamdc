package yaml

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/antchfx/htmlquery"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

func (p *SearchPlugin) OnGetHosts(_ context.Context) []string {
	return append([]string(nil), p.spec.hosts...)
}

func (p *SearchPlugin) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	if p.spec.precheck == nil {
		return true, nil
	}
	if len(p.spec.precheck.numberPatterns) > 0 {
		matched := false
		for _, pattern := range p.spec.precheck.numberPatterns {
			ok, err := regexpMatch(pattern, number)
			if err != nil {
				return false, err
			}
			if ok {
				matched = true
				break
			}
		}
		if !matched {
			return false, nil
		}
	}
	evalCtx := &evalContext{number: number, host: selectedHost(nil, p.spec.hosts), vars: map[string]string{}}
	for key, value := range p.spec.precheck.variables {
		rendered, err := value.Render(evalCtx)
		if err != nil {
			return false, err
		}
		evalCtx.vars[key] = rendered
		pluginapi.SetContainerValue(ctx, ctxVarKey(key), rendered)
	}
	return true, nil
}

func (p *SearchPlugin) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	host := pluginapi.MustSelectDomain(p.spec.hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	if p.spec.request == nil {
		if p.spec.multiRequest == nil {
			return nil, errRequestNil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, host, nil)
		if err != nil {
			return nil, fmt.Errorf("create placeholder request: %w", err)
		}
		return req, nil
	}
	return p.buildRequest(ctx, p.spec.request, &evalContext{
		number: number,
		host:   host,
		vars:   readVarsFromContext(ctx),
	})
}

func (p *SearchPlugin) OnDecorateRequest(_ context.Context, _ *http.Request) error {
	return nil
}

func (p *SearchPlugin) OnHandleHTTPRequest(ctx context.Context, invoker pluginapi.HTTPInvoker, req *http.Request) (
	*http.Response,
	error,
) {
	evalCtx := &evalContext{
		number: ctxNumber(ctx),
		host:   currentHost(ctx, p.spec.hosts),
		vars:   readVarsFromContext(ctx),
	}
	if p.spec.multiRequest != nil {
		rsp, err := p.spec.multiRequest.handle(ctx, p, invoker, evalCtx)
		if err != nil {
			return nil, err
		}
		if p.spec.workflow != nil {
			return p.spec.workflow.handleResponse(ctx, p, invoker, rsp, evalCtx, p.spec.multiRequest.request.decodeCharset)
		}
		return rsp, nil
	}
	if p.spec.workflow == nil {
		return invoker(ctx, req)
	}
	return p.spec.workflow.handleRequest(ctx, p, invoker, req, evalCtx)
}

func (p *SearchPlugin) OnPrecheckResponse(_ context.Context, _ *http.Request, rsp *http.Response) (bool, error) {
	finalReq := p.spec.finalRequest()
	if finalReq == nil {
		if rsp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return true, nil
	}
	for _, code := range finalReq.notFoundStatusCodes {
		if rsp.StatusCode == code {
			return false, nil
		}
	}
	if len(finalReq.acceptStatusCodes) == 0 {
		return rsp.StatusCode != http.StatusNotFound, nil
	}
	for _, code := range finalReq.acceptStatusCodes {
		if rsp.StatusCode == code {
			return true, nil
		}
	}
	return false, fmt.Errorf("status code %d: %w", rsp.StatusCode, errStatusCodeNotAccepted)
}

func (p *SearchPlugin) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	finalReq := p.spec.finalRequest()
	decoded, err := decodeBytes(data, finalReq.decodeCharset)
	if err != nil {
		return nil, false, err
	}
	var mv *model.MovieMeta
	switch p.spec.scrape.format {
	case formatHTML:
		node, err := htmlquery.Parse(bytes.NewReader(decoded))
		if err != nil {
			return nil, false, fmt.Errorf("parse html failed: %w", err)
		}
		mv, err = p.decodeHTML(ctx, node)
		if err != nil {
			return nil, false, err
		}
	case formatJSON:
		mv, err = p.decodeJSON(ctx, decoded)
		if err != nil {
			return nil, false, err
		}
	default:
		return nil, false, fmt.Errorf("%w: %s", errUnsupportedScrapeFormat, p.spec.scrape.format)
	}
	if mv == nil {
		return nil, false, nil
	}
	p.applyPostprocess(ctx, mv)
	return mv, true, nil
}

func (p *SearchPlugin) OnDecorateMediaRequest(ctx context.Context, req *http.Request) error {
	baseReq := p.spec.finalRequest()
	if baseReq == nil {
		return nil
	}
	for key, value := range baseReq.headers {
		rendered, err := value.Render(&evalContext{
			number: ctxNumber(ctx),
			host:   currentHost(ctx, p.spec.hosts),
			vars:   readVarsFromContext(ctx),
		})
		if err != nil {
			return err
		}
		req.Header.Set(key, rendered)
	}
	for key, value := range baseReq.cookies {
		rendered, err := value.Render(&evalContext{
			number: ctxNumber(ctx),
			host:   currentHost(ctx, p.spec.hosts),
			vars:   readVarsFromContext(ctx),
		})
		if err != nil {
			return err
		}
		req.AddCookie(&http.Cookie{Name: key, Value: rendered})
	}
	if referer, ok := pluginapi.GetContainerValue(ctx, ctxKeyFinalPage); ok && referer != "" {
		req.Header.Set("Referer", referer)
	}
	return nil
}
