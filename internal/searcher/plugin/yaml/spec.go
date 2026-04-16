package yaml

import (
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

var errInvalidParserNodeKind = errors.New("invalid parser node kind")

const (
	typeOneStep = "one-step"
	typeTwoStep = "two-step"

	bodyKindForm = "form"
	bodyKindJSON = "json"
	bodyKindRaw  = "raw"

	formatHTML = "html"
	formatJSON = "json"
)

type PluginSpec struct {
	Version      int               `yaml:"version" json:"version"`
	Name         string            `yaml:"name" json:"name"`
	Type         string            `yaml:"type" json:"type"`
	FetchType    string            `yaml:"fetch_type" json:"fetch_type"`
	Hosts        []string          `yaml:"hosts" json:"hosts"`
	Precheck     *PrecheckSpec     `yaml:"precheck" json:"precheck"`
	Request      *RequestSpec      `yaml:"request" json:"request"`
	MultiRequest *MultiRequestSpec `yaml:"multi_request" json:"multi_request"`
	Workflow     *WorkflowSpec     `yaml:"workflow" json:"workflow"`
	Scrape       *ScrapeSpec       `yaml:"scrape" json:"scrape"`
	Postprocess  *PostprocessSpec  `yaml:"postprocess" json:"postprocess"`
}

type PrecheckSpec struct {
	NumberPatterns []string          `yaml:"number_patterns" json:"number_patterns"`
	Variables      map[string]string `yaml:"variables" json:"variables"`
}

type RequestSpec struct {
	Method              string            `yaml:"method" json:"method"`
	Path                string            `yaml:"path" json:"path"`
	URL                 string            `yaml:"url" json:"url"`
	Query               map[string]string `yaml:"query" json:"query"`
	Headers             map[string]string `yaml:"headers" json:"headers"`
	Cookies             map[string]string `yaml:"cookies" json:"cookies"`
	Body                *RequestBodySpec  `yaml:"body" json:"body"`
	AcceptStatusCodes   []int             `yaml:"accept_status_codes" json:"accept_status_codes"`
	NotFoundStatusCodes []int             `yaml:"not_found_status_codes" json:"not_found_status_codes"`
	Response            *ResponseSpec     `yaml:"response" json:"response"`
	Browser             *BrowserSpec      `yaml:"browser" json:"browser"`
}

type BrowserSpec struct {
	WaitSelector string `yaml:"wait_selector" json:"wait_selector"`
	WaitTimeout  int    `yaml:"wait_timeout" json:"wait_timeout"`
	WaitStable   int    `yaml:"wait_stable" json:"wait_stable"`
}

type RequestBodySpec struct {
	Kind    string            `yaml:"kind" json:"kind"`
	Values  map[string]string `yaml:"values" json:"values"`
	Content string            `yaml:"content" json:"content"`
}

type ResponseSpec struct {
	DecodeCharset string `yaml:"decode_charset" json:"decode_charset"`
}

type WorkflowSpec struct {
	SearchSelect *SearchSelectWorkflowSpec `yaml:"search_select" json:"search_select"`
}

type SearchSelectWorkflowSpec struct {
	Selectors     []*SelectorListSpec `yaml:"selectors" json:"selectors"`
	ItemVariables map[string]string   `yaml:"item_variables" json:"item_variables"`
	Match         *ConditionGroupSpec `yaml:"match" json:"match"`
	Return        string              `yaml:"return" json:"return"`
	NextRequest   *RequestSpec        `yaml:"next_request" json:"next_request"`
}

type MultiRequestSpec struct {
	Candidates  []string            `yaml:"candidates" json:"candidates"`
	Unique      bool                `yaml:"unique" json:"unique"`
	Request     *RequestSpec        `yaml:"request" json:"request"`
	SuccessWhen *ConditionGroupSpec `yaml:"success_when" json:"success_when"`
}

type ConditionGroupSpec struct {
	Mode       string   `yaml:"mode" json:"mode"`
	Conditions []string `yaml:"conditions" json:"conditions"`
	// ExpectCount limits the number of matched items after conditions are evaluated.
	// A zero value means the count constraint is disabled.
	ExpectCount int `yaml:"expect_count" json:"expect_count"`
}

type ScrapeSpec struct {
	Format string                `yaml:"format" json:"format"`
	Fields map[string]*FieldSpec `yaml:"fields" json:"fields"`
}

type FieldSpec struct {
	Selector   *SelectorSpec    `yaml:"selector" json:"selector"`
	Transforms []*TransformSpec `yaml:"transforms" json:"transforms"`
	Parser     ParserSpec       `yaml:"parser" json:"parser"`
	Required   bool             `yaml:"required" json:"required"`
}

type SelectorSpec struct {
	Kind  string `yaml:"kind" json:"kind"`
	Expr  string `yaml:"expr" json:"expr"`
	Multi bool   `yaml:"multi" json:"multi"`
}

type SelectorListSpec struct {
	Name string `yaml:"name" json:"name"`
	Kind string `yaml:"kind" json:"kind"`
	Expr string `yaml:"expr" json:"expr"`
}

type TransformSpec struct {
	Kind   string `yaml:"kind" json:"kind"`
	Old    string `yaml:"old" json:"old"`
	New    string `yaml:"new" json:"new"`
	Cutset string `yaml:"cutset" json:"cutset"`
	Sep    string `yaml:"sep" json:"sep"`
	Index  int    `yaml:"index" json:"index"`
	Value  string `yaml:"value" json:"value"`
}

type ParserSpec struct {
	Kind   string `yaml:"kind" json:"kind"`
	Layout string `yaml:"layout" json:"layout"`
}

func (p *ParserSpec) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind { //nolint:exhaustive // only scalar and mapping are valid parser nodes
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return fmt.Errorf("decode scalar parser: %w", err)
		}
		p.Kind = s
		return nil
	case yaml.MappingNode:
		type alias ParserSpec
		var tmp alias
		if err := node.Decode(&tmp); err != nil {
			return fmt.Errorf("decode mapping parser: %w", err)
		}
		*p = ParserSpec(tmp)
		return nil
	default:
		return fmt.Errorf("invalid parser node kind: %d: %w", node.Kind, errInvalidParserNodeKind)
	}
}

func (p *ParserSpec) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("unmarshal json string parser: %w", err)
		}
		p.Kind = s
		p.Layout = ""
		return nil
	}
	type alias ParserSpec
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("unmarshal json parser: %w", err)
	}
	*p = ParserSpec(tmp)
	return nil
}

type PostprocessSpec struct {
	Assign       map[string]string `yaml:"assign" json:"assign"`
	Defaults     *DefaultsSpec     `yaml:"defaults" json:"defaults"`
	SwitchConfig *SwitchConfigSpec `yaml:"switch_config" json:"switch_config"`
}

type DefaultsSpec struct {
	TitleLang  string `yaml:"title_lang" json:"title_lang"`
	PlotLang   string `yaml:"plot_lang" json:"plot_lang"`
	GenresLang string `yaml:"genres_lang" json:"genres_lang"`
	ActorsLang string `yaml:"actors_lang" json:"actors_lang"`
}

type SwitchConfigSpec struct {
	DisableReleaseDateCheck bool `yaml:"disable_release_date_check" json:"disable_release_date_check"`
	DisableNumberReplace    bool `yaml:"disable_number_replace" json:"disable_number_replace"`
}
