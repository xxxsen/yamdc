package movieidcleaner

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestRuleSet(t *testing.T) *RuleSet {
	t.Helper()
	rs, err := LoadRuleSetFromPath(filepath.Join("testdata", "default-bundle"))
	require.NoError(t, err)
	return rs
}

func TestCleanerClean(t *testing.T) {
	cl, err := NewCleaner(loadTestRuleSet(t))
	require.NoError(t, err)

	cases := map[string]struct {
		input           string
		normalized      string
		numberID        string
		status          Status
		category        string
		categoryMatched bool
		uncensor        bool
		uncensorMatched bool
		suffixes        []string
	}{
		"rawx": {
			input:           "[VID] rawxppv12345 sub.mp4",
			normalized:      "RAWX-PPV-12345-C",
			numberID:        "RAWX-PPV-12345",
			status:          StatusSuccess,
			category:        "RAWX",
			categoryMatched: true,
			uncensor:        true,
			uncensorMatched: true,
			suffixes:        []string{"C"},
		},
		"generic": {
			input:           "abc123 disc2.avi",
			normalized:      "ABC-123-CD2",
			numberID:        "ABC-123",
			status:          StatusSuccess,
			category:        "",
			categoryMatched: false,
			uncensor:        false,
			uncensorMatched: false,
			suffixes:        []string{"CD2"},
		},
		"open": {
			input:           "www.example.com OPEN-1234 leak.mp4",
			normalized:      "OPEN-1234-LEAK",
			numberID:        "OPEN-1234",
			status:          StatusSuccess,
			category:        "",
			categoryMatched: false,
			uncensor:        true,
			uncensorMatched: true,
			suffixes:        []string{"LEAK"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := cl.Clean(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.normalized, res.Normalized)
			require.Equal(t, tc.numberID, res.NumberID)
			require.Equal(t, tc.status, res.Status)
			require.Equal(t, tc.category, res.Category)
			require.Equal(t, tc.categoryMatched, res.CategoryMatched)
			require.Equal(t, tc.uncensor, res.Uncensor)
			require.Equal(t, tc.uncensorMatched, res.UncensorMatched)
			require.Equal(t, tc.suffixes, res.Suffixes)
		})
	}
}

func TestNormalizeHyphenPostProcessor(t *testing.T) {
	cl, err := NewCleaner(loadTestRuleSet(t))
	require.NoError(t, err)

	res, err := cl.Clean("ABC_123.mp4")
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, res.Status)
	require.Equal(t, "ABC-123", res.Normalized)
	require.NotContains(t, res.Normalized, "_")
}

func TestCleanerNoMatch(t *testing.T) {
	cl, err := NewCleaner(loadTestRuleSet(t))
	require.NoError(t, err)

	res, err := cl.Clean("pure-noise-file-name")
	require.NoError(t, err)
	require.Equal(t, StatusNoMatch, res.Status)
	require.Empty(t, res.Normalized)
	require.Empty(t, res.NumberID)
}

func TestCleanerExplain(t *testing.T) {
	cl, err := NewCleaner(loadTestRuleSet(t))
	require.NoError(t, err)

	explain, err := cl.Explain("[VID] rawxppv12345 sub.mp4")
	require.NoError(t, err)
	require.NotNil(t, explain)
	require.NotNil(t, explain.Final)
	require.Equal(t, "RAWX-PPV-12345-C", explain.Final.Normalized)
	require.NotEmpty(t, explain.Steps)

	var hasSuffix bool
	var hasMatcher bool
	var hasSelected bool
	var hasParse bool
	for _, step := range explain.Steps {
		switch {
		case step.Stage == "suffix_rules" && step.Matched:
			hasSuffix = true
		case step.Stage == "matchers" && step.Candidate != nil:
			hasMatcher = true
		case step.Stage == "result" && step.Rule == "selected_candidate" && step.Selected:
			hasSelected = true
		case step.Stage == "result" && step.Rule == "number_parse" && step.Matched:
			hasParse = true
		}
	}
	require.True(t, hasSuffix)
	require.True(t, hasMatcher)
	require.True(t, hasSelected)
	require.True(t, hasParse)
}

