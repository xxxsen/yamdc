package yaml

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

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
