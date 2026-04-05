package yamlplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"gopkg.in/yaml.v3"
)

const (
	ctxKeyHost        = "yamlplugin.host"
	ctxKeyFinalPage   = "yamlplugin.final_page"
	ctxKeyRequestPath = "yamlplugin.request_path"
)

type compiledPlugin struct {
	version     int
	name        string
	pluginType  string
	hosts       []string
	precheck    *compiledPrecheck
	request     *compiledRequest
	multiRequest *compiledMultiRequest
	workflow    *compiledSearchSelectWorkflow
	scrape      *compiledScrape
	postprocess *compiledPostprocess
}

type compiledPrecheck struct {
	numberPatterns []string
	variables      map[string]*template
}

type compiledRequest struct {
	method              string
	path                *template
	rawURL              *template
	query               map[string]*template
	headers             map[string]*template
	cookies             map[string]*template
	body                *compiledRequestBody
	acceptStatusCodes   []int
	notFoundStatusCodes []int
	decodeCharset       string
}

type compiledRequestBody struct {
	kind    string
	values  map[string]*template
	content *template
}

type compiledSearchSelectWorkflow struct {
	selectors     []*compiledSelectorList
	itemVariables map[string]*template
	match         *compiledConditionGroup
	ret           *template
	nextRequest   *compiledRequest
}

type compiledMultiRequest struct {
	candidates  []*template
	unique      bool
	request     *compiledRequest
	successWhen *compiledConditionGroup
}

type compiledSelector struct {
	kind string
	expr string
}

type compiledSelectorList struct {
	name string
	compiledSelector
}

type compiledField struct {
	name       string
	selector   *compiledSelector
	transforms []*TransformSpec
	parser     ParserSpec
	required   bool
}

type compiledScrape struct {
	format string
	fields []*compiledField
}

type compiledPostprocess struct {
	assign       map[string]*template
	defaults     *DefaultsSpec
	switchConfig *SwitchConfigSpec
}

type YAMLSearchPlugin struct {
	spec *compiledPlugin
}

func (p *compiledPlugin) finalRequest() *compiledRequest {
	if p.workflow != nil {
		return p.workflow.nextRequest
	}
	if p.multiRequest != nil {
		return p.multiRequest.request
	}
	return p.request
}

func NewFromBytes(data []byte) (pluginapi.IPlugin, error) {
	raw := &PluginSpec{}
	if err := yaml.Unmarshal(data, raw); err != nil {
		return nil, fmt.Errorf("decode yaml plugin failed, err:%w", err)
	}
	spec, err := compilePlugin(raw)
	if err != nil {
		return nil, err
	}
	return &YAMLSearchPlugin{spec: spec}, nil
}

