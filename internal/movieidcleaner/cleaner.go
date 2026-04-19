package movieidcleaner

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/width"

	"github.com/xxxsen/yamdc/internal/number"
)

type compiledNormalizerRule struct {
	name     string
	typ      string
	builtin  string
	replacer *strings.Replacer
}

type compiledRewriteRule struct {
	name    string
	re      *regexp.Regexp
	replace string
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
	uncensorValue     bool
	uncensorSet       bool
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
	rewriteRules   []compiledRewriteRule
	suffixRules    []compiledSuffixRule
	noiseRules     []compiledNoiseRule
	matchers       []compiledMatcherRule
	postProcessors []compiledPostProcessRule
}

type cleaner struct {
	rules *compiledRuleSet
}

type passthroughCleaner struct{}

type explainCollector struct {
	steps []ExplainStep
}

func (c *explainCollector) add(stage, rule, input, output string, matched bool, summary string) {
	if c == nil {
		return
	}
	c.steps = append(c.steps, ExplainStep{
		Stage:   stage,
		Rule:    rule,
		Input:   input,
		Output:  output,
		Matched: matched,
		Summary: summary,
	})
}

func (c *explainCollector) addWithValues(
	stage,
	rule,
	input,
	output string,
	matched bool,
	summary string,
	values []string,
) {
	if c == nil {
		return
	}
	step := ExplainStep{
		Stage:   stage,
		Rule:    rule,
		Input:   input,
		Output:  output,
		Matched: matched,
		Summary: summary,
	}
	if len(values) != 0 {
		step.Values = append([]string(nil), values...)
	}
	c.steps = append(c.steps, step)
}

func (c *explainCollector) addCandidate(
	stage,
	rule,
	input,
	output string,
	matched bool,
	summary string,
	candidate Candidate,
	selected bool,
) {
	if c == nil {
		return
	}
	cp := candidate
	c.steps = append(c.steps, ExplainStep{
		Stage:     stage,
		Rule:      rule,
		Input:     input,
		Output:    output,
		Matched:   matched,
		Selected:  selected,
		Summary:   summary,
		Candidate: &cp,
	})
}

func NewPassthroughCleaner() Cleaner {
	return &passthroughCleaner{}
}

func (p *passthroughCleaner) Clean(input string) (*Result, error) {
	return &Result{
		RawInput:        input,
		InputNoExt:      input,
		Normalized:      "",
		NumberID:        "",
		Status:          StatusLowQuality,
		Confidence:      ConfidenceLow,
		Warnings:        []string{"movieid cleaner disabled"},
		CategoryMatched: false,
		UncensorMatched: false,
	}, nil
}

func (p *passthroughCleaner) Explain(input string) (*ExplainResult, error) {
	final, err := p.Clean(input)
	if err != nil {
		return nil, err
	}
	return &ExplainResult{
		Input:      input,
		InputNoExt: final.InputNoExt,
		Final:      final,
		Steps: []ExplainStep{
			{
				Stage:   "result",
				Rule:    "passthrough",
				Input:   input,
				Output:  final.Normalized,
				Matched: false,
				Summary: "movieid cleaner disabled",
			},
		},
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

func compileNormalizers(items []NormalizerRule) []compiledNormalizerRule {
	out := make([]compiledNormalizerRule, 0, len(items))
	for _, item := range items {
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
		out = append(out, r)
	}
	return out
}

func compileSuffixRules(items []SuffixRule) ([]compiledSuffixRule, error) {
	out := make([]compiledSuffixRule, 0, len(items))
	for _, item := range items {
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
				return nil, &CleanError{
					Code: ErrInvalidRuleSet, Message: "compile suffix rule failed",
					Rule: item.Name, Cause: err,
				}
			}
			r.re = re
		}
		if item.Type == "token" {
			r.aliasRegex = compileAliasRegex(item.Aliases)
		}
		out = append(out, r)
	}
	return out, nil
}

