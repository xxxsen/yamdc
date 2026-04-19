package yaml

import (
	"errors"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
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
	fetchTypeGoHTTP       = "go-http"
	fetchTypeBrowser      = "browser"
	fetchTypeFlaresolverr = "flaresolverr"
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

// validatePluginSpec 是按顺序叠加的 yaml 结构校验, 每条 if 对应一种不同配置错误,
// 拆分后每个 helper 只剩 1-2 行校验, 反而让读者要跳多个函数才能看完一份 spec 的
// 合法性规则.
//
//nolint:gocyclo // sequential validation pipeline, each branch reports a distinct spec error
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
	if ft != fetchTypeGoHTTP && ft != fetchTypeBrowser && ft != fetchTypeFlaresolverr {
		return "", fmt.Errorf("%w: %s", errInvalidFetchType, ft)
	}
	return ft, nil
}
