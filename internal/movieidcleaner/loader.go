package movieidcleaner

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlLoader struct{}

var (
	allowedBuiltinNormalizers = map[string]struct{}{
		"basename":               {},
		"strip_ext":              {},
		"fullwidth_to_halfwidth": {},
		"trim_space":             {},
		"collapse_spaces":        {},
		"to_upper":               {},
		"replace_pairs":          {},
	}
	allowedBuiltinPostProcessors = map[string]struct{}{
		"reorder_suffix":   {},
		"normalize_hyphen": {},
	}
	allowedSuffixes = map[string]struct{}{
		"C": {}, "4K": {}, "8K": {}, "VR": {}, "LEAK": {}, "U": {}, "UC": {},
	}
)

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
		return nil, fmt.Errorf("load rule set from bytes failed: %w", err)
	}
	return NewCleaner(rs)
}

func LoadRuleSetFromPath(path string) (*RuleSet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat ruleset path %s failed: %w", path, err)
	}
	if info.IsDir() {
		fsys := os.DirFS(path)
		if manifest, ok, err := readManifest(fsys, "."); err != nil {
			return nil, err
		} else if ok {
			entry, err := cleanBundleEntry(manifest.Entry)
			if err != nil {
				return nil, err
			}
			return LoadRuleSetFromFS(fsys, entry)
		}
		return LoadRuleSetFromFS(fsys, ".")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ruleset file %s failed: %w", path, err)
	}
	rs, err := NewLoader().Load(data)
	if err != nil {
		return nil, fmt.Errorf("load ruleset from file %s failed: %w", path, err)
	}
	return rs, nil
}

func LoadRuleSetFromDir(dir string) (*RuleSet, error) {
	return LoadRuleSetFromFS(os.DirFS(dir), ".")
}

func ListRuleSetFilesFromDir(dir string) ([]string, error) {
	return ListRuleSetFilesFromFS(os.DirFS(dir), ".")
}

func MergeRuleSets(base, override *RuleSet) (*RuleSet, error) {
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
	out.Normalizers = mergeNamedRules(
		base.Normalizers, override.Normalizers,
		func(v NormalizerRule) string { return v.Name },
		func(v NormalizerRule) bool { return v.Disabled },
	)
	out.RewriteRules = mergeNamedRules(
		base.RewriteRules, override.RewriteRules,
		func(v RewriteRule) string { return v.Name },
		func(v RewriteRule) bool { return v.Disabled },
	)
	out.SuffixRules = mergeNamedRules(
		base.SuffixRules, override.SuffixRules,
		func(v SuffixRule) string { return v.Name },
		func(v SuffixRule) bool { return v.Disabled },
	)
	out.NoiseRules = mergeNamedRules(
		base.NoiseRules, override.NoiseRules,
		func(v NoiseRule) string { return v.Name },
		func(v NoiseRule) bool { return v.Disabled },
	)
	out.Matchers = mergeNamedRules(
		base.Matchers, override.Matchers,
		func(v MatcherRule) string { return v.Name },
		func(v MatcherRule) bool { return v.Disabled },
	)
	out.PostProcessors = mergeNamedRules(
		base.PostProcessors, override.PostProcessors,
		func(v PostProcessRule) string { return v.Name },
		func(v PostProcessRule) bool { return v.Disabled },
	)
	if err := validateRuleSet(out); err != nil {
		return nil, err
	}
	return out, nil
}

func LoadRuleSetFromFS(fsys fs.FS, dir string) (*RuleSet, error) {
	files, err := collectRuleSetFilesFromFS(fsys, dir)
	if err != nil {
		return nil, err
	}
	var merged *RuleSet
	for _, file := range files {
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("read rule file %s failed: %w", file, err)
		}
		part, err := NewLoader().Load(data)
		if err != nil {
			return nil, &CleanError{
				Code: ErrInvalidRuleSet, Message: fmt.Sprintf("load rule fragment failed: %s", file), Cause: err,
			}
		}
		if merged == nil {
			merged = cloneRuleSet(part)
			continue
		}
		merged, err = mergeRuleSetFragments(merged, part)
		if err != nil {
			return nil, &CleanError{
				Code: ErrInvalidRuleSet, Message: fmt.Sprintf("merge rule fragment failed: %s", file), Cause: err,
			}
		}
	}
	if err := validateRuleSet(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func ListRuleSetFilesFromFS(fsys fs.FS, dir string) ([]string, error) {
	files, err := collectRuleSetFilesFromFS(fsys, dir)
	if err != nil {
		return nil, err
	}
	return slices.Clone(files), nil
}

func collectRuleSetFilesFromFS(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read ruleset dir %s failed: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			continue
		}
		files = append(files, path.Join(dir, name))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, &CleanError{
			Code: ErrInvalidRuleSet, Message: fmt.Sprintf("no yaml files found in dir: %s", dir),
		}
	}
	return files, nil
}