func TestCleanerExplainNoMatch(t *testing.T) {
	cl, err := NewCleaner(loadTestRuleSet(t))
	require.NoError(t, err)

	explain, err := cl.Explain("pure-noise-file-name")
	require.NoError(t, err)
	require.NotNil(t, explain)
	require.NotNil(t, explain.Final)
	require.Equal(t, StatusNoMatch, explain.Final.Status)

	var hasNoMatch bool
	for _, step := range explain.Steps {
		if step.Stage == "result" && step.Rule == "selected_candidate" && !step.Matched {
			hasNoMatch = true
			require.Equal(t, "no candidate matched", step.Summary)
		}
	}
	require.True(t, hasNoMatch)
}

func TestMergeRuleSets(t *testing.T) {
	base := loadTestRuleSet(t)

	overrideRaw := []byte(`
version: v1
rewrite_rules:
  - name: rewrite_rawx_ppv
    pattern: '(?i)^RAWX[-_\s]?([0-9]{4,})$'
    replace: 'RAWX-PPV-$1'
matchers:
  - name: generic_censored
    pattern: '(?i)\b([A-Z]{3,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 99
post_processors:
  - name: reorder_suffix
    disabled: true
`)
	override, err := NewLoader().Load(overrideRaw)
	require.NoError(t, err)

	merged, err := MergeRuleSets(base, override)
	require.NoError(t, err)
	require.NotNil(t, merged)
	require.Len(t, merged.PostProcessors, len(base.PostProcessors)-1)
	require.Len(t, merged.RewriteRules, len(base.RewriteRules))

	found := false
	for _, item := range merged.Matchers {
		if item.Name == "generic_censored" {
			found = true
			require.Equal(t, 99, item.Score)
		}
	}
	require.True(t, found)
}

func TestPassthroughCleaner(t *testing.T) {
	cl := NewPassthroughCleaner()

	t.Run("clean", func(t *testing.T) {
		res, err := cl.Clean("ABC-123.mp4")
		require.NoError(t, err)
		assert.Equal(t, StatusLowQuality, res.Status)
		assert.Equal(t, ConfidenceLow, res.Confidence)
		assert.Equal(t, "ABC-123.mp4", res.RawInput)
		assert.Equal(t, "ABC-123.mp4", res.InputNoExt)
		assert.Empty(t, res.Normalized)
		assert.Empty(t, res.NumberID)
		assert.False(t, res.CategoryMatched)
		assert.False(t, res.UncensorMatched)
		assert.Contains(t, res.Warnings, "movieid cleaner disabled")
	})

	t.Run("explain", func(t *testing.T) {
		res, err := cl.Explain("ABC-123.mp4")
		require.NoError(t, err)
		require.NotNil(t, res)
		require.NotNil(t, res.Final)
		assert.Equal(t, StatusLowQuality, res.Final.Status)
		assert.Equal(t, "ABC-123.mp4", res.Input)
		require.Len(t, res.Steps, 1)
		assert.Equal(t, "result", res.Steps[0].Stage)
		assert.Equal(t, "passthrough", res.Steps[0].Rule)
		assert.False(t, res.Steps[0].Matched)
	})
}

func TestSuffixRank(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"c_suffix", "C", 1},
		{"uc_suffix", "UC", 2},
		{"u_suffix", "U", 2},
		{"leak_suffix", "LEAK", 3},
		{"vr_suffix", "VR", 4},
		{"4k_suffix", "4K", 5},
		{"8k_suffix", "8K", 6},
		{"cd1_suffix", "CD1", 7},
		{"cd2_suffix", "CD2", 7},
		{"unknown_suffix", "FOO", 99},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, suffixRank(tc.input))
		})
	}
}

