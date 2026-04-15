package yaml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

var (
	errPrecheckNotMatched           = errors.New("precheck did not match current plugin")
	errResponseTreatedAsNotFound    = errors.New("response is treated as not found")
	errWorkflowNotConfigured        = errors.New("workflow is not configured")
	errNoMultiRequestCandidateTried = errors.New("no multi_request candidate tried")
	errMissingWorkflowBaseResponse  = errors.New("missing workflow base response")
)

type CompileSummary struct {
	HasRequest      bool   `json:"has_request"`
	HasMultiRequest bool   `json:"has_multi_request"`
	HasWorkflow     bool   `json:"has_workflow"`
	ScrapeFormat    string `json:"scrape_format"`
	FieldCount      int    `json:"field_count"`
}

type CompileResult struct {
	YAML    string         `json:"yaml"`
	Summary CompileSummary `json:"summary"`
}

type HTTPRequestDebug struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type HTTPResponseDebug struct {
	StatusCode  int                 `json:"status_code"`
	Headers     map[string][]string `json:"headers"`
	Body        string              `json:"body"`
	BodyPreview string              `json:"body_preview"`
}

type RequestDebugAttempt struct {
	Candidate string             `json:"candidate,omitempty"`
	Request   HTTPRequestDebug   `json:"request"`
	Response  *HTTPResponseDebug `json:"response,omitempty"`
	Matched   bool               `json:"matched"`
	Error     string             `json:"error,omitempty"`
}

type RequestDebugResult struct {
	Candidate string                `json:"candidate,omitempty"`
	Request   HTTPRequestDebug      `json:"request"`
	Response  *HTTPResponseDebug    `json:"response,omitempty"`
	Attempts  []RequestDebugAttempt `json:"attempts,omitempty"`
}

type TransformStep struct {
	Kind   string      `json:"kind"`
	Input  interface{} `json:"input"`
	Output interface{} `json:"output"`
}

type FieldDebugResult struct {
	SelectorValues []string        `json:"selector_values"`
	TransformSteps []TransformStep `json:"transform_steps"`
	ParserResult   interface{}     `json:"parser_result,omitempty"`
	Required       bool            `json:"required"`
	Matched        bool            `json:"matched"`
}

type ScrapeDebugResult struct {
	Request  HTTPRequestDebug            `json:"request"`
	Response *HTTPResponseDebug          `json:"response,omitempty"`
	Fields   map[string]FieldDebugResult `json:"fields"`
	Meta     *model.MovieMeta            `json:"meta,omitempty"`
	Error    string                      `json:"error,omitempty"`
}

type WorkflowMatchDetail struct {
	Condition string `json:"condition"`
	Pass      bool   `json:"pass"`
}

type WorkflowSelectorItem struct {
	Index         int                   `json:"index"`
	Item          map[string]string     `json:"item"`
	ItemVariables map[string]string     `json:"item_variables,omitempty"`
	Matched       bool                  `json:"matched"`
	MatchDetails  []WorkflowMatchDetail `json:"match_details,omitempty"`
}

type WorkflowDebugStep struct {
	Stage         string                 `json:"stage"`
	Summary       string                 `json:"summary"`
	Candidate     string                 `json:"candidate,omitempty"`
	Request       *HTTPRequestDebug      `json:"request,omitempty"`
	Response      *HTTPResponseDebug     `json:"response,omitempty"`
	Selectors     map[string][]string    `json:"selectors,omitempty"`
	Items         []WorkflowSelectorItem `json:"items,omitempty"`
	SelectedValue string                 `json:"selected_value,omitempty"`
}

type WorkflowDebugResult struct {
	Steps []WorkflowDebugStep `json:"steps"`
	Error string              `json:"error,omitempty"`
}

type CaseOutput struct {
	Title    string   `json:"title"`
	TagSet   []string `json:"tag_set"`
	ActorSet []string `json:"actor_set"`
	Status   string   `json:"status"`
}

type CaseSpec struct {
	Name   string     `json:"name"`
	Input  string     `json:"input"`
	Output CaseOutput `json:"output"`
}

type CaseDebugResult struct {
	Pass   bool             `json:"pass"`
	Errmsg string           `json:"errmsg"`
	Meta   *model.MovieMeta `json:"meta,omitempty"`
}

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

