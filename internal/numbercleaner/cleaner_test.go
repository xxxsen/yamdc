package numbercleaner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func loadDefaultRule(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "rules", "number_cleaner.yaml"))
	require.NoError(t, err)
	return data
}

func TestCleanerClean(t *testing.T) {
	cl, err := NewCleanerFromBytes(loadDefaultRule(t))
	require.NoError(t, err)

	cases := map[string]struct {
		input      string
		normalized string
		numberID   string
		status     Status
	}{
		"fc2": {
			input:      "[JAV] fc2ppv12345 中文字幕.mp4",
			normalized: "FC2-PPV-12345-C",
			numberID:   "FC2-PPV-12345",
			status:     StatusSuccess,
		},
		"carib": {
			input:      "carib-010123-001 4k.mkv",
			normalized: "010123-001-4K",
			numberID:   "010123-001",
			status:     StatusSuccess,
		},
		"generic": {
			input:      "abc123 cd2.avi",
			normalized: "ABC-123-CD2",
			numberID:   "ABC-123",
			status:     StatusSuccess,
		},
		"heyzo": {
			input:      "heyzo_1234 leak.mp4",
			normalized: "HEYZO-1234-LEAK",
			numberID:   "HEYZO-1234",
			status:     StatusSuccess,
		},
		"heyzo_with_domain_prefix": {
			input:      "www.baidu.com HEYZO-0001.mp4",
			normalized: "HEYZO-0001",
			numberID:   "HEYZO-0001",
			status:     StatusSuccess,
		},
		"heyzo_with_domain_prefix_no_ext": {
			input:      "www.baidu.com HEYZO-0001",
			normalized: "HEYZO-0001",
			numberID:   "HEYZO-0001",
			status:     StatusSuccess,
		},
		"heyzo_with_domain_and_at": {
			input:      "8000abc.com@HEYZO-0055.mp4",
			normalized: "HEYZO-0055",
			numberID:   "HEYZO-0055",
			status:     StatusSuccess,
		},
		"heyzo_with_domain_and_at_no_ext": {
			input:      "8000abc.com@HEYZO-0055",
			normalized: "HEYZO-0055",
			numberID:   "HEYZO-0055",
			status:     StatusSuccess,
		},
		"onepondo": {
			input:      "1pondo-011516_227-C",
			normalized: "011516_227-C",
			numberID:   "011516_227",
			status:     StatusSuccess,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := cl.Clean(tc.input)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Equal(t, tc.normalized, res.Normalized)
			require.Equal(t, tc.numberID, res.NumberID)
			require.Equal(t, tc.status, res.Status)
		})
	}
}

func TestCleanerNoMatch(t *testing.T) {
	cl, err := NewCleanerFromBytes(loadDefaultRule(t))
	require.NoError(t, err)

	res, err := cl.Clean("pure-noise-file-name")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, StatusNoMatch, res.Status)
	require.Empty(t, res.Normalized)
}

func TestMergeRuleSets(t *testing.T) {
	base, err := NewLoader().Load(loadDefaultRule(t))
	require.NoError(t, err)

	overrideRaw := []byte(`
version: v1
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
	found := false
	for _, item := range merged.Matchers {
		if item.Name == "generic_censored" {
			found = true
			require.Equal(t, 99, item.Score)
		}
	}
	require.True(t, found)
}