func TestConfidenceByScore(t *testing.T) {
	tests := []struct {
		name     string
		score    int
		expected Confidence
	}{
		{"high_score_100", 100, ConfidenceHigh},
		{"high_score_90", 90, ConfidenceHigh},
		{"medium_score_70", 70, ConfidenceMedium},
		{"medium_score_89", 89, ConfidenceMedium},
		{"low_score_69", 69, ConfidenceLow},
		{"low_score_0", 0, ConfidenceLow},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, confidenceByScore(tc.score))
		})
	}
}

func TestIsLikelyExt(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"mp4", ".mp4", true},
		{"avi", ".avi", true},
		{"mkv", ".mkv", true},
		{"too_short", ".", false},
		{"empty", "", false},
		{"too_long", ".abcdefgh", false},
		{"has_special_chars", ".mp-4", false},
		{"has_space", ".mp 4", false},
		{"uppercase", ".MP4", true},
		{"digits_only", ".123", true},
		{"mixed", ".h264", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isLikelyExt(tc.ext))
		})
	}
}

func TestApplyNormalizer(t *testing.T) {
	opts := Options{CaseMode: "upper", CollapseSpaces: true}

	tests := []struct {
		name     string
		input    string
		rule     compiledNormalizerRule
		opts     Options
		expected string
	}{
		{
			name:     "replace_type_with_replacer",
			input:    "foo_bar",
			rule:     compiledNormalizerRule{typ: "replace", replacer: newTestReplacer("_", "-")},
			opts:     opts,
			expected: "foo-bar",
		},
		{
			name:     "replace_type_nil_replacer",
			input:    "foo_bar",
			rule:     compiledNormalizerRule{typ: "replace"},
			opts:     opts,
			expected: "foo_bar",
		},
		{
			name:     "builtin_basename",
			input:    "/path/to/file.mp4",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "basename"},
			opts:     opts,
			expected: "file.mp4",
		},
		{
			name:     "builtin_strip_ext",
			input:    "file.mp4",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "strip_ext"},
			opts:     opts,
			expected: "file",
		},
		{
			name:     "builtin_strip_ext_no_ext",
			input:    "file",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "strip_ext"},
			opts:     opts,
			expected: "file",
		},
		{
			name:     "builtin_strip_ext_unlikely_ext",
			input:    "file.this-is-not-ext",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "strip_ext"},
			opts:     opts,
			expected: "file.this-is-not-ext",
		},
		{
			name:     "builtin_trim_space",
			input:    "  hello  ",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "trim_space"},
			opts:     opts,
			expected: "hello",
		},
		{
			name:     "builtin_collapse_spaces_enabled",
			input:    "a  b   c",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "collapse_spaces"},
			opts:     opts,
			expected: "a b c",
		},
		{
			name:     "builtin_collapse_spaces_disabled",
			input:    "a  b   c",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "collapse_spaces"},
			opts:     Options{CollapseSpaces: false},
			expected: "a  b   c",
		},
		{
			name:     "builtin_to_upper",
			input:    "abc",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "to_upper"},
			opts:     opts,
			expected: "ABC",
		},
		{
			name:     "builtin_to_upper_non_upper_mode",
			input:    "abc",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "to_upper"},
			opts:     Options{CaseMode: "lower"},
			expected: "abc",
		},
		{
			name:     "builtin_replace_pairs_with_replacer",
			input:    "x_y",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "replace_pairs", replacer: newTestReplacer("_", "-")},
			opts:     opts,
			expected: "x-y",
		},
		{
			name:     "builtin_replace_pairs_nil_replacer",
			input:    "x_y",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "replace_pairs"},
			opts:     opts,
			expected: "x_y",
		},
		{
			name:     "builtin_unknown",
			input:    "hello",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "nonexistent"},
			opts:     opts,
			expected: "hello",
		},
		{
			name:     "unknown_type",
			input:    "hello",
			rule:     compiledNormalizerRule{typ: "unknown_type"},
			opts:     opts,
			expected: "hello",
		},
		{
			name:     "builtin_fullwidth_to_halfwidth",
			input:    "\uff21\uff22\uff23",
			rule:     compiledNormalizerRule{typ: "builtin", builtin: "fullwidth_to_halfwidth"},
			opts:     opts,
			expected: "ABC",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, applyNormalizer(tc.input, tc.rule, tc.opts))
		})
	}
}