func compileNoiseRules(items []NoiseRule) ([]compiledNoiseRule, error) {
	out := make([]compiledNoiseRule, 0, len(items))
	for _, item := range items {
		if item.Disabled {
			continue
		}
		r := compiledNoiseRule{name: item.Name, typ: item.Type, aliases: item.Aliases}
		if item.Type == "regex" {
			re, err := regexp.Compile(item.Pattern)
			if err != nil {
				return nil, &CleanError{
					Code: ErrInvalidRuleSet, Message: "compile noise rule failed",
					Rule: item.Name, Cause: err,
				}
			}
			r.re = re
		}
		if item.Type == "token" {
			r.aliasRegex = compileAliasRegex(item.Aliases)
		}
		out = append(out, r)
	}
	return out, nil
}

func compileMatcherRules(items []MatcherRule) ([]compiledMatcherRule, error) {
	out := make([]compiledMatcherRule, 0, len(items))
	for _, item := range items {
		if item.Disabled {
			continue
		}
		re, err := regexp.Compile(item.Pattern)
		if err != nil {
			return nil, &CleanError{
				Code: ErrInvalidRuleSet, Message: "compile matcher rule failed",
				Rule: item.Name, Cause: err,
			}
		}
		m := compiledMatcherRule{
			name:              item.Name,
			category:          item.Category,
			uncensorSet:       item.Uncensor != nil,
			re:                re,
			normalizeTemplate: item.NormalizeTemplate,
			score:             item.Score,
			requireBoundary:   item.RequireBoundary,
			prefixes:          item.Prefixes,
		}
		if item.Uncensor != nil {
			m.uncensorValue = *item.Uncensor
		}
		out = append(out, m)
	}
	return out, nil
}

