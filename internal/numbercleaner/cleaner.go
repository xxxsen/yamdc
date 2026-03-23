package numbercleaner

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/xxxsen/yamdc/internal/number"
	"golang.org/x/text/width"
)

type compiledNormalizerRule struct {
	name     string
	typ      string
	builtin  string
	replacer *strings.Replacer
}

type compiledSuffixRule struct {
	name              string
	typ               string
	aliases           []string
	aliasRegex        []*regexp.Regexp
	re                *regexp.Regexp
	canonical         string
	canonicalTemplate string
	priority          int
}

type compiledNoiseRule struct {
	name       string
	typ        string
	aliases    []string
	aliasRegex []*regexp.Regexp
	re         *regexp.Regexp
}

type compiledMatcherRule struct {
	name              string
	category          string
	re                *regexp.Regexp
	normalizeTemplate string
	score             int
	requireBoundary   bool
	prefixes          []string
}

type compiledPostProcessRule struct {
	name    string
	typ     string
	builtin string
}

type compiledRuleSet struct {
	options        Options
	normalizers    []compiledNormalizerRule
	suffixRules    []compiledSuffixRule
	noiseRules     []compiledNoiseRule
	matchers       []compiledMatcherRule
	postProcessors []compiledPostProcessRule
}

type cleaner struct {
	rules *compiledRuleSet
}

type passthroughCleaner struct{}

func NewPassthroughCleaner() Cleaner {
	return &passthroughCleaner{}
}

func (p *passthroughCleaner) Clean(input string) (*Result, error) {
	return &Result{
		RawInput:   input,
		InputNoExt: input,
		Normalized: "",
		NumberID:   "",
		Status:     StatusLowQuality,
		Confidence: ConfidenceLow,
		Warnings:   []string{"number cleaner disabled"},
	}, nil
}

func NewCleaner(rs *RuleSet) (Cleaner, error) {
	if err := validateRuleSet(rs); err != nil {
		return nil, err
	}
	crs, err := compileRuleSet(rs)
	if err != nil {
		return nil, err
	}
	return &cleaner{rules: crs}, nil
}

func compileRuleSet(rs *RuleSet) (*compiledRuleSet, error) {
	out := &compiledRuleSet{options: rs.Options}
	if out.options.CaseMode == "" {
		out.options.CaseMode = "upper"
	}
	for _, item := range rs.Normalizers {
		if item.Disabled {
			continue
		}
		r := compiledNormalizerRule{name: item.Name, typ: item.Type, builtin: item.Builtin}
		if item.Type == "replace" && len(item.Pairs) > 0 {
			kv := make([]string, 0, len(item.Pairs)*2)
			keys := make([]string, 0, len(item.Pairs))
			for k := range item.Pairs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				kv = append(kv, k, item.Pairs[k])
			}
			r.replacer = strings.NewReplacer(kv...)
		}
		out.normalizers = append(out.normalizers, r)
	}
	for _, item := range rs.SuffixRules {
		if item.Disabled {
			continue
		}
		r := compiledSuffixRule{
			name:              item.Name,
			typ:               item.Type,
			aliases:           item.Aliases,
			canonical:         strings.ToUpper(item.Canonical),
			canonicalTemplate: item.CanonicalTemplate,
			priority:          item.Priority,
		}
		if item.Type == "regex" {
			re, err := regexp.Compile(item.Pattern)
			if err != nil {
				return nil, &CleanError{Code: ErrInvalidRuleSet, Message: "compile suffix rule failed", Rule: item.Name, Cause: err}
			}
			r.re = re
		}
		if item.Type == "token" {
			r.aliasRegex = compileAliasRegex(item.Aliases)
		}
		out.suffixRules = append(out.suffixRules, r)
	}
	for _, item := range rs.NoiseRules {
		if item.Disabled {
			continue
		}
		r := compiledNoiseRule{name: item.Name, typ: item.Type, aliases: item.Aliases}
		if item.Type == "regex" {
			re, err := regexp.Compile(item.Pattern)
			if err != nil {
				return nil, &CleanError{Code: ErrInvalidRuleSet, Message: "compile noise rule failed", Rule: item.Name, Cause: err}
			}
			r.re = re
		}
		if item.Type == "token" {
			r.aliasRegex = compileAliasRegex(item.Aliases)
		}
		out.noiseRules = append(out.noiseRules, r)
	}
	for _, item := range rs.Matchers {
		if item.Disabled {
			continue
		}
		re, err := regexp.Compile(item.Pattern)
		if err != nil {
			return nil, &CleanError{Code: ErrInvalidRuleSet, Message: "compile matcher rule failed", Rule: item.Name, Cause: err}
		}
		out.matchers = append(out.matchers, compiledMatcherRule{
			name:              item.Name,
			category:          item.Category,
			re:                re,
			normalizeTemplate: item.NormalizeTemplate,
			score:             item.Score,
			requireBoundary:   item.RequireBoundary,
			prefixes:          item.Prefixes,
		})
	}
	for _, item := range rs.PostProcessors {
		if item.Disabled {
			continue
		}
		out.postProcessors = append(out.postProcessors, compiledPostProcessRule{
			name: item.Name, typ: item.Type, builtin: item.Builtin,
		})
	}
	return out, nil
}

