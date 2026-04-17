package yaml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/flarerr"
)

func resolveRequestURL(spec *compiledRequest, evalCtx *evalContext) (string, error) {
	if spec.rawURL != nil {
		return spec.rawURL.Render(evalCtx)
	}
	path, err := spec.path.Render(evalCtx)
	if err != nil {
		return "", err
	}
	return buildURL(evalCtx.host, path), nil
}

func buildRequestBodyReader(spec *compiledRequest, evalCtx *evalContext) (io.Reader, error) {
	if spec.body == nil {
		return nil, nil //nolint:nilnil // nil body means no request body
	}
	switch spec.body.kind {
	case bodyKindForm:
		return renderFormBody(spec.body, evalCtx)
	case bodyKindJSON:
		return renderJSONBody(spec.body, evalCtx)
	case bodyKindRaw:
		return renderRawBody(spec.body, evalCtx)
	default:
		return nil, nil //nolint:nilnil // unknown body kind
	}
}

func renderFormBody(body *compiledRequestBody, evalCtx *evalContext) (io.Reader, error) {
	vals := url.Values{}
	for key, tmpl := range body.values {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		vals.Set(key, rendered)
	}
	return strings.NewReader(vals.Encode()), nil
}

func renderJSONBody(body *compiledRequestBody, evalCtx *evalContext) (io.Reader, error) {
	payload := map[string]string{}
	for key, tmpl := range body.values {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		payload[key] = rendered
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal json body: %w", err)
	}
	return bytes.NewReader(raw), nil
}

func renderRawBody(body *compiledRequestBody, evalCtx *evalContext) (io.Reader, error) {
	if body.content == nil {
		return nil, nil //nolint:nilnil // nil body for raw without content
	}
	rendered, err := body.content.Render(evalCtx)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(rendered), nil
}

func applyRequestParams(req *http.Request, spec *compiledRequest, evalCtx *evalContext) error {
	for key, tmpl := range spec.query {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return err
		}
		q := req.URL.Query()
		q.Set(key, rendered)
		req.URL.RawQuery = q.Encode()
	}
	for key, tmpl := range spec.headers {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return err
		}
		req.Header.Set(key, rendered)
	}
	for key, tmpl := range spec.cookies {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return err
		}
		req.AddCookie(&http.Cookie{Name: key, Value: rendered})
	}
	return nil
}

func setBodyContentType(req *http.Request, spec *compiledRequest) {
	if spec.body == nil {
		return
	}
	switch spec.body.kind {
	case bodyKindForm:
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	case bodyKindJSON:
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}
}

const defaultWaitStable = 5 * time.Second

func (p *SearchPlugin) applyFetchTypeContext(req *http.Request, spec *compiledRequest) *http.Request {
	switch p.spec.fetchType {
	case fetchTypeBrowser:
		return applyBrowserParams(req, spec)
	case fetchTypeFlaresolverr:
		return req.WithContext(flarerr.WithParams(req.Context(), &flarerr.Params{}))
	default:
		return req
	}
}

func applyBrowserParams(req *http.Request, spec *compiledRequest) *http.Request {
	params := &browser.Params{}
	if spec.browser != nil {
		params.WaitSelector = spec.browser.waitSelector
		params.WaitTimeout = spec.browser.waitTimeout
		params.WaitStableDuration = spec.browser.waitStable
	}
	if params.WaitSelector == "" && params.WaitStableDuration == 0 {
		params.WaitStableDuration = defaultWaitStable
	}
	if len(spec.headers) > 0 {
		params.Headers = make(http.Header, len(spec.headers))
		for key := range spec.headers {
			if v := req.Header.Get(key); v != "" {
				params.Headers.Set(key, v)
			}
		}
	}
	return req.WithContext(browser.WithParams(req.Context(), params))
}

func (p *SearchPlugin) buildRequest(
	ctx context.Context, spec *compiledRequest, evalCtx *evalContext,
) (*http.Request, error) {
	if evalCtx.host == "" {
		evalCtx.host = currentHost(ctx, p.spec.hosts)
	}
	targetURL, err := resolveRequestURL(spec, evalCtx)
	if err != nil {
		return nil, err
	}
	body, err := buildRequestBodyReader(spec, evalCtx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, spec.method, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	if err := applyRequestParams(req, spec, evalCtx); err != nil {
		return nil, err
	}
	setBodyContentType(req, spec)
	req = p.applyFetchTypeContext(req, spec)
	return req, nil
}
