package numbercleaner

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlLoader struct{}

func NewLoader() Loader {
	return &yamlLoader{}
}

func (l *yamlLoader) Load(data []byte) (*RuleSet, error) {
	rs := &RuleSet{}
	if err := yaml.Unmarshal(data, rs); err != nil {
		return nil, &CleanError{Code: ErrInvalidRuleSet, Message: "decode yaml rule set failed", Cause: err}
	}
	if err := validateRuleSet(rs); err != nil {
		return nil, err
	}
	return rs, nil
}

func NewCleanerFromBytes(data []byte) (Cleaner, error) {
	rs, err := NewLoader().Load(data)
	if err != nil {
		return nil, err
	}
	return NewCleaner(rs)
}

func MergeRuleSets(base *RuleSet, override *RuleSet) (*RuleSet, error) {
	if base == nil && override == nil {
		return nil, &CleanError{Code: ErrInvalidRuleSet, Message: "empty rule sets"}
	}
	if base == nil {
		return cloneRuleSet(override), validateRuleSet(override)
	}
	if override == nil {
		return cloneRuleSet(base), validateRuleSet(base)
	}
	out := cloneRuleSet(base)
	if override.Version != "" {
		out.Version = override.Version
	}
	out.Options = mergeOptions(base.Options, override.Options)
	out.Normalizers = mergeNamedRules(base.Normalizers, override.Normalizers, func(v NormalizerRule) string { return v.Name }, func(v NormalizerRule) bool { return v.Disabled })
	out.RewriteRules = mergeNamedRules(base.RewriteRules, override.RewriteRules, func(v RewriteRule) string { return v.Name }, func(v RewriteRule) bool { return v.Disabled })
	out.SuffixRules = mergeNamedRules(base.SuffixRules, override.SuffixRules, func(v SuffixRule) string { return v.Name }, func(v SuffixRule) bool { return v.Disabled })
	out.NoiseRules = mergeNamedRules(base.NoiseRules, override.NoiseRules, func(v NoiseRule) string { return v.Name }, func(v NoiseRule) bool { return v.Disabled })
	out.Matchers = mergeNamedRules(base.Matchers, override.Matchers, func(v MatcherRule) string { return v.Name }, func(v MatcherRule) bool { return v.Disabled })
	out.PostProcessors = mergeNamedRules(base.PostProcessors, override.PostProcessors, func(v PostProcessRule) string { return v.Name }, func(v PostProcessRule) bool { return v.Disabled })
	if err := validateRuleSet(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateRuleSet(rs *RuleSet) error {
	if rs == nil {
		return &CleanError{Code: ErrInvalidRuleSet, Message: "rule set is nil"}
	}
	if strings.TrimSpace(rs.Version) == "" {
		return &CleanError{Code: ErrInvalidRuleSet, Message: "rule set version is required"}
	}
	allowedBuiltinNormalizer := map[string]struct{}{
		"basename":               {},
		"strip_ext":              {},
		"fullwidth_to_halfwidth": {},
		"trim_space":             {},
		"collapse_spaces":        {},
		"to_upper":               {},
		"replace_pairs":          {},
	}
	allowedBuiltinPostProcessors := map[string]struct{}{
		"reorder_suffix":   {},
		"normalize_hyphen": {},
	}
	allowedSuffixes := map[string]struct{}{
		"C": {}, "4K": {}, "8K": {}, "VR": {}, "LEAK": {}, "U": {}, "UC": {},
	}
	seen := make(map[string]struct{})
	for _, item := range rs.Normalizers {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "normalizer name is required"}
		}
		if item.Type == "builtin" {
			if _, ok := allowedBuiltinNormalizer[item.Builtin]; !ok {
				return &CleanError{Code: ErrInvalidRuleSet, Message: fmt.Sprintf("unsupported normalizer builtin: %s", item.Builtin), Rule: item.Name}
			}
		}
	}
	seen = make(map[string]struct{})
	for _, item := range rs.RewriteRules {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "rewrite rule name is required"}
		}
		if _, ok := seen[item.Name]; ok {
			return &CleanError{Code: ErrInvalidRuleSet, Message: fmt.Sprintf("duplicate rewrite rule name: %s", item.Name), Rule: item.Name}
		}
		seen[item.Name] = struct{}{}
		if strings.TrimSpace(item.Pattern) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "rewrite rule pattern is required", Rule: item.Name}
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "compile rewrite rule regexp failed", Rule: item.Name, Cause: err}
		}
	}
	seen = make(map[string]struct{})
	for _, item := range rs.SuffixRules {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "suffix rule name is required"}
		}
		if item.Type == "regex" {
			if _, err := regexp.Compile(item.Pattern); err != nil {
				return &CleanError{Code: ErrInvalidRuleSet, Message: "compile suffix rule regexp failed", Rule: item.Name, Cause: err}
			}
		}
		if item.Canonical == "" && item.CanonicalTemplate == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "suffix canonical or canonical_template is required", Rule: item.Name}
		}
		if item.Canonical != "" {
			if _, ok := allowedSuffixes[strings.ToUpper(item.Canonical)]; !ok && !strings.HasPrefix(strings.ToUpper(item.Canonical), "CD") {
				return &CleanError{Code: ErrInvalidRuleSet, Message: fmt.Sprintf("unsupported suffix canonical: %s", item.Canonical), Rule: item.Name}
			}
		}
	}
	for _, item := range rs.NoiseRules {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "noise rule name is required"}
		}
		if item.Type == "regex" {
			if _, err := regexp.Compile(item.Pattern); err != nil {
				return &CleanError{Code: ErrInvalidRuleSet, Message: "compile noise rule regexp failed", Rule: item.Name, Cause: err}
			}
		}
	}
	for _, item := range rs.Matchers {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "matcher rule name is required"}
		}
		if _, ok := seen[item.Name]; ok {
			return &CleanError{Code: ErrInvalidRuleSet, Message: fmt.Sprintf("duplicate matcher rule name: %s", item.Name), Rule: item.Name}
		}
		seen[item.Name] = struct{}{}
		if strings.TrimSpace(item.NormalizeTemplate) == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "matcher normalize_template is required", Rule: item.Name}
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return &CleanError{Code: ErrInvalidRuleSet, Message: "compile matcher rule regexp failed", Rule: item.Name, Cause: err}
		}
	}
	for _, item := range rs.PostProcessors {
		if item.Disabled {
			continue
		}
		if item.Type == "builtin" {
			if _, ok := allowedBuiltinPostProcessors[item.Builtin]; !ok {
				return &CleanError{Code: ErrInvalidRuleSet, Message: fmt.Sprintf("unsupported post processor builtin: %s", item.Builtin), Rule: item.Name}
			}
		}
	}
	return nil
}

