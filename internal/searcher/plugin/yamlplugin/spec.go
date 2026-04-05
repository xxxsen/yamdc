package yamlplugin

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	typeOneStep = "one-step"
	typeTwoStep = "two-step"

	bodyKindForm = "form"
	bodyKindJSON = "json"
	bodyKindRaw  = "raw"

	formatHTML = "html"
)

type PluginSpec struct {
	Version     int              `yaml:"version"`
	Name        string           `yaml:"name"`
	Type        string           `yaml:"type"`
	Hosts       []string         `yaml:"hosts"`
	Precheck    *PrecheckSpec    `yaml:"precheck"`
	Request     *RequestSpec     `yaml:"request"`
	MultiRequest *MultiRequestSpec `yaml:"multi_request"`
	Workflow    *WorkflowSpec    `yaml:"workflow"`
	Scrape      *ScrapeSpec      `yaml:"scrape"`
	Postprocess *PostprocessSpec `yaml:"postprocess"`
}

type PrecheckSpec struct {
	NumberPatterns []string          `yaml:"number_patterns"`
	Variables      map[string]string `yaml:"variables"`
}

type RequestSpec struct {
	Method              string            `yaml:"method"`
	Path                string            `yaml:"path"`
	URL                 string            `yaml:"url"`
	Query               map[string]string `yaml:"query"`
	Headers             map[string]string `yaml:"headers"`
	Cookies             map[string]string `yaml:"cookies"`
	Body                *RequestBodySpec  `yaml:"body"`
	AcceptStatusCodes   []int             `yaml:"accept_status_codes"`
	NotFoundStatusCodes []int             `yaml:"not_found_status_codes"`
	Response            *ResponseSpec     `yaml:"response"`
}

type RequestBodySpec struct {
	Kind    string            `yaml:"kind"`
	Values  map[string]string `yaml:"values"`
	Content string            `yaml:"content"`
}

type ResponseSpec struct {
	DecodeCharset string `yaml:"decode_charset"`
}

type WorkflowSpec struct {
	SearchSelect *SearchSelectWorkflowSpec `yaml:"search_select"`
}

type SearchSelectWorkflowSpec struct {
	Selectors     []*SelectorListSpec `yaml:"selectors"`
	ItemVariables map[string]string   `yaml:"item_variables"`
	Match         *ConditionGroupSpec `yaml:"match"`
	Return        string              `yaml:"return"`
	NextRequest   *RequestSpec        `yaml:"next_request"`
}

type MultiRequestSpec struct {
	Candidates  []string            `yaml:"candidates"`
	Unique      bool                `yaml:"unique"`
	Request     *RequestSpec        `yaml:"request"`
	SuccessWhen *ConditionGroupSpec `yaml:"success_when"`
}

type ConditionGroupSpec struct {
	Mode       string   `yaml:"mode"`
	Conditions []string `yaml:"conditions"`
}

type ScrapeSpec struct {
	Format string                `yaml:"format"`
	Fields map[string]*FieldSpec `yaml:"fields"`
}

type FieldSpec struct {
	Selector   *SelectorSpec    `yaml:"selector"`
	Transforms []*TransformSpec `yaml:"transforms"`
	Parser     ParserSpec       `yaml:"parser"`
	Required   bool             `yaml:"required"`
}

type SelectorSpec struct {
	Kind  string `yaml:"kind"`
	Expr  string `yaml:"expr"`
	Multi bool   `yaml:"multi"`
}

type SelectorListSpec struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"`
	Expr string `yaml:"expr"`
}

type TransformSpec struct {
	Kind   string `yaml:"kind"`
	Old    string `yaml:"old"`
	New    string `yaml:"new"`
	Cutset string `yaml:"cutset"`
	Sep    string `yaml:"sep"`
	Index  int    `yaml:"index"`
	Value  string `yaml:"value"`
}

type ParserSpec struct {
	Kind   string `yaml:"kind"`
	Layout string `yaml:"layout"`
}

func (p *ParserSpec) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		p.Kind = s
		return nil
	case yaml.MappingNode:
		type alias ParserSpec
		var tmp alias
		if err := node.Decode(&tmp); err != nil {
			return err
		}
		*p = ParserSpec(tmp)
		return nil
	default:
		return fmt.Errorf("invalid parser node kind:%d", node.Kind)
	}
}

type PostprocessSpec struct {
	Assign       map[string]string `yaml:"assign"`
	Defaults     *DefaultsSpec     `yaml:"defaults"`
	SwitchConfig *SwitchConfigSpec `yaml:"switch_config"`
}

type DefaultsSpec struct {
	TitleLang  string `yaml:"title_lang"`
	PlotLang   string `yaml:"plot_lang"`
	GenresLang string `yaml:"genres_lang"`
	ActorsLang string `yaml:"actors_lang"`
}

type SwitchConfigSpec struct {
	DisableReleaseDateCheck bool `yaml:"disable_release_date_check"`
	DisableNumberReplace    bool `yaml:"disable_number_replace"`
}
