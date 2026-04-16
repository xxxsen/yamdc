package yaml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/xxxsen/yamdc/internal/browser"
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

var (
	errUnsupportedVersion              = errors.New("unsupported version")
	errNameRequired                    = errors.New("name is required")
	errInvalidPluginType               = errors.New("invalid plugin type")
	errHostsRequired                   = errors.New("hosts is required")
	errRequestAndMultiRequestExclusive = errors.New("request and multi_request are mutually exclusive")
	errTwoStepRequiresSearchSelect     = errors.New("two-step requires workflow.search_select")
	errRequestOrMultiRequestRequired   = errors.New("request or multi_request is required")
	errInvalidFetchType                = errors.New("invalid fetch_type")
	errRequestMethodRequired           = errors.New("request method is required")
	errRequestPathOrURLRequired        = errors.New("request path or url is required")
	errRequestPathAndURLExclusive      = errors.New("request path and url are mutually exclusive")
	errUnsupportedRequestMethod        = errors.New("unsupported request method")
	errUnsupportedBodyKind             = errors.New("unsupported body kind")
	errSearchSelectRequiresSelector    = errors.New("search_select requires at least 1 selector")
	errSearchSelectNextRequestRequired = errors.New("search_select next_request is required")
	errUnsupportedSelectorKind         = errors.New("unsupported selector kind")
	errMultiRequestCandidatesRequired  = errors.New("multi_request candidates is required")
	errMultiRequestRequestRequired     = errors.New("multi_request request is required")
	errScrapeRequired                  = errors.New("scrape is required")
	errUnsupportedScrapeFormat         = errors.New("unsupported scrape format")
	errScrapeFieldsRequired            = errors.New("scrape fields is required")
	errFieldSelectorRequired           = errors.New("field selector is required")
	errFieldSelectorKindUnsupported    = errors.New("field selector kind unsupported")
	errRequestNil                      = errors.New("request is nil")
	errStatusCodeNotAccepted           = errors.New("status code not accepted")
	errStatusCodeNotFound              = errors.New("status code not found")
	errSelectorCountMismatch           = errors.New("selector result count mismatch")
	errSearchSelectCountMismatch       = errors.New("search_select matched count mismatch")
	errNoSearchSelectMatched           = errors.New("no search_select result matched")
	errNoMultiRequestMatched           = errors.New("no multi_request matched")
	errUnsupportedParser               = errors.New("unsupported parser")
	errUnsupportedListParser           = errors.New("unsupported list parser")
	errUnsupportedCharset              = errors.New("unsupported charset")
)

const (
	ctxKeyHost        = "yaml.host"
	ctxKeyFinalPage   = "yaml.final_page"
	ctxKeyRequestPath = "yaml.request_path"
)

const (
	fetchTypeGoHTTP  = "go-http"
	fetchTypeBrowser = "browser"
)

type compiledPlugin struct {
	version      int
	name         string
	pluginType   string
	fetchType    string
	hosts        []string
	precheck     *compiledPrecheck
	request      *compiledRequest
	multiRequest *compiledMultiRequest
	workflow     *compiledSearchSelectWorkflow
	scrape       *compiledScrape
	postprocess  *compiledPostprocess
}

type compiledPrecheck struct {
	numberPatterns []string
	variables      map[string]*template
}