func compilePlugin(raw *PluginSpec) (*compiledPlugin, error) {
	if raw.Version != 1 {
		return nil, fmt.Errorf("unsupported version:%d", raw.Version)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if raw.Type != typeOneStep && raw.Type != typeTwoStep {
		return nil, fmt.Errorf("invalid type:%s", raw.Type)
	}
	if len(raw.Hosts) == 0 {
		return nil, fmt.Errorf("hosts is required")
	}
	if raw.Request != nil && raw.MultiRequest != nil {
		return nil, fmt.Errorf("request and multi_request are mutually exclusive")
	}
	if raw.Type == typeTwoStep && (raw.Workflow == nil || raw.Workflow.SearchSelect == nil) {
		return nil, fmt.Errorf("two-step requires workflow.search_select")
	}
	if raw.Request == nil && raw.MultiRequest == nil {
		return nil, fmt.Errorf("request or multi_request is required")
	}
	out := &compiledPlugin{
		version:    raw.Version,
		name:       raw.Name,
		pluginType: raw.Type,
		hosts:      append([]string(nil), raw.Hosts...),
	}
	var err error
	if out.precheck, err = compilePrecheck(raw.Precheck); err != nil {
		return nil, err
	}
	if raw.Request != nil {
		if out.request, err = compileRequest(raw.Request); err != nil {
			return nil, err
		}
	}
	if raw.MultiRequest != nil {
		if out.multiRequest, err = compileMultiRequest(raw.MultiRequest); err != nil {
			return nil, err
		}
	}
	if raw.Workflow != nil {
		if out.workflow, err = compileWorkflow(raw.Workflow); err != nil {
			return nil, err
		}
	}
	if out.scrape, err = compileScrape(raw.Scrape); err != nil {
		return nil, err
	}
	if out.postprocess, err = compilePostprocess(raw.Postprocess); err != nil {
		return nil, err
	}
	return out, nil
}

func compilePrecheck(raw *PrecheckSpec) (*compiledPrecheck, error) {
	if raw == nil {
		return nil, nil
	}
	out := &compiledPrecheck{
		numberPatterns: append([]string(nil), raw.NumberPatterns...),
		variables:      make(map[string]*template, len(raw.Variables)),
	}
	for key, value := range raw.Variables {
		t, err := compileTemplate(value)
		if err != nil {
			return nil, fmt.Errorf("compile precheck variable %s failed, err:%w", key, err)
		}
		out.variables[key] = t
	}
	return out, nil
}

func compileRequest(raw *RequestSpec) (*compiledRequest, error) {
	if raw == nil {
		return nil, nil
	}
	if raw.Method == "" {
		return nil, fmt.Errorf("request method is required")
	}
	if raw.Path == "" && raw.URL == "" {
		return nil, fmt.Errorf("request path or url is required")
	}
	if raw.Path != "" && raw.URL != "" {
		return nil, fmt.Errorf("request path and url are mutually exclusive")
	}
	out := &compiledRequest{
		method:              strings.ToUpper(raw.Method),
		query:               make(map[string]*template, len(raw.Query)),
		headers:             make(map[string]*template, len(raw.Headers)),
		cookies:             make(map[string]*template, len(raw.Cookies)),
		acceptStatusCodes:   append([]int(nil), raw.AcceptStatusCodes...),
		notFoundStatusCodes: append([]int(nil), raw.NotFoundStatusCodes...),
	}
	if out.method != http.MethodGet && out.method != http.MethodPost {
		return nil, fmt.Errorf("unsupported request method:%s", out.method)
	}
	var err error
	if raw.Path != "" {
		if out.path, err = compileTemplate(raw.Path); err != nil {
			return nil, err
		}
	}
	if raw.URL != "" {
		if out.rawURL, err = compileTemplate(raw.URL); err != nil {
			return nil, err
		}
	}
	for k, v := range raw.Query {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out.query[k] = t
	}
	for k, v := range raw.Headers {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out.headers[k] = t
	}
	for k, v := range raw.Cookies {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out.cookies[k] = t
	}
	if raw.Body != nil {
		body, err := compileRequestBody(raw.Body)
		if err != nil {
			return nil, err
		}
		out.body = body
	}
	if raw.Response != nil {
		out.decodeCharset = strings.ToLower(strings.TrimSpace(raw.Response.DecodeCharset))
	}
	return out, nil
}

func compileRequestBody(raw *RequestBodySpec) (*compiledRequestBody, error) {
	out := &compiledRequestBody{kind: raw.Kind, values: make(map[string]*template, len(raw.Values))}
	switch raw.Kind {
	case bodyKindForm, bodyKindJSON, bodyKindRaw:
	default:
		return nil, fmt.Errorf("unsupported body kind:%s", raw.Kind)
	}
	for k, v := range raw.Values {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out.values[k] = t
	}
	if raw.Content != "" {
		t, err := compileTemplate(raw.Content)
		if err != nil {
			return nil, err
		}
		out.content = t
	}
	return out, nil
}

func compileWorkflow(raw *WorkflowSpec) (*compiledSearchSelectWorkflow, error) {
	if raw == nil {
		return nil, nil
	}
	if raw.SearchSelect != nil {
		return compileSearchSelect(raw.SearchSelect)
	}
	return nil, nil
}

func compileSearchSelect(raw *SearchSelectWorkflowSpec) (*compiledSearchSelectWorkflow, error) {
	if len(raw.Selectors) < 2 {
		return nil, fmt.Errorf("search_select requires at least 2 selectors")
	}
	if raw.NextRequest == nil {
		return nil, fmt.Errorf("search_select next_request is required")
	}
	nextReq, err := compileRequest(raw.NextRequest)
	if err != nil {
		return nil, err
	}
	match, err := compileConditionGroup(raw.Match)
	if err != nil {
		return nil, err
	}
	ret, err := compileTemplate(raw.Return)
	if err != nil {
		return nil, err
	}
	out := &compiledSearchSelectWorkflow{
		itemVariables: make(map[string]*template, len(raw.ItemVariables)),
		match:         match,
		ret:           ret,
		nextRequest:   nextReq,
	}
	for _, item := range raw.Selectors {
		if item.Kind != "xpath" {
			return nil, fmt.Errorf("unsupported selector kind:%s", item.Kind)
		}
		out.selectors = append(out.selectors, &compiledSelectorList{
			name: item.Name,
			compiledSelector: compiledSelector{
				kind: item.Kind,
				expr: item.Expr,
			},
		})
	}
	for k, v := range raw.ItemVariables {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out.itemVariables[k] = t
	}
	return out, nil
}

func compileMultiRequest(raw *MultiRequestSpec) (*compiledMultiRequest, error) {
	if len(raw.Candidates) == 0 {
		return nil, fmt.Errorf("multi_request candidates is required")
	}
	if raw.Request == nil {
		return nil, fmt.Errorf("multi_request request is required")
	}
	req, err := compileRequest(raw.Request)
	if err != nil {
		return nil, err
	}
	successWhen, err := compileConditionGroup(raw.SuccessWhen)
	if err != nil {
		return nil, err
	}
	out := &compiledMultiRequest{
		unique:      raw.Unique,
		request:     req,
		successWhen: successWhen,
	}
	for _, candidate := range raw.Candidates {
		t, err := compileTemplate(candidate)
		if err != nil {
			return nil, err
		}
		out.candidates = append(out.candidates, t)
	}
	return out, nil
}

func compileScrape(raw *ScrapeSpec) (*compiledScrape, error) {
	if raw == nil {
		return nil, fmt.Errorf("scrape is required")
	}
	if raw.Format != formatHTML {
		return nil, fmt.Errorf("unsupported scrape format:%s", raw.Format)
	}
	if len(raw.Fields) == 0 {
		return nil, fmt.Errorf("scrape fields is required")
	}
	out := &compiledScrape{format: raw.Format}
	ordered := []string{"number", "title", "plot", "actors", "release_date", "duration", "studio", "label", "director", "series", "genres", "cover", "poster", "sample_images"}
	for _, name := range ordered {
		spec, ok := raw.Fields[name]
		if !ok {
			continue
		}
		if spec.Selector == nil {
			return nil, fmt.Errorf("field %s selector is required", name)
		}
		if spec.Selector.Kind != "xpath" {
			return nil, fmt.Errorf("field %s selector kind unsupported:%s", name, spec.Selector.Kind)
		}
		out.fields = append(out.fields, &compiledField{
			name: name,
			selector: &compiledSelector{
				kind: spec.Selector.Kind,
				expr: spec.Selector.Expr,
			},
			transforms: spec.Transforms,
			parser:     spec.Parser,
			required:   spec.Required,
		})
	}
	return out, nil
}

func compilePostprocess(raw *PostprocessSpec) (*compiledPostprocess, error) {
	if raw == nil {
		return nil, nil
	}
	out := &compiledPostprocess{
		assign:       make(map[string]*template, len(raw.Assign)),
		defaults:     raw.Defaults,
		switchConfig: raw.SwitchConfig,
	}
	for k, v := range raw.Assign {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out.assign[k] = t
	}
	return out, nil
}

func (p *YAMLSearchPlugin) OnGetHosts(ctx context.Context) []string {
	return append([]string(nil), p.spec.hosts...)
}

func (p *YAMLSearchPlugin) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
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

func (p *YAMLSearchPlugin) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	host := pluginapi.MustSelectDomain(p.spec.hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	if p.spec.request == nil {
		if p.spec.multiRequest == nil {
			return nil, fmt.Errorf("request is nil")
		}
		return http.NewRequestWithContext(ctx, http.MethodGet, host, nil)
	}
	return p.buildRequest(ctx, p.spec.request, &evalContext{
		number: number,
		host:   host,
		vars:   readVarsFromContext(ctx),
	})
}

func (p *YAMLSearchPlugin) OnDecorateRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (p *YAMLSearchPlugin) OnHandleHTTPRequest(ctx context.Context, invoker pluginapi.HTTPInvoker, req *http.Request) (*http.Response, error) {
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

func (p *YAMLSearchPlugin) OnPrecheckResponse(ctx context.Context, req *http.Request, rsp *http.Response) (bool, error) {
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
	return false, fmt.Errorf("status code:%d not in accept list", rsp.StatusCode)
}

func (p *YAMLSearchPlugin) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	finalReq := p.spec.finalRequest()
	decoded, err := decodeBytes(data, finalReq.decodeCharset)
	if err != nil {
		return nil, false, err
	}
	node, err := htmlquery.Parse(bytes.NewReader(decoded))
	if err != nil {
		return nil, false, err
	}
	mv, err := p.decodeHTML(ctx, node)
	if err != nil {
		return nil, false, err
	}
	if mv == nil {
		return nil, false, nil
	}
	p.applyPostprocess(ctx, mv)
	return mv, true, nil
}

func (p *YAMLSearchPlugin) OnDecorateMediaRequest(ctx context.Context, req *http.Request) error {
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

func (w *compiledSearchSelectWorkflow) handleRequest(ctx context.Context, plg *YAMLSearchPlugin, invoker pluginapi.HTTPInvoker, req *http.Request, evalCtx *evalContext) (*http.Response, error) {
	rsp, err := invoker(ctx, req)
	if err != nil {
		return nil, err
	}
	return w.handleResponse(ctx, plg, invoker, rsp, evalCtx, plg.spec.request.decodeCharset)
}

func (w *compiledSearchSelectWorkflow) handleResponse(ctx context.Context, plg *YAMLSearchPlugin, invoker pluginapi.HTTPInvoker, rsp *http.Response, evalCtx *evalContext, decodeCharset string) (*http.Response, error) {
	body, node, err := readResponseBody(rsp, decodeCharset)
	if err != nil {
		return nil, err
	}
	if pReq := plg.spec.request; pReq != nil {
		if err := checkAcceptedStatus(pReq, rsp.StatusCode); err != nil {
			return nil, err
		}
	} else if pReq := plg.spec.multiRequest; pReq != nil {
		if err := checkAcceptedStatus(pReq.request, rsp.StatusCode); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	results := make(map[string][]string, len(w.selectors))
	expectedLen := -1
	for _, sel := range w.selectors {
		items := decoder.DecodeList(node, sel.expr)
		results[sel.name] = items
		if expectedLen == -1 {
			expectedLen = len(items)
			continue
		}
		if expectedLen != len(items) {
			return nil, fmt.Errorf("selector result count mismatch")
		}
	}
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
		if !ok {
			continue
		}
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
	return nil, fmt.Errorf("no search_select result matched")
}

func (w *compiledMultiRequest) handle(ctx context.Context, plg *YAMLSearchPlugin, invoker pluginapi.HTTPInvoker, evalCtx *evalContext) (*http.Response, error) {
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
	return nil, fmt.Errorf("no multi_request matched")
}

func (p *YAMLSearchPlugin) buildRequest(ctx context.Context, spec *compiledRequest, evalCtx *evalContext) (*http.Request, error) {
	if evalCtx.host == "" {
		evalCtx.host = currentHost(ctx, p.spec.hosts)
	}
	targetURL := ""
	var err error
	if spec.rawURL != nil {
		targetURL, err = spec.rawURL.Render(evalCtx)
		if err != nil {
			return nil, err
		}
	} else {
		path, err := spec.path.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		targetURL = buildURL(evalCtx.host, path)
	}
	var body io.Reader
	switch {
	case spec.body == nil:
	case spec.body.kind == bodyKindForm:
		vals := url.Values{}
		for key, tmpl := range spec.body.values {
			rendered, err := tmpl.Render(evalCtx)
			if err != nil {
				return nil, err
			}
			vals.Set(key, rendered)
		}
		body = strings.NewReader(vals.Encode())
	case spec.body.kind == bodyKindJSON:
		payload := map[string]string{}
		for key, tmpl := range spec.body.values {
			rendered, err := tmpl.Render(evalCtx)
			if err != nil {
				return nil, err
			}
			payload[key] = rendered
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	case spec.body.kind == bodyKindRaw:
		if spec.body.content != nil {
			rendered, err := spec.body.content.Render(evalCtx)
			if err != nil {
				return nil, err
			}
			body = strings.NewReader(rendered)
		}
	}
	req, err := http.NewRequestWithContext(ctx, spec.method, targetURL, body)
	if err != nil {
		return nil, err
	}
	for key, tmpl := range spec.query {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set(key, rendered)
		req.URL.RawQuery = q.Encode()
	}
	for key, tmpl := range spec.headers {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		req.Header.Set(key, rendered)
	}
	for key, tmpl := range spec.cookies {
		rendered, err := tmpl.Render(evalCtx)
		if err != nil {
			return nil, err
		}
		req.AddCookie(&http.Cookie{Name: key, Value: rendered})
	}
	if spec.body != nil {
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
	return req, nil
}

func (p *YAMLSearchPlugin) decodeHTML(ctx context.Context, node *html.Node) (*model.MovieMeta, error) {
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	for _, field := range p.spec.scrape.fields {
		switch field.name {
		case "actors", "genres", "sample_images":
			values := decoder.DecodeList(node, field.selector.expr)
			values = applyListTransforms(values, field.transforms)
			if field.required && len(values) == 0 {
				return nil, nil
			}
			if err := assignListField(ctx, mv, field.name, values, field.parser); err != nil {
				return nil, err
			}
		default:
			value := decoder.DecodeSingle(node, field.selector.expr)
			value = applyStringTransforms(value, field.transforms)
			if field.required && strings.TrimSpace(value) == "" {
				return nil, nil
			}
			if err := assignStringField(ctx, mv, field.name, value, field.parser); err != nil {
				return nil, err
			}
		}
	}
	return mv, nil
}

func assignStringField(ctx context.Context, mv *model.MovieMeta, field, value string, parserSpec ParserSpec) error {
	switch parserSpec.Kind {
	case "", "string":
		switch field {
		case "number":
			mv.Number = value
		case "title":
			mv.Title = value
		case "plot":
			mv.Plot = value
		case "studio":
			mv.Studio = value
		case "label":
			mv.Label = value
		case "director":
			mv.Director = value
		case "series":
			mv.Series = value
		case "cover":
			mv.Cover.Name = value
		case "poster":
			mv.Poster.Name = value
		}
	case "date_only":
		mv.ReleaseDate = parser.DateOnlyReleaseDateParser(ctx)(value)
	case "duration_default":
		mv.Duration = parser.DefaultDurationParser(ctx)(value)
	case "duration_hhmmss":
		mv.Duration = parser.DefaultHHMMSSDurationParser(ctx)(value)
	case "duration_mm":
		mv.Duration = parser.DefaultMMDurationParser(ctx)(value)
	case "duration_human":
		mv.Duration = parser.HumanDurationToSecond(value)
	case "time_format":
		t, err := timeParse(parserSpec.Layout, value)
		if err != nil {
			return err
		}
		if field == "release_date" {
			mv.ReleaseDate = t
		}
	case "date_layout_soft":
		if field == "release_date" {
			mv.ReleaseDate = softTimeParse(parserSpec.Layout, value)
		}
	default:
		return fmt.Errorf("unsupported parser:%s", parserSpec.Kind)
	}
	return nil
}

func assignListField(ctx context.Context, mv *model.MovieMeta, field string, values []string, parserSpec ParserSpec) error {
	switch parserSpec.Kind {
	case "", "string_list":
		switch field {
		case "actors":
			mv.Actors = values
		case "genres":
			mv.Genres = values
		case "sample_images":
			for _, item := range values {
				mv.SampleImages = append(mv.SampleImages, &model.File{Name: item})
			}
		}
	default:
		return fmt.Errorf("unsupported list parser:%s", parserSpec.Kind)
	}
	return nil
}

func applyStringTransforms(value string, transforms []*TransformSpec) string {
	out := value
	for _, item := range transforms {
		switch item.Kind {
		case "trim":
			out = strings.TrimSpace(out)
		case "trim_prefix":
			out = strings.TrimPrefix(out, item.Value)
		case "trim_suffix":
			out = strings.TrimSuffix(out, item.Value)
		case "trim_charset":
			out = strings.Trim(out, item.Cutset)
		case "replace":
			out = strings.ReplaceAll(out, item.Old, item.New)
		case "split_index":
			parts := strings.Split(out, item.Sep)
			if item.Index >= 0 && item.Index < len(parts) {
				out = parts[item.Index]
			} else {
				out = ""
			}
		case "to_upper":
			out = strings.ToUpper(out)
		case "to_lower":
			out = strings.ToLower(out)
		}
	}
	return out
}

func applyListTransforms(values []string, transforms []*TransformSpec) []string {
	out := append([]string(nil), values...)
	for _, item := range transforms {
		switch item.Kind {
		case "remove_empty":
			filtered := make([]string, 0, len(out))
			for _, value := range out {
				if strings.TrimSpace(value) == "" {
					continue
				}
				filtered = append(filtered, value)
			}
			out = filtered
		case "dedupe":
			seen := make(map[string]struct{}, len(out))
			deduped := make([]string, 0, len(out))
			for _, value := range out {
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				deduped = append(deduped, value)
			}
			out = deduped
		case "map_trim":
			for i, value := range out {
				out[i] = strings.TrimSpace(value)
			}
		case "replace":
			for i, value := range out {
				out[i] = strings.ReplaceAll(value, item.Old, item.New)
			}
		case "split":
			split := make([]string, 0, len(out))
			for _, value := range out {
				parts := strings.Split(value, item.Sep)
				split = append(split, parts...)
			}
			out = split
		case "to_upper":
			for i, value := range out {
				out[i] = strings.ToUpper(value)
			}
		case "to_lower":
			for i, value := range out {
				out[i] = strings.ToLower(value)
			}
		}
	}
	return out
}

func (p *YAMLSearchPlugin) applyPostprocess(ctx context.Context, mv *model.MovieMeta) {
	if p.spec.postprocess == nil {
		return
	}
	metaMap := movieMetaStringMap(mv)
	if len(p.spec.postprocess.assign) != 0 {
		evalCtx := &evalContext{
			number: ctxNumber(ctx),
			host:   currentHost(ctx, p.spec.hosts),
			vars:   readVarsFromContext(ctx),
			meta:   metaMap,
		}
		for key, tmpl := range p.spec.postprocess.assign {
			value, err := tmpl.Render(evalCtx)
			if err != nil {
				continue
			}
			_ = assignStringField(ctx, mv, key, value, ParserSpec{Kind: "string"})
			metaMap = movieMetaStringMap(mv)
			evalCtx.meta = metaMap
		}
	}
	if p.spec.postprocess.defaults != nil {
		mv.TitleLang = normalizeLang(p.spec.postprocess.defaults.TitleLang)
		mv.PlotLang = normalizeLang(p.spec.postprocess.defaults.PlotLang)
		mv.GenresLang = normalizeLang(p.spec.postprocess.defaults.GenresLang)
		mv.ActorsLang = normalizeLang(p.spec.postprocess.defaults.ActorsLang)
	}
	if p.spec.postprocess.switchConfig != nil {
		mv.SwithConfig.DisableReleaseDateCheck = p.spec.postprocess.switchConfig.DisableReleaseDateCheck
		mv.SwithConfig.DisableNumberReplace = p.spec.postprocess.switchConfig.DisableNumberReplace
	}
}

func checkAcceptedStatus(spec *compiledRequest, code int) error {
	for _, item := range spec.notFoundStatusCodes {
		if code == item {
			return fmt.Errorf("status code:%d is not found", code)
		}
	}
	if len(spec.acceptStatusCodes) == 0 {
		if code != http.StatusOK {
			return fmt.Errorf("status code:%d not accepted", code)
		}
		return nil
	}
	for _, item := range spec.acceptStatusCodes {
		if code == item {
			return nil
		}
	}
	return fmt.Errorf("status code:%d not accepted", code)
}

func readResponseBody(rsp *http.Response, charset string) (string, *html.Node, error) {
	defer func() { _ = rsp.Body.Close() }()
	raw, err := client.ReadHTTPData(rsp)
	if err != nil {
		return "", nil, err
	}
	decoded, err := decodeBytes(raw, charset)
	if err != nil {
		return "", nil, err
	}
	node, err := htmlquery.Parse(bytes.NewReader(decoded))
	if err != nil {
		return "", nil, err
	}
	return string(decoded), node, nil
}

func decodeBytes(data []byte, charset string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8":
		return data, nil
	case "euc-jp":
		reader := transform.NewReader(bytes.NewReader(data), japanese.EUCJP.NewDecoder())
		return io.ReadAll(reader)
	default:
		return nil, fmt.Errorf("unsupported charset:%s", charset)
	}
}

func normalizeLang(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "":
		return ""
	case "ja":
		return enum.MetaLangJa
	case "en":
		return enum.MetaLangEn
	case "zh-cn":
		return enum.MetaLangZH
	case "zh-tw":
		return enum.MetaLangZHTW
	default:
		return in
	}
}

func buildURL(host, path string) string {
	u, err := url.Parse(host)
	if err != nil {
		return host + path
	}
	ref, err := url.Parse(path)
	if err != nil {
		return host + path
	}
	return u.ResolveReference(ref).String()
}

func movieMetaStringMap(mv *model.MovieMeta) map[string]string {
	out := map[string]string{
		"number": mv.Number,
		"title":  mv.Title,
		"plot":   mv.Plot,
		"studio": mv.Studio,
		"label":  mv.Label,
		"series": mv.Series,
	}
	if mv.Cover != nil {
		out["cover"] = mv.Cover.Name
	}
	if mv.Poster != nil {
		out["poster"] = mv.Poster.Name
	}
	return out
}

func readVarsFromContext(ctx context.Context) map[string]string {
	out := map[string]string{}
	for key, value := range pluginapi.ExportContainerData(ctx) {
		if strings.HasPrefix(key, "yamlplugin.var.") {
			out[strings.TrimPrefix(key, "yamlplugin.var.")] = value
		}
	}
	return out
}

func ctxVarKey(name string) string { return "yamlplugin.var." + name }

func currentHost(ctx context.Context, hosts []string) string {
	if host, ok := pluginapi.GetContainerValue(ctx, ctxKeyHost); ok && host != "" {
		return host
	}
	host := pluginapi.MustSelectDomain(hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	return host
}

func ctxNumber(ctx context.Context) string {
	return meta.GetNumberId(ctx)
}

func regexpMatch(pattern, value string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(value), nil
}

func timeParse(layout, value string) (int64, error) {
	t, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func softTimeParse(layout, value string) int64 {
	t, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}