func compileAliasRegex(aliases []string) []*regexp.Regexp {
	rs := make([]*regexp.Regexp, 0, len(aliases))
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		pattern := `(?i)(^|[\s\[\]\(\)_\-])` + regexp.QuoteMeta(trimmed) + `($|[\s\[\]\(\)_\-])`
		rs = append(rs, regexp.MustCompile(pattern))
	}
	return rs
}

func (c *cleaner) Clean(input string) (*Result, error) {
	raw := input
	work := input
	for _, item := range c.rules.normalizers {
		work = applyNormalizer(work, item, c.rules.options)
	}
	inputNoExt := work
	suffixes, work, suffixHits := extractSuffixes(work, c.rules.suffixRules)
	work, noiseHits := removeNoise(work, c.rules.noiseRules)
	candidates := collectCandidates(work, c.rules.matchers)
	if len(candidates) == 0 {
		return &Result{
			RawInput:   raw,
			InputNoExt: inputNoExt,
			Status:     StatusNoMatch,
			Confidence: ConfidenceLow,
			RuleHits:   append(suffixHits, noiseHits...),
			Warnings:   []string{"no candidate matched"},
			Candidates: nil,
		}, nil
	}
	best := candidates[0]
	confidence := confidenceByScore(best.Score)
	normalized := rebuild(best.NumberID, suffixes, c.rules.postProcessors)
	parsed, err := number.Parse(normalized)
	if err != nil {
		return &Result{
			RawInput:   raw,
			InputNoExt: inputNoExt,
			Status:     StatusLowQuality,
			Confidence: ConfidenceLow,
			RuleHits:   append(append(suffixHits, noiseHits...), best.Matcher),
			Warnings:   []string{"normalized output rejected by number.Parse"},
			Candidates: candidates,
		}, nil
	}
	status := StatusSuccess
	warnings := []string{}
	if confidence == ConfidenceLow {
		status = StatusLowQuality
		warnings = append(warnings, "low confidence candidate")
	}
	return &Result{
		RawInput:   raw,
		InputNoExt: inputNoExt,
		Normalized: normalized,
		NumberID:   parsed.GetNumberID(),
		Suffixes:   suffixes,
		Confidence: confidence,
		Status:     status,
		RuleHits:   append(append(suffixHits, noiseHits...), best.Matcher),
		Warnings:   warnings,
		Candidates: candidates,
	}, nil
}

func applyNormalizer(in string, rule compiledNormalizerRule, opts Options) string {
	switch rule.typ {
	case "replace":
		if rule.replacer != nil {
			return rule.replacer.Replace(in)
		}
		return in
	case "builtin":
		switch rule.builtin {
		case "basename":
			return filepath.Base(in)
		case "strip_ext":
			ext := filepath.Ext(in)
			if ext == "" {
				return in
			}
			if !isLikelyExt(ext) {
				return in
			}
			return strings.TrimSuffix(in, ext)
		case "fullwidth_to_halfwidth":
			return width.Narrow.String(in)
		case "trim_space":
			return strings.TrimSpace(in)
		case "collapse_spaces":
			if !opts.CollapseSpaces {
				return in
			}
			return strings.Join(strings.Fields(in), " ")
		case "to_upper":
			if strings.EqualFold(opts.CaseMode, "upper") || opts.CaseMode == "" {
				return strings.ToUpper(in)
			}
			return in
		case "replace_pairs":
			if rule.replacer != nil {
				return rule.replacer.Replace(in)
			}
			return in
		default:
			return in
		}
	default:
		return in
	}
}