type compiledBrowser struct {
	waitSelector string
	waitTimeout  time.Duration
	waitStable   time.Duration
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
	browser             *compiledBrowser
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

type SearchPlugin struct {
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
	return &SearchPlugin{spec: spec}, nil
}

func validatePluginSpec(raw *PluginSpec) (string, error) {
	if raw.Version != 1 {
		return "", fmt.Errorf("%w: %d", errUnsupportedVersion, raw.Version)
	}
	if raw.Name == "" {
		return "", errNameRequired
	}
	if raw.Type != typeOneStep && raw.Type != typeTwoStep {
		return "", fmt.Errorf("%w: %s", errInvalidPluginType, raw.Type)
	}
	if len(raw.Hosts) == 0 {
		return "", errHostsRequired
	}
	if raw.Request != nil && raw.MultiRequest != nil {
		return "", errRequestAndMultiRequestExclusive
	}
	if raw.Type == typeTwoStep && (raw.Workflow == nil || raw.Workflow.SearchSelect == nil) {
		return "", errTwoStepRequiresSearchSelect
	}
	if raw.Request == nil && raw.MultiRequest == nil {
		return "", errRequestOrMultiRequestRequired
	}
	ft := raw.FetchType
	if ft == "" {
		ft = fetchTypeGoHTTP
	}
	if ft != fetchTypeGoHTTP && ft != fetchTypeBrowser {
		return "", fmt.Errorf("%w: %s", errInvalidFetchType, ft)
	}
	return ft, nil
}

func compilePlugin(raw *PluginSpec) (*compiledPlugin, error) {
	ft, err := validatePluginSpec(raw)
	if err != nil {
		return nil, err
	}
	out := &compiledPlugin{
		version:    raw.Version,
		name:       raw.Name,
		pluginType: raw.Type,
		fetchType:  ft,
		hosts:      append([]string(nil), raw.Hosts...),
	}
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
		return nil, nil //nolint:nilnil // nil spec means no precheck configured
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

func compileTemplateMap(raw map[string]string) (map[string]*template, error) {
	out := make(map[string]*template, len(raw))
	for k, v := range raw {
		t, err := compileTemplate(v)
		if err != nil {
			return nil, err
		}
		out[k] = t
	}
	return out, nil
}

func compileRequest(raw *RequestSpec) (*compiledRequest, error) {
	if raw == nil {
		return nil, nil //nolint:nilnil // nil spec means no request configured
	}
	if raw.Method == "" {
		return nil, errRequestMethodRequired
	}
	if raw.Path == "" && raw.URL == "" {
		return nil, errRequestPathOrURLRequired
	}
	if raw.Path != "" && raw.URL != "" {
		return nil, errRequestPathAndURLExclusive
	}
	out := &compiledRequest{
		method:              strings.ToUpper(raw.Method),
		acceptStatusCodes:   append([]int(nil), raw.AcceptStatusCodes...),
		notFoundStatusCodes: append([]int(nil), raw.NotFoundStatusCodes...),
	}
	if out.method != http.MethodGet && out.method != http.MethodPost {
		return nil, fmt.Errorf("%w: %s", errUnsupportedRequestMethod, out.method)
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
	if out.query, err = compileTemplateMap(raw.Query); err != nil {
		return nil, err
	}
	if out.headers, err = compileTemplateMap(raw.Headers); err != nil {
		return nil, err
	}
	if out.cookies, err = compileTemplateMap(raw.Cookies); err != nil {
		return nil, err
	}
	if raw.Body != nil {
		if out.body, err = compileRequestBody(raw.Body); err != nil {
			return nil, err
		}
	}
	if raw.Response != nil {
		out.decodeCharset = strings.ToLower(strings.TrimSpace(raw.Response.DecodeCharset))
	}
	if raw.Browser != nil {
		out.browser = &compiledBrowser{
			waitSelector: raw.Browser.WaitSelector,
			waitTimeout:  time.Duration(raw.Browser.WaitTimeout) * time.Second,
			waitStable:   time.Duration(raw.Browser.WaitStable) * time.Second,
		}
	}
	return out, nil
}

func compileRequestBody(raw *RequestBodySpec) (*compiledRequestBody, error) {
	out := &compiledRequestBody{kind: raw.Kind, values: make(map[string]*template, len(raw.Values))}
	switch raw.Kind {
	case bodyKindForm, bodyKindJSON, bodyKindRaw:
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedBodyKind, raw.Kind)
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
		return nil, nil //nolint:nilnil // nil spec means no workflow configured
	}
	if raw.SearchSelect != nil {
		return compileSearchSelect(raw.SearchSelect)
	}
	return nil, nil //nolint:nilnil // no search_select means no workflow
}

func compileSearchSelect(raw *SearchSelectWorkflowSpec) (*compiledSearchSelectWorkflow, error) {
	if len(raw.Selectors) < 1 {
		return nil, errSearchSelectRequiresSelector
	}
	if raw.NextRequest == nil {
		return nil, errSearchSelectNextRequestRequired
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
			return nil, fmt.Errorf("%w: %s", errUnsupportedSelectorKind, item.Kind)
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
		return nil, errMultiRequestCandidatesRequired
	}
	if raw.Request == nil {
		return nil, errMultiRequestRequestRequired
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
		return nil, errScrapeRequired
	}
	if raw.Format != formatHTML && raw.Format != formatJSON {
		return nil, fmt.Errorf("%w: %s", errUnsupportedScrapeFormat, raw.Format)
	}
	if len(raw.Fields) == 0 {
		return nil, errScrapeFieldsRequired
	}
	out := &compiledScrape{format: raw.Format}
	ordered := []string{
		"number", "title", "plot", "actors", "release_date", "duration", "studio", "label", "director",
		"series", "genres", "cover", "poster", "sample_images",
	}
	for _, name := range ordered {
		spec, ok := raw.Fields[name]
		if !ok {
			continue
		}
		if spec.Selector == nil {
			return nil, fmt.Errorf("field %s: %w", name, errFieldSelectorRequired)
		}
		if raw.Format == formatHTML && spec.Selector.Kind != "xpath" {
			return nil, fmt.Errorf("field %s %w: %s", name, errFieldSelectorKindUnsupported, spec.Selector.Kind)
		}
		if raw.Format == formatJSON && spec.Selector.Kind != "jsonpath" {
			return nil, fmt.Errorf("field %s %w: %s", name, errFieldSelectorKindUnsupported, spec.Selector.Kind)
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
		return nil, nil //nolint:nilnil // nil spec means no postprocess configured
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

func (p *SearchPlugin) applyBrowserContext(req *http.Request, spec *compiledRequest) *http.Request {
	if p.spec.fetchType != fetchTypeBrowser {
		return req
	}
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
	req = p.applyBrowserContext(req, spec)
	return req, nil
}

func (p *SearchPlugin) decodeHTML(ctx context.Context, node *html.Node) (*model.MovieMeta, error) {
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
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignListField(ctx, mv, field.name, values, field.parser); err != nil {
				return nil, err
			}
		default:
			value := decoder.DecodeSingle(node, field.selector.expr)
			value = applyStringTransforms(value, field.transforms)
			if field.required && strings.TrimSpace(value) == "" {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignStringField(ctx, mv, field.name, value, field.parser); err != nil {
				return nil, err
			}
		}
	}
	return mv, nil
}

func (p *SearchPlugin) decodeJSON(ctx context.Context, data []byte) (*model.MovieMeta, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode json data failed, err:%w", err)
	}
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	for _, field := range p.spec.scrape.fields {
		values, err := evalJSONPathStrings(doc, field.selector.expr)
		if err != nil {
			return nil, err
		}
		switch field.name {
		case "actors", "genres", "sample_images":
			values = applyListTransforms(values, field.transforms)
			if field.required && len(values) == 0 {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignListField(ctx, mv, field.name, values, field.parser); err != nil {
				return nil, err
			}
		default:
			value := ""
			if len(values) > 0 {
				value = values[0]
			}
			value = applyStringTransforms(value, field.transforms)
			if field.required && strings.TrimSpace(value) == "" {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignStringField(ctx, mv, field.name, value, field.parser); err != nil {
				return nil, err
			}
		}
	}
	return mv, nil
}

func assignStringFieldByName(mv *model.MovieMeta, field, value string) {
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
}

func parseDurationMMSS(value string) int64 {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0
	}
	minutes, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0
	}
	return minutes*60 + sec
}

func parseDurationByKind(ctx context.Context, kind, value string) int64 {
	switch kind {
	case "duration_default":
		return parser.DefaultDurationParser(ctx)(value)
	case "duration_hhmmss":
		return parser.DefaultHHMMSSDurationParser(ctx)(value)
	case "duration_mm":
		return parser.DefaultMMDurationParser(ctx)(value)
	case "duration_human":
		return parser.HumanDurationToSecond(value)
	case "duration_mmss":
		return parseDurationMMSS(value)
	default:
		return 0
	}
}

func assignDateField(mv *model.MovieMeta, field string, parserSpec ParserSpec, value string) error {
	switch parserSpec.Kind {
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
	}
	return nil
}

func assignStringField(ctx context.Context, mv *model.MovieMeta, field, value string, parserSpec ParserSpec) error {
	switch parserSpec.Kind {
	case "", "string":
		assignStringFieldByName(mv, field, value)
	case "date_only":
		mv.ReleaseDate = parser.DateOnlyReleaseDateParser(ctx)(value)
	case "duration_default", "duration_hhmmss", "duration_mm", "duration_human", "duration_mmss":
		mv.Duration = parseDurationByKind(ctx, parserSpec.Kind, value)
	case "time_format", "date_layout_soft":
		return assignDateField(mv, field, parserSpec, value)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedParser, parserSpec.Kind)
	}
	return nil
}

func assignListField(
	_ context.Context, mv *model.MovieMeta, field string, values []string, parserSpec ParserSpec,
) error {
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
		return fmt.Errorf("%w: %s", errUnsupportedListParser, parserSpec.Kind)
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
		case "regex_extract":
			re, err := regexp.Compile(item.Value)
			if err != nil {
				out = ""
				continue
			}
			matches := re.FindStringSubmatch(out)
			if item.Index >= 0 && item.Index < len(matches) {
				out = matches[item.Index]
			} else {
				out = ""
			}
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

func applyOneListTransform(out []string, item *TransformSpec) []string {
	switch item.Kind {
	case "remove_empty":
		filtered := make([]string, 0, len(out))
		for _, value := range out {
			if strings.TrimSpace(value) != "" {
				filtered = append(filtered, value)
			}
		}
		return filtered
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
		return deduped
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
			split = append(split, strings.Split(value, item.Sep)...)
		}
		return split
	case "to_upper":
		for i, value := range out {
			out[i] = strings.ToUpper(value)
		}
	case "to_lower":
		for i, value := range out {
			out[i] = strings.ToLower(value)
		}
	}
	return out
}

func applyListTransforms(values []string, transforms []*TransformSpec) []string {
	out := append([]string(nil), values...)
	for _, item := range transforms {
		out = applyOneListTransform(out, item)
	}
	return out
}

func (p *SearchPlugin) applyPostprocess(ctx context.Context, mv *model.MovieMeta) {
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
			return fmt.Errorf("status code %d: %w", code, errStatusCodeNotFound)
		}
	}
	if len(spec.acceptStatusCodes) == 0 {
		if code != http.StatusOK {
			return fmt.Errorf("status code %d: %w", code, errStatusCodeNotAccepted)
		}
		return nil
	}
	for _, item := range spec.acceptStatusCodes {
		if code == item {
			return nil
		}
	}
	return fmt.Errorf("status code %d: %w", code, errStatusCodeNotAccepted)
}