type scrapeHTTPResult struct {
	finalReq   *http.Request
	decoded    []byte
	statusCode int
	headers    http.Header
}

func debugScrapeFetch(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin, number string,
) (*scrapeHTTPResult, error) {
	ok, err := plg.OnPrecheckRequest(ctx, strings.TrimSpace(number))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errPrecheckNotMatched
	}
	req, err := plg.OnMakeHTTPRequest(ctx, number)
	if err != nil {
		return nil, err
	}
	finalReq := req
	invoker := func(_ context.Context, target *http.Request) (*http.Response, error) {
		finalReq = target
		return cli.Do(target)
	}
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rsp.Body.Close() }()
	ok, err = plg.OnPrecheckResponse(ctx, finalReq, rsp)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errResponseTreatedAsNotFound
	}
	rawData, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read response data: %w", err)
	}
	decoded, err := decodeBytes(rawData, plg.spec.finalRequest().decodeCharset)
	if err != nil {
		return nil, err
	}
	return &scrapeHTTPResult{
		finalReq:   finalReq,
		decoded:    decoded,
		statusCode: rsp.StatusCode,
		headers:    rsp.Header,
	}, nil
}

func debugScrapeDecodeFields(ctx context.Context, plg *SearchPlugin, result *ScrapeDebugResult, decoded []byte) {
	switch plg.spec.scrape.format {
	case formatHTML:
		node, err := htmlquery.Parse(bytes.NewReader(decoded))
		if err != nil {
			result.Error = fmt.Sprintf("parse html failed: %s", err.Error())
			return
		}
		result.Meta, err = plg.traceDecodeHTML(ctx, node, result.Fields)
		if err != nil {
			result.Error = fmt.Sprintf("trace html fields failed: %s", err.Error())
			return
		}
	case formatJSON:
		var err error
		result.Meta, err = plg.traceDecodeJSON(ctx, decoded, result.Fields)
		if err != nil {
			result.Error = fmt.Sprintf("trace json fields failed: %s", err.Error())
			return
		}
	default:
		result.Error = fmt.Sprintf("unsupported scrape format:%s", plg.spec.scrape.format)
	}
}

func DebugScrape(
	ctx context.Context, cli client.IHTTPClient, raw *PluginSpec, number string,
) (*ScrapeDebugResult, error) {
	plg, err := newCompiledPlugin(raw)
	if err != nil {
		return nil, err
	}
	ctx = pluginapi.InitContainer(ctx)
	ctx = meta.SetNumberID(ctx, number)
	fetch, err := debugScrapeFetch(ctx, cli, plg, number)
	if err != nil {
		return nil, err
	}
	result := &ScrapeDebugResult{
		Request: requestDebug(fetch.finalReq),
		Response: &HTTPResponseDebug{
			StatusCode:  fetch.statusCode,
			Headers:     cloneHeader(fetch.headers),
			Body:        string(fetch.decoded),
			BodyPreview: previewBody(string(fetch.decoded)),
		},
		Fields: make(map[string]FieldDebugResult, len(plg.spec.scrape.fields)),
	}
	debugScrapeDecodeFields(ctx, plg, result, fetch.decoded)
	if result.Meta != nil {
		plg.applyPostprocess(ctx, result.Meta)
	}
	return result, nil
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
	result := &WorkflowDebugResult{}
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

func DebugCase(ctx context.Context, cli client.IHTTPClient, raw *PluginSpec, spec CaseSpec) (*CaseDebugResult, error) {
	scrape, err := DebugScrape(ctx, cli, raw, spec.Input)
	if err != nil {
		if strings.EqualFold(strings.TrimSpace(spec.Output.Status), "error") {
			return &CaseDebugResult{Pass: true}, nil
		}
		return &CaseDebugResult{Pass: false, Errmsg: err.Error()}, nil
	}
	status := "not_found"
	if scrape.Meta != nil {
		status = "success"
	}
	if expected := strings.TrimSpace(spec.Output.Status); expected != "" && !strings.EqualFold(expected, status) {
		return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
			"expected status=%s but got %s",
			expected,
			status,
		), Meta: scrape.Meta}, nil
	}
	if expected := strings.TrimSpace(spec.Output.Title); expected != "" {
		got := ""
		if scrape.Meta != nil {
			got = strings.TrimSpace(scrape.Meta.Title)
		}
		if got != expected {
			return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
				"expected title=%s but got %s",
				expected,
				got,
			), Meta: scrape.Meta}, nil
		}
	}
	if len(spec.Output.TagSet) != 0 {
		got := []string(nil)
		if scrape.Meta != nil {
			got = scrape.Meta.Genres
		}
		if !equalNormalizedSet(spec.Output.TagSet, got) {
			return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
				"expected tag_set=%v but got %v",
				normalizeStringSet(spec.Output.TagSet),
				normalizeStringSet(got),
			), Meta: scrape.Meta}, nil
		}
	}
	if len(spec.Output.ActorSet) != 0 {
		got := []string(nil)
		if scrape.Meta != nil {
			got = scrape.Meta.Actors
		}
		if !equalNormalizedSet(spec.Output.ActorSet, got) {
			return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
				"expected actor_set=%v but got %v",
				normalizeStringSet(spec.Output.ActorSet),
				normalizeStringSet(got),
			), Meta: scrape.Meta}, nil
		}
	}
	return &CaseDebugResult{Pass: true, Meta: scrape.Meta}, nil
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