func mergeOptions(base Options, override Options) Options {
	out := base
	if override.CaseMode != "" {
		out.CaseMode = override.CaseMode
	}
	if override.CollapseSpaces {
		out.CollapseSpaces = true
	}
	if override.EnableEmbeddedMatch {
		out.EnableEmbeddedMatch = true
	}
	if override.FailWhenNoMatch {
		out.FailWhenNoMatch = true
	}
	return out
}

func mergeNamedRules[T any](base []T, override []T, nameFn func(T) string, disabledFn func(T) bool) []T {
	out := make([]T, 0, len(base)+len(override))
	idx := make(map[string]int, len(base)+len(override))
	for _, item := range base {
		name := nameFn(item)
		if name == "" {
			continue
		}
		idx[name] = len(out)
		out = append(out, item)
	}
	for _, item := range override {
		name := nameFn(item)
		if name == "" {
			continue
		}
		if disabledFn(item) {
			if i, ok := idx[name]; ok {
				out = slices.Delete(out, i, i+1)
				idx = rebuildIndex(out, nameFn)
			}
			continue
		}
		if i, ok := idx[name]; ok {
			out[i] = item
			continue
		}
		idx[name] = len(out)
		out = append(out, item)
	}
	return out
}

func rebuildIndex[T any](items []T, nameFn func(T) string) map[string]int {
	idx := make(map[string]int, len(items))
	for i, item := range items {
		idx[nameFn(item)] = i
	}
	return idx
}

func cloneRuleSet(in *RuleSet) *RuleSet {
	if in == nil {
		return nil
	}
	out := *in
	out.Normalizers = slices.Clone(in.Normalizers)
	out.RewriteRules = slices.Clone(in.RewriteRules)
	out.SuffixRules = slices.Clone(in.SuffixRules)
	out.NoiseRules = slices.Clone(in.NoiseRules)
	out.Matchers = slices.Clone(in.Matchers)
	out.PostProcessors = slices.Clone(in.PostProcessors)
	return &out
}