func compileRuleSet(rs *RuleSet) (*compiledRuleSet, error) {
	out := &compiledRuleSet{options: rs.Options}
	if out.options.CaseMode == "" {
		out.options.CaseMode = "upper"
	}
	out.normalizers = compileNormalizers(rs.Normalizers)
	var err error
	for _, item := range rs.RewriteRules {
		if item.Disabled {
			continue
		}
		re, err := regexp.Compile(item.Pattern)
		if err != nil {
			return nil, &CleanError{
				Code: ErrInvalidRuleSet, Message: "compile rewrite rule failed",
				Rule: item.Name, Cause: err,
			}
		}
		out.rewriteRules = append(out.rewriteRules, compiledRewriteRule{
			name: item.Name, re: re, replace: item.Replace,
		})
	}
	if out.suffixRules, err = compileSuffixRules(rs.SuffixRules); err != nil {
		return nil, err
	}
	if out.noiseRules, err = compileNoiseRules(rs.NoiseRules); err != nil {
		return nil, err
	}
	if out.matchers, err = compileMatcherRules(rs.Matchers); err != nil {
		return nil, err
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
	result, _ := c.run(input, false)
	return result, nil
}

func (c *cleaner) Explain(input string) (*ExplainResult, error) {
	result, collector := c.run(input, true)
	return &ExplainResult{
		Input:      input,
		InputNoExt: result.InputNoExt,
		Steps:      collector.steps,
		Final:      result,
	}, nil
}

type runState struct {
	raw         string
	inputNoExt  string
	suffixes    []string
	rewriteHits []string
	suffixHits  []string
	noiseHits   []string
}

func (rs *runState) allHits() []string {
	return append(append(rs.rewriteHits, rs.suffixHits...), rs.noiseHits...)
}

func buildNoMatchResult(state *runState, work string, collector *explainCollector) *Result {
	if collector != nil {
		collector.add("result", "selected_candidate", work, "", false, "no candidate matched")
	}
	return &Result{
		RawInput:   state.raw,
		InputNoExt: state.inputNoExt,
		Status:     StatusNoMatch,
		Confidence: ConfidenceLow,
		RuleHits:   state.allHits(),
		Warnings:   []string{"no candidate matched"},
	}
}

func buildMatchResult(
	state *runState,
	best Candidate,
	candidates []Candidate,
	normalized string,
	collector *explainCollector,
) *Result {
	parsed, err := number.Parse(normalized)
	if err != nil {
		if collector != nil {
			collector.add("result", "number_parse", normalized, "", false, "normalized output rejected by number.Parse")
		}
		return &Result{
			RawInput:        state.raw,
			InputNoExt:      state.inputNoExt,
			Status:          StatusLowQuality,
			Confidence:      ConfidenceLow,
			RuleHits:        append(state.allHits(), best.RuleHits...),
			Warnings:        []string{"normalized output rejected by number.Parse"},
			Candidates:      candidates,
			Category:        best.Category,
			CategoryMatched: best.CategoryMatched,
			Uncensor:        best.Uncensor,
			UncensorMatched: best.UncensorMatched,
		}
	}
	confidence := confidenceByScore(best.Score)
	status := StatusSuccess
	warnings := []string{}
	if confidence == ConfidenceLow {
		status = StatusLowQuality
		warnings = append(warnings, "low confidence candidate")
	}
	if collector != nil {
		collector.add("result", "number_parse", normalized, parsed.GetNumberID(), true,
			"normalized output accepted by number.Parse")
	}
	return &Result{
		RawInput:        state.raw,
		InputNoExt:      state.inputNoExt,
		Normalized:      normalized,
		NumberID:        parsed.GetNumberID(),
		Suffixes:        state.suffixes,
		Category:        best.Category,
		Uncensor:        best.Uncensor,
		CategoryMatched: best.CategoryMatched,
		UncensorMatched: best.UncensorMatched,
		Confidence:      confidence,
		Status:          status,
		RuleHits:        append(state.allHits(), best.RuleHits...),
		Warnings:        warnings,
		Candidates:      candidates,
	}
}

func (c *cleaner) run(input string, withExplain bool) (*Result, *explainCollector) {
	work := input
	var collector *explainCollector
	if withExplain {
		collector = &explainCollector{}
	}
	for _, item := range c.rules.normalizers {
		before := work
		work = applyNormalizer(work, item, c.rules.options)
		if collector != nil {
			collector.add("normalizers", item.name, before, work, before != work, "")
		}
	}
	state := &runState{raw: input, inputNoExt: work}
	work, state.rewriteHits = applyRewriteRulesWithExplain(work, c.rules.rewriteRules, collector)
	state.suffixes, work, state.suffixHits = extractSuffixesWithExplain(work, c.rules.suffixRules, collector)
	work, state.noiseHits = removeNoiseWithExplain(work, c.rules.noiseRules, collector)
	candidates := collectCandidatesWithExplain(work, c.rules.matchers, collector)
	if len(candidates) == 0 {
		return buildNoMatchResult(state, work, collector), collector
	}
	best := candidates[0]
	if collector != nil {
		collector.addCandidate("result", "selected_candidate", work, best.NumberID, true,
			"selected best candidate", best, true)
	}
	normalized := rebuildWithExplain(best.NumberID, state.suffixes, c.rules.postProcessors, collector)
	result := buildMatchResult(state, best, candidates, normalized, collector)
	return result, collector
}

// applyNormalizer 针对 rule.typ 做 dispatch, 每个 case 是一种 normalize 策略,
// 拆分成多函数只会把同一张策略表打散在多个文件, 反而难看清支持哪些 typ.
//
//nolint:gocyclo // dispatch switch over normalizer types
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

func applyRewriteRulesWithExplain(
	in string,
	rules []compiledRewriteRule,
	collector *explainCollector,
) (string, []string) {
	work := in
	hits := make([]string, 0, len(rules))
	for _, rule := range rules {
		before := work
		if !rule.re.MatchString(work) {
			if collector != nil {
				collector.add("rewrite_rules", rule.name, before, before, false, "pattern not matched")
			}
			continue
		}
		next := rule.re.ReplaceAllString(work, rule.replace)
		if collector != nil {
			summary := "rewrite applied"
			matched := next != before
			if !matched {
				summary = "pattern matched but output unchanged"
			}
			collector.add("rewrite_rules", rule.name, before, next, matched, summary)
		}
		if next == work {
			continue
		}
		work = next
		hits = append(hits, rule.name)
	}
	return normalizeSpaces(work), hits
}

func removeNoiseRegex(work string, rule compiledNoiseRule, collector *explainCollector) (string, bool) {
	before := work
	if !rule.re.MatchString(work) {
		if collector != nil {
			collector.add("noise_rules", rule.name, before, before, false, "pattern not matched")
		}
		return work, false
	}
	work = rule.re.ReplaceAllString(work, " ")
	if collector != nil {
		collector.add("noise_rules", rule.name, before, normalizeSpaces(work), true, "noise removed")
	}
	return work, true
}

func removeNoiseToken(work string, rule compiledNoiseRule, collector *explainCollector) (string, bool) {
	before := work
	matched := false
	for _, aliasRe := range rule.aliasRegex {
		if aliasRe.MatchString(work) {
			matched = true
			work = aliasRe.ReplaceAllString(work, " ")
		}
	}
	if collector != nil {
		summary := "token not matched"
		if matched {
			summary = "noise removed"
		}
		collector.add("noise_rules", rule.name, before, normalizeSpaces(work), matched, summary)
	}
	return work, matched
}

func removeNoiseWithExplain(in string, rules []compiledNoiseRule, collector *explainCollector) (string, []string) {
	work := in
	hits := make([]string, 0, 8)
	for _, rule := range rules {
		var matched bool
		switch rule.typ {
		case "regex":
			work, matched = removeNoiseRegex(work, rule, collector)
		case "token":
			work, matched = removeNoiseToken(work, rule, collector)
		default:
			if collector != nil {
				collector.add("noise_rules", rule.name, work, work, false, "unsupported noise rule type")
			}
			continue
		}
		if matched {
			hits = append(hits, rule.name)
		}
	}
	return normalizeSpaces(work), hits
}

func collectCandidatesWithExplain(in string, rules []compiledMatcherRule, collector *explainCollector) []Candidate {
	candidates := make([]Candidate, 0, 8)
	for _, rule := range rules {
		matches := rule.re.FindAllStringSubmatchIndex(in, -1)
		if len(matches) == 0 {
			if collector != nil {
				collector.add("matchers", rule.name, in, "", false, "pattern not matched")
			}
			continue
		}
		for _, match := range matches {
			normalized := string(rule.re.ExpandString(nil, rule.normalizeTemplate, in, match))
			normalized = normalizeSpaces(normalized)
			if normalized == "" {
				if collector != nil {
					collector.add("matchers", rule.name, in, "", false, "matched but normalized candidate is empty")
				}
				continue
			}
			score := rule.score
			start, end := match[0], match[1]
			if start == 0 {
				score += 5
			}
			candidate := Candidate{
				NumberID:        normalized,
				Score:           score,
				RuleHits:        []string{rule.name},
				Matcher:         rule.name,
				Start:           start,
				End:             end,
				Category:        strings.TrimSpace(rule.category),
				CategoryMatched: strings.TrimSpace(rule.category) != "",
				Uncensor:        rule.uncensorValue,
				UncensorMatched: rule.uncensorSet,
			}
			candidates = append(candidates, candidate)
			if collector != nil {
				collector.addCandidate("matchers", rule.name, in, normalized, true, "candidate matched", candidate, false)
			}
		}
	}
	return dedupeCandidates(candidates)
}

func dedupeCandidates(items []Candidate) []Candidate {
	seen := make(map[string]struct{}, len(items))
	idx := make(map[string]int, len(items))
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.NumberID]; ok {
			i := idx[item.NumberID]
			if !out[i].CategoryMatched && item.CategoryMatched {
				out[i].Category = item.Category
				out[i].CategoryMatched = true
			}
			if !out[i].UncensorMatched && item.UncensorMatched {
				out[i].Uncensor = item.Uncensor
				out[i].UncensorMatched = true
			}
			out[i].RuleHits = append(out[i].RuleHits, item.RuleHits...)
			continue
		}
		seen[item.NumberID] = struct{}{}
		idx[item.NumberID] = len(out)
		out = append(out, item)
	}
	return out
}