func validateFragmentVersions(base, part *RuleSet) error {
	if strings.TrimSpace(part.Version) == "" {
		return &CleanError{Code: ErrInvalidRuleSet, Message: "rule fragment version is required"}
	}
	if base.Version != part.Version {
		return &CleanError{
			Code:    ErrInvalidRuleSet,
			Message: fmt.Sprintf("rule fragment version mismatch: %s != %s", base.Version, part.Version),
		}
	}
	return nil
}

func appendAllFragmentRules(out, part *RuleSet) error {
	var err error
	out.Normalizers, err = appendUniqueNamedRules(
		out.Normalizers, part.Normalizers, func(v NormalizerRule) string { return v.Name })
	if err != nil {
		return err
	}
	out.RewriteRules, err = appendUniqueNamedRules(
		out.RewriteRules, part.RewriteRules, func(v RewriteRule) string { return v.Name })
	if err != nil {
		return err
	}
	out.SuffixRules, err = appendUniqueNamedRules(
		out.SuffixRules, part.SuffixRules, func(v SuffixRule) string { return v.Name })
	if err != nil {
		return err
	}
	out.NoiseRules, err = appendUniqueNamedRules(
		out.NoiseRules, part.NoiseRules, func(v NoiseRule) string { return v.Name })
	if err != nil {
		return err
	}
	out.Matchers, err = appendUniqueNamedRules(
		out.Matchers, part.Matchers, func(v MatcherRule) string { return v.Name })
	if err != nil {
		return err
	}
	out.PostProcessors, err = appendUniqueNamedRules(
		out.PostProcessors, part.PostProcessors, func(v PostProcessRule) string { return v.Name })
	return err
}

func mergeRuleSetFragments(base, part *RuleSet) (*RuleSet, error) {
	if base == nil {
		return cloneRuleSet(part), nil
	}
	if part == nil {
		return cloneRuleSet(base), nil
	}
	if strings.TrimSpace(base.Version) == "" {
		base.Version = part.Version
	}
	if err := validateFragmentVersions(base, part); err != nil {
		return nil, err
	}
	out := cloneRuleSet(base)
	if !isZeroOptions(part.Options) {
		if isZeroOptions(out.Options) {
			out.Options = part.Options
		} else if !reflect.DeepEqual(out.Options, part.Options) {
			return nil, &CleanError{Code: ErrInvalidRuleSet, Message: "options conflict across rule fragments"}
		}
	}
	if err := appendAllFragmentRules(out, part); err != nil {
		return nil, err
	}
	return out, nil
}

func appendUniqueNamedRules[T any](base, extra []T, nameFn func(T) string) ([]T, error) {
	out := slices.Clone(base)
	seen := make(map[string]struct{}, len(base)+len(extra))
	for _, item := range base {
		name := strings.TrimSpace(nameFn(item))
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	for _, item := range extra {
		name := strings.TrimSpace(nameFn(item))
		if name == "" {
			out = append(out, item)
			continue
		}
		if _, ok := seen[name]; ok {
			return nil, &CleanError{
				Code: ErrInvalidRuleSet, Message: fmt.Sprintf("duplicate rule name across fragments: %s", name), Rule: name,
			}
		}
		seen[name] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func isZeroOptions(opts Options) bool {
	return reflect.DeepEqual(opts, Options{})
}

func validateRuleSet(rs *RuleSet) error {
	if rs == nil {
		return &CleanError{Code: ErrInvalidRuleSet, Message: "rule set is nil"}
	}
	if strings.TrimSpace(rs.Version) == "" {
		return &CleanError{Code: ErrInvalidRuleSet, Message: "rule set version is required"}
	}
	if err := validateNormalizers(rs.Normalizers); err != nil {
		return err
	}
	if err := validateRewriteRules(rs.RewriteRules); err != nil {
		return err
	}
	if err := validateSuffixRules(rs.SuffixRules); err != nil {
		return err
	}
	if err := validateNoiseRules(rs.NoiseRules); err != nil {
		return err
	}
	if err := validateMatchers(rs.Matchers); err != nil {
		return err
	}
	return validatePostProcessors(rs.PostProcessors)
}

func validateNormalizers(items []NormalizerRule) error {
	if err := validateUniqueRuleNames(items, "normalizer"); err != nil {
		return err
	}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if item.Type == "builtin" {
			if _, ok := allowedBuiltinNormalizers[item.Builtin]; !ok {
				return &CleanError{
					Code: ErrInvalidRuleSet, Message: fmt.Sprintf("unsupported normalizer builtin: %s", item.Builtin), Rule: item.Name,
				}
			}
		}
	}
	return nil
}

func validateRewriteRules(items []RewriteRule) error {
	if err := validateUniqueRuleNames(items, "rewrite rule"); err != nil {
		return err
	}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.Pattern) == "" {
			return &CleanError{
				Code: ErrInvalidRuleSet, Message: "rewrite rule pattern is required", Rule: item.Name,
			}
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return &CleanError{
				Code: ErrInvalidRuleSet, Message: "compile rewrite rule regexp failed", Rule: item.Name, Cause: err,
			}
		}
	}
	return nil
}

