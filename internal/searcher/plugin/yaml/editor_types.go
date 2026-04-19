package yaml

import "github.com/xxxsen/yamdc/internal/model"

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
	Kind   string `json:"kind"`
	Input  any    `json:"input"`
	Output any    `json:"output"`
}

type FieldDebugResult struct {
	SelectorValues []string        `json:"selector_values"`
	TransformSteps []TransformStep `json:"transform_steps"`
	ParserResult   any             `json:"parser_result,omitempty"`
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
