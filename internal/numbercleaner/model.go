package numbercleaner

type Status string

const (
	StatusSuccess    Status = "success"
	StatusNoMatch    Status = "no_match"
	StatusLowQuality Status = "low_quality"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type ErrorCode string

const (
	ErrInvalidRuleSet ErrorCode = "invalid_rule_set"
	ErrInvalidOutput  ErrorCode = "invalid_output"
	ErrInternal       ErrorCode = "internal"
)

type CleanError struct {
	Code    ErrorCode
	Message string
	Rule    string
	Cause   error
}

func (e *CleanError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Message
	}
	if len(e.Message) == 0 {
		return e.Cause.Error()
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *CleanError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type Result struct {
	RawInput   string
	InputNoExt string
	Normalized string
	NumberID   string
	Suffixes   []string
	Category   string
	Uncensor   bool

	CategoryMatched bool
	UncensorMatched bool
	Confidence      Confidence
	Status          Status
	RuleHits        []string
	Warnings        []string
	Candidates      []Candidate
}

type Candidate struct {
	NumberID string
	Score    int
	RuleHits []string
	Matcher  string
	Start    int
	End      int

	Category        string
	CategoryMatched bool
	Uncensor        bool
	UncensorMatched bool
}

type Options struct {
	CaseMode            string `yaml:"case_mode"`
	CollapseSpaces      bool   `yaml:"collapse_spaces"`
	EnableEmbeddedMatch bool   `yaml:"enable_embedded_match"`
	FailWhenNoMatch     bool   `yaml:"fail_when_no_match"`
}

type RuleSet struct {
	Version        string            `yaml:"version"`
	Options        Options           `yaml:"options"`
	Normalizers    []NormalizerRule  `yaml:"normalizers"`
	RewriteRules   []RewriteRule     `yaml:"rewrite_rules"`
	SuffixRules    []SuffixRule      `yaml:"suffix_rules"`
	NoiseRules     []NoiseRule       `yaml:"noise_rules"`
	Matchers       []MatcherRule     `yaml:"matchers"`
	PostProcessors []PostProcessRule `yaml:"post_processors"`
}

type NormalizerRule struct {
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"`
	Builtin  string            `yaml:"builtin"`
	Pairs    map[string]string `yaml:"pairs"`
	Disabled bool              `yaml:"disabled"`
}

type RewriteRule struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Replace  string `yaml:"replace"`
	Disabled bool   `yaml:"disabled"`
}

type SuffixRule struct {
	Name              string   `yaml:"name"`
	Type              string   `yaml:"type"`
	Aliases           []string `yaml:"aliases"`
	Pattern           string   `yaml:"pattern"`
	Canonical         string   `yaml:"canonical"`
	CanonicalTemplate string   `yaml:"canonical_template"`
	Priority          int      `yaml:"priority"`
	Disabled          bool     `yaml:"disabled"`
}

type NoiseRule struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Aliases  []string `yaml:"aliases"`
	Pattern  string   `yaml:"pattern"`
	Disabled bool     `yaml:"disabled"`
}

type MatcherRule struct {
	Name              string   `yaml:"name"`
	Category          string   `yaml:"category"`
	Uncensor          *bool    `yaml:"uncensor"`
	Pattern           string   `yaml:"pattern"`
	NormalizeTemplate string   `yaml:"normalize_template"`
	Score             int      `yaml:"score"`
	RequireBoundary   bool     `yaml:"require_boundary"`
	Prefixes          []string `yaml:"prefixes"`
	Disabled          bool     `yaml:"disabled"`
}

type PostProcessRule struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Builtin  string `yaml:"builtin"`
	Disabled bool   `yaml:"disabled"`
}

type Cleaner interface {
	Clean(input string) (*Result, error)
}

type Loader interface {
	Load(data []byte) (*RuleSet, error)
}
