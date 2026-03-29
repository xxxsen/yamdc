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
		input           string
		normalized      string
		numberID        string
		status          Status
		category        string
		categoryMatched bool
		uncensor        bool
		uncensorMatched bool
	}{
		"fc2": {
			input:           "[JAV] fc2ppv12345 中文字幕.mp4",
			normalized:      "FC2-PPV-12345-C",
			numberID:        "FC2-PPV-12345",
			status:          StatusSuccess,
			category:        "FC2",
			categoryMatched: true,
			uncensor:        true,
			uncensorMatched: true,
		},
		"carib": {
			input:           "carib-010123-001 4k.mkv",
			normalized:      "010123-001-4K",
			numberID:        "010123-001",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"generic": {
			input:      "abc123 cd2.avi",
			normalized: "ABC-123-CD2",
			numberID:   "ABC-123",
			status:     StatusSuccess,
		},
		"heyzo": {
			input:           "heyzo_1234 leak.mp4",
			normalized:      "HEYZO-1234-LEAK",
			numberID:        "HEYZO-1234",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"heyzo_with_domain_prefix": {
			input:           "www.baidu.com HEYZO-0001.mp4",
			normalized:      "HEYZO-0001",
			numberID:        "HEYZO-0001",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"heyzo_with_domain_prefix_no_ext": {
			input:           "www.baidu.com HEYZO-0001",
			normalized:      "HEYZO-0001",
			numberID:        "HEYZO-0001",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"heyzo_with_domain_and_at": {
			input:           "8000abc.com@HEYZO-0055.mp4",
			normalized:      "HEYZO-0055",
			numberID:        "HEYZO-0055",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"heyzo_with_domain_and_at_no_ext": {
			input:           "8000abc.com@HEYZO-0055",
			normalized:      "HEYZO-0055",
			numberID:        "HEYZO-0055",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"onepondo": {
			input:           "1pondo-011516_227-C",
			normalized:      "011516_227-C",
			numberID:        "011516_227",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
		},
		"jvr_category": {
			input:           "jvr12345.mp4",
			normalized:      "JVR-12345",
			numberID:        "JVR-12345",
			status:          StatusSuccess,
			category:        "JVR",
			categoryMatched: true,
			uncensor:        true,
			uncensorMatched: true,
		},
		"manyvids_category": {
			input:           "manyvids12345.mp4",
			normalized:      "MANYVIDS-12345",
			numberID:        "MANYVIDS-12345",
			status:          StatusSuccess,
			category:        "MANYVIDS",
			categoryMatched: true,
			uncensor:        true,
			uncensorMatched: true,
		},
		"uncensor_n_code": {
			input:           "N1234.mp4",
			normalized:      "N1234",
			numberID:        "N1234",
			status:          StatusSuccess,
			uncensor:        true,
			uncensorMatched: true,
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
			require.Equal(t, tc.category, res.Category)
			require.Equal(t, tc.categoryMatched, res.CategoryMatched)
			require.Equal(t, tc.uncensor, res.Uncensor)
			require.Equal(t, tc.uncensorMatched, res.UncensorMatched)
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

func TestCleanerRewriteCompatibility(t *testing.T) {
	cl, err := NewCleanerFromBytes(loadDefaultRule(t))
	require.NoError(t, err)

	cases := map[string]string{
		"fc2ppv12345-CD1":      "FC2-PPV-12345-CD1",
		"fc2ppv_1234":          "FC2-PPV-1234",
		"fc2_ppv_1234":         "FC2-PPV-1234",
		"fc2ppv-123":           "FC2-PPV-123",
		"fc2-123445-cd1":       "FC2-PPV-123445-CD1",
		"fc2-12345":            "FC2-PPV-12345",
		"fc2ppv-12345-C-CD1":   "FC2-PPV-12345-C-CD1",
		"carib-1234-222":       "1234-222",
		"1pon-2344-222":        "2344-222",
		"1pondo-1234-222":      "1234-222",
		"madou_aaaa":           "MADOU-AAAA",
		"cospuri-ria-ruok-1a2": "COSPURI-RIA-RUOK-1A2",
	}

	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			res, err := cl.Clean(input)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Equal(t, expected, res.Normalized)
		})
	}
}

func TestCleanerCategoryCompatibility(t *testing.T) {
	cl, err := NewCleanerFromBytes(loadDefaultRule(t))
	require.NoError(t, err)

	cases := map[string]struct {
		category string
		matched  bool
	}{
		"fc2-ppv-1234":              {category: "FC2", matched: true},
		"jvr-12345":                 {category: "JVR", matched: true},
		"qqqq":                      {category: "", matched: false},
		"HEYZO-12345":               {category: "", matched: false},
		"COSPURI-Emiri-Momota-0548": {category: "COSPURI", matched: true},
		"COSPURI-123456":            {category: "COSPURI", matched: true},
		"cospuri-123456":            {category: "COSPURI", matched: true},
		"MADOU-123456":              {category: "MD", matched: true},
		"MADOU_aaaa":                {category: "MD", matched: true},
		"MADOU_bbbb":                {category: "MD", matched: true},
		"MANYVIDS-123456":           {category: "MANYVIDS", matched: true},
	}

	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			res, err := cl.Clean(input)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Equal(t, expected.category, res.Category)
			require.Equal(t, expected.matched, res.CategoryMatched)
		})
	}
}

func TestCleanerUncensorCompatibility(t *testing.T) {
	cl, err := NewCleanerFromBytes(loadDefaultRule(t))
	require.NoError(t, err)

	cases := map[string]bool{
		"FC2-PPV-123":               true,
		"HEYZO-222":                 true,
		"1PON-12345":                true,
		"MXX-22222":                 true,
		"JVR-22222":                 true,
		"H0930-22222":               true,
		"DSAM-22222":                true,
		"CARIB-22222":               true,
		"SM3D2DBD-22222":            true,
		"SSDV-22222":                true,
		"112214_292":                true,
		"112114-291":                true,
		"n11451":                    true,
		"heyzo_1545":                true,
		"hey-1111":                  true,
		"carib-11111-222":           true,
		"22222-333":                 true,
		"010111-222":                true,
		"H4610-Ki1111":              true,
		"MKD-12345":                 true,
		"fc2-ppv-12345":             true,
		"1pon-123":                  true,
		"smd-1234":                  true,
		"kb2134":                    true,
		"c0930-ki240528":            true,
		"YMDS-164":                  false,
		"MBRBI-002":                 false,
		"LUKE-036":                  false,
		"SMDY-123":                  false,
		"COSPURI-aaa1111":           true,
		"COSPURI-RIA-RUOK-aaaa1111": true,
		"MADOU-xg-123":              true,
		"MADOU-cm-123":              true,
		"MADOU-md-123":              true,
	}

	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			res, err := cl.Clean(input)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Equal(t, expected, res.Uncensor)
			require.Equal(t, expected, res.UncensorMatched)
		})
	}
}

func TestMergeRuleSets(t *testing.T) {
	base, err := NewLoader().Load(loadDefaultRule(t))
	require.NoError(t, err)

	overrideRaw := []byte(`
version: v1
rewrite_rules:
  - name: rewrite_fc2_ppv
    pattern: '(?i)^FC2[-_\s]?([0-9]{4,})$'
    replace: 'FC2-PPV-$1'
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