func validateSuffixRules(items []SuffixRule) error {
	if err := validateUniqueRuleNames(items, "suffix rule"); err != nil {
		return err
	}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if item.Type == "regex" {
			if _, err := regexp.Compile(item.Pattern); err != nil {
				return &CleanError{
					Code: ErrInvalidRuleSet, Message: "compile suffix rule regexp failed", Rule: item.Name, Cause: err,
				}
			}
		}
		if item.Canonical == "" && item.CanonicalTemplate == "" {
			return &CleanError{
				Code: ErrInvalidRuleSet, Message: "suffix canonical or canonical_template is required", Rule: item.Name,
			}
		}
		if item.Canonical != "" {
			upper := strings.ToUpper(item.Canonical)
			if _, ok := allowedSuffixes[upper]; !ok && !strings.HasPrefix(upper, "CD") {
				return &CleanError{
					Code: ErrInvalidRuleSet, Message: fmt.Sprintf("unsupported suffix canonical: %s", item.Canonical), Rule: item.Name,
				}
			}
		}
	}
	return nil
}

func validateNoiseRules(items []NoiseRule) error {
	if err := validateUniqueRuleNames(items, "noise rule"); err != nil {
		return err
	}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if item.Type == "regex" {
			if _, err := regexp.Compile(item.Pattern); err != nil {
				return &CleanError{
					Code: ErrInvalidRuleSet, Message: "compile noise rule regexp failed", Rule: item.Name, Cause: err,
				}
			}
		}
	}
	return nil
}

func validateMatchers(items []MatcherRule) error {
	if err := validateUniqueRuleNames(items, "matcher rule"); err != nil {
		return err
	}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.NormalizeTemplate) == "" {
			return &CleanError{
				Code: ErrInvalidRuleSet, Message: "matcher normalize_template is required", Rule: item.Name,
			}
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return &CleanError{
				Code: ErrInvalidRuleSet, Message: "compile matcher rule regexp failed", Rule: item.Name, Cause: err,
			}
		}
	}
	return nil
}

func validatePostProcessors(items []PostProcessRule) error {
	if err := validateUniqueRuleNames(items, "post processor"); err != nil {
		return err
	}
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if item.Type == "builtin" {
			if _, ok := allowedBuiltinPostProcessors[item.Builtin]; !ok {
				return &CleanError{
					Code:    ErrInvalidRuleSet,
					Message: fmt.Sprintf("unsupported post processor builtin: %s", item.Builtin),
					Rule:    item.Name,
				}
			}
		}
	}
	return nil
}

type namedRule interface {
	GetName() string
	IsDisabled() bool
}

func validateUniqueRuleNames[T namedRule](items []T, kind string) error {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.IsDisabled() {
			continue
		}
		name := strings.TrimSpace(item.GetName())
		if name == "" {
			return &CleanError{Code: ErrInvalidRuleSet, Message: fmt.Sprintf("%s name is required", kind)}
		}
		if _, ok := seen[name]; ok {
			return &CleanError{
				Code: ErrInvalidRuleSet, Message: fmt.Sprintf("duplicate %s name: %s", kind, name), Rule: name,
			}
		}
		seen[name] = struct{}{}
	}
	return nil
}

func mergeOptions(base, override Options) Options {
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

func mergeNamedRules[T any](base, override []T, nameFn func(T) string, disabledFn func(T) bool) []T {
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