func readResponseBody(rsp *http.Response, charset string) (string, *html.Node, error) {
	defer func() { _ = rsp.Body.Close() }()
	raw, err := client.ReadHTTPData(rsp)
	if err != nil {
		return "", nil, fmt.Errorf("read response body: %w", err)
	}
	decoded, err := decodeBytes(raw, charset)
	if err != nil {
		return "", nil, err
	}
	node, err := htmlquery.Parse(bytes.NewReader(decoded))
	if err != nil {
		return "", nil, fmt.Errorf("parse html: %w", err)
	}
	return string(decoded), node, nil
}

func decodeBytes(data []byte, charset string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8":
		return data, nil
	case "euc-jp":
		reader := transform.NewReader(bytes.NewReader(data), japanese.EUCJP.NewDecoder())
		out, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("decode euc-jp: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedCharset, charset)
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
		if strings.HasPrefix(key, "yaml.var.") {
			out[strings.TrimPrefix(key, "yaml.var.")] = value
		}
	}
	return out
}

func ctxVarKey(name string) string { return "yaml.var." + name }

func currentHost(ctx context.Context, hosts []string) string {
	if host, ok := pluginapi.GetContainerValue(ctx, ctxKeyHost); ok && host != "" {
		return host
	}
	host := pluginapi.MustSelectDomain(hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	return host
}

func ctxNumber(ctx context.Context) string {
	return meta.GetNumberID(ctx)
}

func regexpMatch(pattern, value string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compile regexp: %w", err)
	}
	return re.MatchString(value), nil
}

func timeParse(layout, value string) (int64, error) {
	t, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("parse time %q: %w", value, err)
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