func rebuildWithExplain(
	numberID string,
	suffixes []string,
	post []compiledPostProcessRule,
	collector *explainCollector,
) string {
	normalized := numberID
	if collector != nil {
		collector.addWithValues(
			"post_processors",
			"attach_suffixes",
			numberID,
			numberID,
			len(suffixes) != 0,
			"prepare normalized output",
			suffixes,
		)
	}
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		before := normalized
		if normalized != "" {
			normalized += "-" + suffix
		} else {
			normalized = suffix
		}
		if collector != nil {
			collector.addWithValues(
				"post_processors",
				"attach_suffixes",
				before,
				normalized,
				true,
				"suffix appended",
				[]string{suffix},
			)
		}
	}
	for _, item := range post {
		before := normalized
		switch item.builtin {
		case "normalize_hyphen":
			normalized = strings.ReplaceAll(normalized, " ", "-")
			normalized = strings.ReplaceAll(normalized, "_", "-")
			normalized = regexp.MustCompile(`-+`).ReplaceAllString(normalized, "-")
			normalized = strings.Trim(normalized, "- ")
		case "reorder_suffix":
			// suffixes are already ordered during extraction
		}
		if collector != nil {
			summary := "post processor applied"
			matched := before != normalized
			if !matched {
				summary = "post processor skipped"
			}
			collector.add("post_processors", item.name, before, normalized, matched, summary)
		}
	}
	return normalized
}