func newTestReplacer(oldnew ...string) *strings.Replacer {
	return strings.NewReplacer(oldnew...)
}

func TestRemoveNoiseToken(t *testing.T) {
	aliasRegex := compileAliasRegex([]string{"noise", "junk"})

	tests := []struct {
		name        string
		input       string
		rule        compiledNoiseRule
		wantMatched bool
	}{
		{
			name:        "token_matched",
			input:       "ABC-123 noise DEF",
			rule:        compiledNoiseRule{name: "test_token", typ: "token", aliasRegex: aliasRegex},
			wantMatched: true,
		},
		{
			name:        "token_not_matched",
			input:       "ABC-123 clean",
			rule:        compiledNoiseRule{name: "test_token", typ: "token", aliasRegex: aliasRegex},
			wantMatched: false,
		},
		{
			name:        "empty_alias_regex",
			input:       "ABC-123",
			rule:        compiledNoiseRule{name: "test_token", typ: "token", aliasRegex: nil},
			wantMatched: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, matched := removeNoiseToken(tc.input, tc.rule, nil)
			assert.Equal(t, tc.wantMatched, matched)
		})
	}
}

func TestRemoveNoiseTokenWithExplain(t *testing.T) {
	aliasRegex := compileAliasRegex([]string{"noise"})
	collector := &explainCollector{}

	rule := compiledNoiseRule{name: "test_token", typ: "token", aliasRegex: aliasRegex}
	_, matched := removeNoiseToken("ABC noise DEF", rule, collector)
	assert.True(t, matched)
	require.NotEmpty(t, collector.steps)
	assert.Equal(t, "noise removed", collector.steps[len(collector.steps)-1].Summary)
}

func TestRemoveNoiseWithExplainUnsupportedType(t *testing.T) {
	rules := []compiledNoiseRule{
		{name: "bad_type", typ: "unsupported"},
	}
	collector := &explainCollector{}
	out, hits := removeNoiseWithExplain("ABC-123", rules, collector)
	assert.Equal(t, "ABC-123", out)
	assert.Empty(t, hits)
	require.NotEmpty(t, collector.steps)
	assert.Equal(t, "unsupported noise rule type", collector.steps[0].Summary)
}