func extractSuffixes(in string, rules []compiledSuffixRule) ([]string, string, []string) {
	work := in
	suffixSet := make(map[string]int)
	hits := make([]string, 0, 8)
	for _, rule := range rules {
		switch rule.typ {
		case "regex":
			matches := rule.re.FindAllStringSubmatchIndex(work, -1)
			if len(matches) == 0 {
				continue
			}
			for _, match := range matches {
				val := resolveCanonical(work, rule.canonical, rule.canonicalTemplate, rule.re, match)
				if val == "" {
					continue
				}
				val = strings.ToUpper(strings.TrimSpace(val))
				if _, ok := suffixSet[val]; !ok {
					suffixSet[val] = rule.priority
				}
				hits = append(hits, rule.name)
			}
			work = rule.re.ReplaceAllString(work, " ")
		case "token":
			matched := false
			for _, aliasRe := range rule.aliasRegex {
				if !aliasRe.MatchString(work) {
					continue
				}
				matched = true
				work = aliasRe.ReplaceAllString(work, " ")
			}
			if matched {
				val := strings.ToUpper(strings.TrimSpace(rule.canonical))
				if val != "" {
					if _, ok := suffixSet[val]; !ok {
						suffixSet[val] = rule.priority
					}
				}
				hits = append(hits, rule.name)
			}
		}
	}
	type item struct {
		val      string
		priority int
	}
	items := make([]item, 0, len(suffixSet))
	for k, v := range suffixSet {
		items = append(items, item{val: k, priority: v})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority == items[j].priority {
			return suffixRank(items[i].val) < suffixRank(items[j].val)
		}
		return items[i].priority > items[j].priority
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.val)
	}
	return out, normalizeSpaces(work), hits
}

func removeNoise(in string, rules []compiledNoiseRule) (string, []string) {
	work := in
	hits := make([]string, 0, 8)
	for _, rule := range rules {
		switch rule.typ {
		case "regex":
			if rule.re.MatchString(work) {
				hits = append(hits, rule.name)
				work = rule.re.ReplaceAllString(work, " ")
			}
		case "token":
			matched := false
			for _, aliasRe := range rule.aliasRegex {
				if aliasRe.MatchString(work) {
					matched = true
					work = aliasRe.ReplaceAllString(work, " ")
				}
			}
			if matched {
				hits = append(hits, rule.name)
			}
		}
	}
	return normalizeSpaces(work), hits
}

func collectCandidates(in string, rules []compiledMatcherRule) []Candidate {
	candidates := make([]Candidate, 0, 8)
	for _, rule := range rules {
		matches := rule.re.FindAllStringSubmatchIndex(in, -1)
		for _, match := range matches {
			normalized := string(rule.re.ExpandString(nil, rule.normalizeTemplate, in, match))
			normalized = normalizeSpaces(normalized)
			if normalized == "" {
				continue
			}
			score := rule.score
			start, end := match[0], match[1]
			if start == 0 {
				score += 5
			}
			candidates = append(candidates, Candidate{
				NumberID: normalized,
				Score:    score,
				RuleHits: []string{rule.name},
				Matcher:  rule.name,
				Start:    start,
				End:      end,
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].Start != candidates[j].Start {
			return candidates[i].Start < candidates[j].Start
		}
		if len(candidates[i].NumberID) != len(candidates[j].NumberID) {
			return len(candidates[i].NumberID) > len(candidates[j].NumberID)
		}
		return candidates[i].Matcher < candidates[j].Matcher
	})
	return dedupeCandidates(candidates)
}

func dedupeCandidates(items []Candidate) []Candidate {
	seen := make(map[string]struct{}, len(items))
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.NumberID]; ok {
			continue
		}
		seen[item.NumberID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func rebuild(numberID string, suffixes []string, post []compiledPostProcessRule) string {
	normalized := numberID
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		if normalized != "" {
			normalized += "-" + suffix
		} else {
			normalized = suffix
		}
	}
	for _, item := range post {
		switch item.builtin {
		case "normalize_hyphen":
			normalized = strings.ReplaceAll(normalized, " ", "-")
			normalized = strings.ReplaceAll(normalized, "_", "_")
			normalized = regexp.MustCompile(`-+`).ReplaceAllString(normalized, "-")
			normalized = strings.Trim(normalized, "- ")
		case "reorder_suffix":
			// suffixes are already ordered during extraction
		}
	}
	return normalized
}

func resolveCanonical(src string, canonical string, tmpl string, re *regexp.Regexp, match []int) string {
	if tmpl != "" {
		return string(re.ExpandString(nil, tmpl, src, match))
	}
	return canonical
}

func normalizeSpaces(in string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(in), " "))
}

func isLikelyExt(ext string) bool {
	if len(ext) < 2 || len(ext) > 8 {
		return false
	}
	for _, ch := range ext[1:] {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		return false
	}
	return true
}

func confidenceByScore(score int) Confidence {
	switch {
	case score >= 90:
		return ConfidenceHigh
	case score >= 70:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func suffixRank(v string) int {
	switch {
	case v == "C":
		return 1
	case v == "UC" || v == "U":
		return 2
	case v == "LEAK":
		return 3
	case v == "VR":
		return 4
	case v == "4K":
		return 5
	case v == "8K":
		return 6
	case strings.HasPrefix(v, "CD"):
		return 7
	default:
		return 99
	}
}
