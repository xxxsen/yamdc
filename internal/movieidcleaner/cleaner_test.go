package movieidcleaner

import (
	"path/filepath"
	"testing"

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