func captureHTTPResponse(rsp *http.Response, charset string) (*HTTPResponseDebug, error) {
	defer func() { _ = rsp.Body.Close() }()
	raw, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read http data: %w", err)
	}
	decoded, err := decodeBytes(raw, charset)
	if err != nil {
		return nil, err
	}
	return &HTTPResponseDebug{
		StatusCode:  rsp.StatusCode,
		Headers:     cloneHeader(rsp.Header),
		Body:        string(decoded),
		BodyPreview: previewBody(string(decoded)),
	}, nil
}

func requestDebug(req *http.Request) HTTPRequestDebug {
	headers := make(map[string]string, len(req.Header))
	for key, values := range req.Header {
		headers[key] = strings.Join(values, ", ")
	}
	body := ""
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
		req.Body = io.NopCloser(bytes.NewReader(raw))
	}
	return HTTPRequestDebug{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    body,
	}
}

func cloneHeader(in http.Header) map[string][]string {
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func previewBody(body string) string {
	const maxLen = 4000
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen]
}

func (p *SearchPlugin) traceDecodeHTML(ctx context.Context, node *html.Node, out map[string]FieldDebugResult) (
	*model.MovieMeta,
	error,
) {
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	fieldNames := make([]string, 0, len(p.spec.scrape.fields))
	for _, field := range p.spec.scrape.fields {
		fieldNames = append(fieldNames, field.name)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := p.fieldByName(fieldName)
		dbg, err := p.traceFieldHTML(ctx, mv, node, field)
		if err != nil {
			return nil, err
		}
		out[field.name] = dbg
		if field.required && !dbg.Matched {
			return nil, nil //nolint:nilnil // nil signals "not found" to caller
		}
	}
	return mv, nil
}

func (p *SearchPlugin) traceDecodeJSON(ctx context.Context, data []byte, out map[string]FieldDebugResult) (
	*model.MovieMeta,
	error,
) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode json data failed, err:%w", err)
	}
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	fieldNames := make([]string, 0, len(p.spec.scrape.fields))
	for _, field := range p.spec.scrape.fields {
		fieldNames = append(fieldNames, field.name)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := p.fieldByName(fieldName)
		dbg, err := p.traceFieldJSON(ctx, mv, doc, field)
		if err != nil {
			return nil, err
		}
		out[field.name] = dbg
		if field.required && !dbg.Matched {
			return nil, nil //nolint:nilnil // nil signals "not found" to caller
		}
	}
	return mv, nil
}

func (p *SearchPlugin) fieldByName(name string) *compiledField {
	for _, field := range p.spec.scrape.fields {
		if field.name == name {
			return field
		}
	}
	return nil
}