func TestDedupeCandidates(t *testing.T) {
	tests := []struct {
		name     string
		input    []Candidate
		expected []Candidate
	}{
		{
			name:     "empty",
			input:    nil,
			expected: []Candidate{},
		},
		{
			name: "no_duplicates",
			input: []Candidate{
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r1"}},
				{NumberID: "B-2", Score: 20, RuleHits: []string{"r2"}},
			},
			expected: []Candidate{
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r1"}},
				{NumberID: "B-2", Score: 20, RuleHits: []string{"r2"}},
			},
		},
		{
			name: "merge_category",
			input: []Candidate{
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r1"}, CategoryMatched: false},
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r2"}, Category: "CAT", CategoryMatched: true},
			},
			expected: []Candidate{
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r1", "r2"}, Category: "CAT", CategoryMatched: true},
			},
		},
		{
			name: "merge_uncensor",
			input: []Candidate{
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r1"}, UncensorMatched: false},
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r2"}, Uncensor: true, UncensorMatched: true},
			},
			expected: []Candidate{
				{NumberID: "A-1", Score: 10, RuleHits: []string{"r1", "r2"}, Uncensor: true, UncensorMatched: true},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeCandidates(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSortSuffixSet(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]int
		expected []string
	}{
		{
			name:     "empty",
			input:    map[string]int{},
			expected: []string{},
		},
		{
			name:     "single",
			input:    map[string]int{"C": 10},
			expected: []string{"C"},
		},
		{
			name:     "same_priority_sorted_by_rank",
			input:    map[string]int{"VR": 10, "C": 10, "LEAK": 10},
			expected: []string{"C", "LEAK", "VR"},
		},
		{
			name:     "different_priorities",
			input:    map[string]int{"C": 10, "LEAK": 30, "CD1": 20},
			expected: []string{"LEAK", "CD1", "C"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sortSuffixSet(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestCompileNormalizers(t *testing.T) {
	tests := []struct {
		name        string
		input       []NormalizerRule
		expectedLen int
		hasReplacer bool
	}{
		{
			name:        "empty",
			input:       nil,
			expectedLen: 0,
		},
		{
			name: "disabled_skipped",
			input: []NormalizerRule{
				{Name: "skip_me", Type: "builtin", Builtin: "basename", Disabled: true},
			},
			expectedLen: 0,
		},
		{
			name: "replace_with_pairs",
			input: []NormalizerRule{
				{Name: "rep", Type: "replace", Pairs: map[string]string{"a": "b", "c": "d"}},
			},
			expectedLen: 1,
			hasReplacer: true,
		},
		{
			name: "replace_without_pairs",
			input: []NormalizerRule{
				{Name: "rep", Type: "replace"},
			},
			expectedLen: 1,
			hasReplacer: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compileNormalizers(tc.input)
			assert.Len(t, got, tc.expectedLen)
			if tc.hasReplacer && tc.expectedLen > 0 {
				assert.NotNil(t, got[0].replacer)
			}
		})
	}
}

func TestCompileAliasRegex(t *testing.T) {
	tests := []struct {
		name     string
		aliases  []string
		inputStr string
		match    bool
	}{
		{
			name:     "normal_alias",
			aliases:  []string{"SUB"},
			inputStr: "ABC SUB DEF",
			match:    true,
		},
		{
			name:     "empty_alias_skipped",
			aliases:  []string{"  "},
			inputStr: "anything",
			match:    false,
		},
		{
			name:     "no_aliases",
			aliases:  nil,
			inputStr: "anything",
			match:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			regexes := compileAliasRegex(tc.aliases)
			matched := false
			for _, re := range regexes {
				if re.MatchString(tc.inputStr) {
					matched = true
				}
			}
			assert.Equal(t, tc.match, matched)
		})
	}
}

func TestResolveCanonical(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		canonical string
		tmpl      string
		re        *regexp.Regexp
		match     []int
		expected  string
	}{
		{
			name:      "with_template",
			src:       "DISC3",
			canonical: "",
			tmpl:      "CD$1",
			re:        regexp.MustCompile(`DISC(\d+)`),
			match:     []int{0, 5, 4, 5},
			expected:  "CD3",
		},
		{
			name:      "without_template",
			src:       "anything",
			canonical: "C",
			tmpl:      "",
			re:        regexp.MustCompile(`.*`),
			match:     []int{0, 8},
			expected:  "C",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCanonical(tc.src, tc.canonical, tc.tmpl, tc.re, tc.match)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestExplainCollectorNil(_ *testing.T) {
	var c *explainCollector
	c.add("stage", "rule", "in", "out", true, "summary")
	c.addWithValues("stage", "rule", "in", "out", true, "summary", []string{"v"})
	c.addCandidate("stage", "rule", "in", "out", true, "summary", Candidate{}, true)
}

func TestExplainCollectorAddWithValues(t *testing.T) {
	c := &explainCollector{}
	c.addWithValues("stage", "rule", "in", "out", true, "summary", nil)
	require.Len(t, c.steps, 1)
	assert.Nil(t, c.steps[0].Values)

	c.addWithValues("stage", "rule", "in", "out", true, "summary", []string{"v1"})
	require.Len(t, c.steps, 2)
	assert.Equal(t, []string{"v1"}, c.steps[1].Values)
}

func TestBuildMatchResultLowConfidence(t *testing.T) {
	rs := &RuleSet{
		Version: "v1",
		Matchers: []MatcherRule{
			{Name: "m1", Pattern: `(?i)([A-Z]+)(\d+)`, NormalizeTemplate: "$1-$2", Score: 10},
		},
		PostProcessors: []PostProcessRule{
			{Name: "normalize_hyphen", Type: "builtin", Builtin: "normalize_hyphen"},
		},
	}
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	res, err := cl.Clean("ABC123")
	require.NoError(t, err)
	assert.Equal(t, StatusLowQuality, res.Status)
	assert.Equal(t, ConfidenceLow, res.Confidence)
	assert.Contains(t, res.Warnings, "low confidence candidate")
}

func TestBuildMatchResultMediumConfidence(t *testing.T) {
	rs := &RuleSet{
		Version: "v1",
		Matchers: []MatcherRule{
			{Name: "m1", Pattern: `(?i)([A-Z]+)[-_]?(\d+)`, NormalizeTemplate: "$1-$2", Score: 75},
		},
		PostProcessors: []PostProcessRule{
			{Name: "normalize_hyphen", Type: "builtin", Builtin: "normalize_hyphen"},
		},
	}
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	res, err := cl.Clean("ABC-123")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, res.Status)
	assert.Equal(t, ConfidenceMedium, res.Confidence)
}

func TestRebuildWithExplainEmptySuffix(t *testing.T) {
	suffixes := []string{"", "C"}
	collector := &explainCollector{}
	result := rebuildWithExplain("ABC-123", suffixes, nil, collector)
	assert.Equal(t, "ABC-123-C", result)
}

func TestRebuildWithExplainEmptyNumberID(t *testing.T) {
	suffixes := []string{"C"}
	result := rebuildWithExplain("", suffixes, nil, nil)
	assert.Equal(t, "C", result)
}

func TestExtractSuffixesUnsupportedType(t *testing.T) {
	rules := []compiledSuffixRule{
		{name: "bad", typ: "unsupported"},
	}
	collector := &explainCollector{}
	suffixes, work, hits := extractSuffixesWithExplain("ABC-123", rules, collector)
	assert.Empty(t, suffixes)
	assert.Equal(t, "ABC-123", work)
	assert.Empty(t, hits)
	require.NotEmpty(t, collector.steps)
}

func TestExtractSuffixRegexEmptyVal(t *testing.T) {
	re := regexp.MustCompile(`(?i)\b(DISC)\s*\b`)
	rule := compiledSuffixRule{
		name:              "disc",
		typ:               "regex",
		re:                re,
		canonicalTemplate: "",
		canonical:         "",
	}
	suffixSet := make(map[string]int)
	_, collected, _ := extractSuffixRegex("DISC something", rule, suffixSet)
	assert.Empty(t, collected)
}

func TestCompileRuleSetErrorPaths(t *testing.T) {
	t.Run("bad_rewrite_pattern", func(t *testing.T) {
		rs := &RuleSet{
			Version:      "v1",
			RewriteRules: []RewriteRule{{Name: "bad", Pattern: "[invalid"}},
		}
		_, err := compileRuleSet(rs)
		require.Error(t, err)
	})

	t.Run("bad_suffix_pattern", func(t *testing.T) {
		rs := &RuleSet{
			Version:     "v1",
			SuffixRules: []SuffixRule{{Name: "bad", Type: "regex", Pattern: "[invalid", Canonical: "C"}},
		}
		_, err := compileRuleSet(rs)
		require.Error(t, err)
	})

	t.Run("bad_noise_pattern", func(t *testing.T) {
		rs := &RuleSet{
			Version:    "v1",
			NoiseRules: []NoiseRule{{Name: "bad", Type: "regex", Pattern: "[invalid"}},
		}
		_, err := compileRuleSet(rs)
		require.Error(t, err)
	})

	t.Run("bad_matcher_pattern", func(t *testing.T) {
		rs := &RuleSet{
			Version:  "v1",
			Matchers: []MatcherRule{{Name: "bad", Pattern: "[invalid", NormalizeTemplate: "$1"}},
		}
		_, err := compileRuleSet(rs)
		require.Error(t, err)
	})

	t.Run("disabled_rules_skipped", func(t *testing.T) {
		rs := &RuleSet{
			Version:      "v1",
			RewriteRules: []RewriteRule{{Name: "skip", Pattern: "[invalid", Disabled: true}},
			PostProcessors: []PostProcessRule{
				{Name: "skip", Type: "builtin", Builtin: "normalize_hyphen", Disabled: true},
			},
		}
		crs, err := compileRuleSet(rs)
		require.NoError(t, err)
		assert.Empty(t, crs.rewriteRules)
		assert.Empty(t, crs.postProcessors)
	})

	t.Run("default_case_mode", func(t *testing.T) {
		rs := &RuleSet{Version: "v1"}
		crs, err := compileRuleSet(rs)
		require.NoError(t, err)
		assert.Equal(t, "upper", crs.options.CaseMode)
	})
}

func TestCompileMatcherRulesUncensor(t *testing.T) {
	boolTrue := true
	rules := []MatcherRule{
		{Name: "m1", Pattern: `(?i)([A-Z]+)(\d+)`, NormalizeTemplate: "$1-$2", Score: 80, Uncensor: &boolTrue},
	}
	compiled, err := compileMatcherRules(rules)
	require.NoError(t, err)
	require.Len(t, compiled, 1)
	assert.True(t, compiled[0].uncensorSet)
	assert.True(t, compiled[0].uncensorValue)
}

func TestCompileSuffixRulesToken(t *testing.T) {
	rules := []SuffixRule{
		{Name: "s1", Type: "token", Aliases: []string{"SUB"}, Canonical: "C"},
	}
	compiled, err := compileSuffixRules(rules)
	require.NoError(t, err)
	require.Len(t, compiled, 1)
	assert.NotEmpty(t, compiled[0].aliasRegex)
}

func TestCompileNoiseRulesToken(t *testing.T) {
	rules := []NoiseRule{
		{Name: "n1", Type: "token", Aliases: []string{"JUNK"}},
	}
	compiled, err := compileNoiseRules(rules)
	require.NoError(t, err)
	require.Len(t, compiled, 1)
	assert.NotEmpty(t, compiled[0].aliasRegex)
}

func TestCollectCandidatesEmptyNormalized(t *testing.T) {
	rules := []compiledMatcherRule{
		{
			name:              "empty_match",
			re:                regexp.MustCompile(`(nothing)?`),
			normalizeTemplate: "$1",
			score:             50,
		},
	}
	collector := &explainCollector{}
	candidates := collectCandidatesWithExplain("ABC-123", rules, collector)
	assert.Empty(t, candidates)
}

func TestCleanerWithNoiseTokenRules(t *testing.T) {
	rs := &RuleSet{
		Version: "v1",
		NoiseRules: []NoiseRule{
			{Name: "remove_junk", Type: "token", Aliases: []string{"JUNK"}},
		},
		Matchers: []MatcherRule{
			{Name: "m1", Pattern: `(?i)([A-Z]+)[-_]?(\d+)`, NormalizeTemplate: "$1-$2", Score: 80},
		},
		PostProcessors: []PostProcessRule{
			{Name: "normalize_hyphen", Type: "builtin", Builtin: "normalize_hyphen"},
		},
	}
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	res, err := cl.Clean("JUNK ABC-123")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, res.Status)
	assert.Contains(t, res.RuleHits, "remove_junk")
}

func TestNewCleanerValidationError(t *testing.T) {
	_, err := NewCleaner(nil)
	require.Error(t, err)
}

func TestMergeRuleSetsEdgeCases(t *testing.T) {
	t.Run("both_nil", func(t *testing.T) {
		_, err := MergeRuleSets(nil, nil)
		require.Error(t, err)
	})
	t.Run("base_nil", func(t *testing.T) {
		override := &RuleSet{Version: "v1"}
		merged, err := MergeRuleSets(nil, override)
		require.NoError(t, err)
		assert.Equal(t, "v1", merged.Version)
	})
	t.Run("override_nil", func(t *testing.T) {
		base := &RuleSet{Version: "v1"}
		merged, err := MergeRuleSets(base, nil)
		require.NoError(t, err)
		assert.Equal(t, "v1", merged.Version)
	})
}

func TestBuildMatchResultParseError(t *testing.T) {
	rs := &RuleSet{
		Version: "v1",
		Matchers: []MatcherRule{
			{Name: "m1", Pattern: `([\w.]+)`, NormalizeTemplate: "$1", Score: 95},
		},
	}
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	// number.Parse rejects strings containing "."
	explainRes, err := cl.Explain("foo.bar")
	require.NoError(t, err)
	assert.Equal(t, StatusLowQuality, explainRes.Final.Status)

	var hasParseFail bool
	for _, step := range explainRes.Steps {
		if step.Stage == "result" && step.Rule == "number_parse" && !step.Matched {
			hasParseFail = true
		}
	}
	assert.True(t, hasParseFail)
}

func TestCleanerExplainWithRewriteMatch(t *testing.T) {
	rs := &RuleSet{
		Version: "v1",
		RewriteRules: []RewriteRule{
			{Name: "rw", Pattern: `(?i)RAWX(\d+)`, Replace: "RAWX-PPV-$1"},
		},
		Matchers: []MatcherRule{
			{Name: "m1", Pattern: `(?i)([A-Z]+[-A-Z]*)[-_]?(\d+)`, NormalizeTemplate: "$1-$2", Score: 80},
		},
		PostProcessors: []PostProcessRule{
			{Name: "normalize_hyphen", Type: "builtin", Builtin: "normalize_hyphen"},
		},
	}
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	explainRes, err := cl.Explain("RAWX12345")
	require.NoError(t, err)

	var hasRewrite bool
	for _, step := range explainRes.Steps {
		if step.Stage == "rewrite_rules" && step.Matched {
			hasRewrite = true
		}
	}
	assert.True(t, hasRewrite)
}

func TestCleanerExplainWithNoiseRegex(t *testing.T) {
	rs := &RuleSet{
		Version: "v1",
		NoiseRules: []NoiseRule{
			{Name: "domains", Type: "regex", Pattern: `(?i)\b\w+\.com\b`},
		},
		Matchers: []MatcherRule{
			{Name: "m1", Pattern: `(?i)([A-Z]+)[-_]?(\d+)`, NormalizeTemplate: "$1-$2", Score: 80},
		},
		PostProcessors: []PostProcessRule{
			{Name: "normalize_hyphen", Type: "builtin", Builtin: "normalize_hyphen"},
		},
	}
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	explainRes, err := cl.Explain("example.com ABC-123")
	require.NoError(t, err)

	var hasNoiseStep bool
	for _, step := range explainRes.Steps {
		if step.Stage == "noise_rules" && step.Matched {
			hasNoiseStep = true
		}
	}
	assert.True(t, hasNoiseStep)
}

func TestMergeOptions(t *testing.T) {
	tests := []struct {
		name     string
		base     Options
		override Options
		expected Options
	}{
		{
			name:     "override_case_mode",
			base:     Options{CaseMode: "upper"},
			override: Options{CaseMode: "lower"},
			expected: Options{CaseMode: "lower"},
		},
		{
			name:     "override_collapse_spaces",
			base:     Options{},
			override: Options{CollapseSpaces: true},
			expected: Options{CollapseSpaces: true},
		},
		{
			name:     "override_enable_embedded",
			base:     Options{},
			override: Options{EnableEmbeddedMatch: true},
			expected: Options{EnableEmbeddedMatch: true},
		},
		{
			name:     "override_fail_when_no_match",
			base:     Options{},
			override: Options{FailWhenNoMatch: true},
			expected: Options{FailWhenNoMatch: true},
		},
		{
			name:     "empty_override_preserves_base",
			base:     Options{CaseMode: "upper", CollapseSpaces: true},
			override: Options{},
			expected: Options{CaseMode: "upper", CollapseSpaces: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeOptions(tc.base, tc.override)
			assert.Equal(t, tc.expected, got)
		})
	}
}