func extractSuffixRegex(
	work string,
	rule compiledSuffixRule,
	suffixSet map[string]int,
) (string, []string, []string) {
	collected := make([]string, 0, 2)
	var hits []string
	matches := rule.re.FindAllStringSubmatchIndex(work, -1)
	if len(matches) == 0 {
		return work, collected, nil
	}
	for _, match := range matches {
		val := resolveCanonical(work, rule.canonical, rule.canonicalTemplate, rule.re, match)
		val = strings.ToUpper(strings.TrimSpace(val))
		if val == "" {
			continue
		}
		collected = append(collected, val)
		if _, ok := suffixSet[val]; !ok {
			suffixSet[val] = rule.priority
		}
		hits = append(hits, rule.name)
	}
	return rule.re.ReplaceAllString(work, " "), collected, hits
}

func extractSuffixToken(
	work string,
	rule compiledSuffixRule,
	suffixSet map[string]int,
) (string, []string, []string) {
	collected := make([]string, 0, 2)
	matched := false
	for _, aliasRe := range rule.aliasRegex {
		if aliasRe.MatchString(work) {
			matched = true
			work = aliasRe.ReplaceAllString(work, " ")
		}
	}
	if !matched {
		return work, collected, nil
	}
	val := strings.ToUpper(strings.TrimSpace(rule.canonical))
	if val != "" {
		collected = append(collected, val)
		if _, ok := suffixSet[val]; !ok {
			suffixSet[val] = rule.priority
		}
	}
	return work, collected, []string{rule.name}
}

func sortSuffixSet(suffixSet map[string]int) []string {
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
	for _, it := range items {
		out = append(out, it.val)
	}
	return out
}

func extractSuffixesWithExplain(
	in string, rules []compiledSuffixRule, collector *explainCollector,
) ([]string, string, []string) {
	work := in
	suffixSet := make(map[string]int)
	hits := make([]string, 0, 8)
	for _, rule := range rules {
		before := work
		var collected []string
		var ruleHits []string
		switch rule.typ {
		case "regex":
			work, collected, ruleHits = extractSuffixRegex(work, rule, suffixSet)
		case "token":
			work, collected, ruleHits = extractSuffixToken(work, rule, suffixSet)
		default:
			if collector != nil {
				collector.add("suffix_rules", rule.name, before, before, false, "suffix not matched")
			}
			continue
		}
		hits = append(hits, ruleHits...)
		if collector != nil {
			matched := len(collected) != 0
			summary := "suffix not matched"
			if matched {
				summary = "suffix extracted"
			}
			collector.addWithValues("suffix_rules", rule.name, before, normalizeSpaces(work), matched, summary, collected)
		}
	}
	return sortSuffixSet(suffixSet), normalizeSpaces(work), hits
}

func resolveCanonical(src, canonical, tmpl string, re *regexp.Regexp, match []int) string {
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