func (p *SearchPlugin) traceFieldHTML(ctx context.Context, mv *model.MovieMeta, node *html.Node, field *compiledField) (
	FieldDebugResult,
	error,
) {
	if isListField(field.name) {
		values := decoder.DecodeList(node, field.selector.expr)
		steps := make([]TransformStep, 0, len(field.transforms))
		out := traceListTransforms(values, field.transforms, &steps)
		dbg := FieldDebugResult{
			SelectorValues: values,
			TransformSteps: steps,
			Required:       field.required,
			Matched:        len(out) > 0,
			ParserResult:   append([]string(nil), out...),
		}
		if len(out) > 0 {
			if err := assignListField(ctx, mv, field.name, out, field.parser); err != nil {
				return dbg, err
			}
		}
		return dbg, nil
	}
	value := decoder.DecodeSingle(node, field.selector.expr)
	steps := make([]TransformStep, 0, len(field.transforms))
	out := traceStringTransforms(value, field.transforms, &steps)
	dbg := FieldDebugResult{
		SelectorValues: []string{value},
		TransformSteps: steps,
		Required:       field.required,
		Matched:        strings.TrimSpace(out) != "",
	}
	parserResult, err := traceAssignStringField(ctx, mv, field.name, out, field.parser)
	dbg.ParserResult = parserResult
	return dbg, err
}

func (p *SearchPlugin) traceFieldJSON(ctx context.Context, mv *model.MovieMeta, doc any, field *compiledField) (
	FieldDebugResult,
	error,
) {
	values, err := evalJSONPathStrings(doc, field.selector.expr)
	if err != nil {
		return FieldDebugResult{}, err
	}
	if isListField(field.name) {
		steps := make([]TransformStep, 0, len(field.transforms))
		out := traceListTransforms(values, field.transforms, &steps)
		dbg := FieldDebugResult{
			SelectorValues: values,
			TransformSteps: steps,
			Required:       field.required,
			Matched:        len(out) > 0,
			ParserResult:   append([]string(nil), out...),
		}
		if len(out) > 0 {
			if err := assignListField(ctx, mv, field.name, out, field.parser); err != nil {
				return dbg, err
			}
		}
		return dbg, nil
	}
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	steps := make([]TransformStep, 0, len(field.transforms))
	out := traceStringTransforms(value, field.transforms, &steps)
	dbg := FieldDebugResult{
		SelectorValues: values,
		TransformSteps: steps,
		Required:       field.required,
		Matched:        strings.TrimSpace(out) != "",
	}
	parserResult, err := traceAssignStringField(ctx, mv, field.name, out, field.parser)
	dbg.ParserResult = parserResult
	return dbg, err
}

func isListField(field string) bool {
	switch field {
	case "actors", "genres", "sample_images":
		return true
	default:
		return false
	}
}

func traceStringTransforms(value string, transforms []*TransformSpec, steps *[]TransformStep) string {
	out := value
	for _, item := range transforms {
		input := out
		out = applyStringTransforms(out, []*TransformSpec{item})
		*steps = append(*steps, TransformStep{
			Kind:   item.Kind,
			Input:  input,
			Output: out,
		})
	}
	return out
}

func traceListTransforms(values []string, transforms []*TransformSpec, steps *[]TransformStep) []string {
	out := append([]string(nil), values...)
	for _, item := range transforms {
		input := append([]string(nil), out...)
		out = applyListTransforms(out, []*TransformSpec{item})
		*steps = append(*steps, TransformStep{
			Kind:   item.Kind,
			Input:  input,
			Output: append([]string(nil), out...),
		})
	}
	return out
}

func traceAssignStringField(
	ctx context.Context,
	mv *model.MovieMeta,
	field,
	value string,
	parserSpec ParserSpec,
) (interface{}, error) {
	if strings.TrimSpace(value) == "" && (parserSpec.Kind == "" || parserSpec.Kind == "string") {
		return value, nil
	}
	if err := assignStringField(ctx, mv, field, value, parserSpec); err != nil {
		return nil, err
	}
	switch parserSpec.Kind {
	case "", "string":
		return value, nil
	case "date_only", "time_format", "date_layout_soft":
		return mv.ReleaseDate, nil
	case "duration_default", "duration_hhmmss", "duration_mm", "duration_human", "duration_mmss":
		return mv.Duration, nil
	default:
		return value, nil
	}
}

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

func renderCondition(cond *compiledCondition) string {
	if cond == nil {
		return ""
	}
	return cond.name
}

func ptrRequestDebug(v HTTPRequestDebug) *HTTPRequestDebug {
	return &v
}

func normalizeStringSet(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func equalNormalizedSet(a, b []string) bool {
	na := normalizeStringSet(a)
	nb := normalizeStringSet(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}
