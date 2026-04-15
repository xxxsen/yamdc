package movieidcleaner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCleanerFromBytes(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name: "valid_yaml",
			data: []byte(`
version: v1
matchers:
  - name: generic
    pattern: '(?i)([A-Z]+)[-_]?(\d+)'
    normalize_template: '$1-$2'
    score: 80
`),
			wantErr: false,
		},
		{
			name:    "invalid_yaml",
			data:    []byte(`not: [valid: yaml`),
			wantErr: true,
		},
		{
			name:    "missing_version",
			data:    []byte(`matchers: []`),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cl, err := NewCleanerFromBytes(tc.data)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, cl)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cl)
			}
		})
	}
}

func TestLoadRuleSetFromDirDirect(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0o600))

	rs, err := LoadRuleSetFromDir(dir)
	require.NoError(t, err)
	require.NotNil(t, rs)
	assert.Equal(t, "v1", rs.Version)
}

func TestListRuleSetFilesFromDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-base.yaml"), []byte(`version: v1`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "002-extra.yml"), []byte(`version: v1`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(`not a rule`), 0o600))

	files, err := ListRuleSetFilesFromDir(dir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestLoadRuleSetFromPathFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "rules.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(`
version: v1
matchers:
  - name: generic
    pattern: '(?i)([A-Z]+)[-_]?(\d+)'
    normalize_template: '$1-$2'
    score: 80
`), 0o600))

	rs, err := LoadRuleSetFromPath(tmpFile)
	require.NoError(t, err)
	require.NotNil(t, rs)
	assert.Equal(t, "v1", rs.Version)
	assert.Len(t, rs.Matchers, 1)
}

func TestLoadRuleSetFromPathFileReadError(t *testing.T) {
	_, err := LoadRuleSetFromPath("/nonexistent/file.yaml")
	require.Error(t, err)
}

func TestLoadRuleSetFromPathDirNoManifest(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0o600))

	rs, err := LoadRuleSetFromPath(dir)
	require.NoError(t, err)
	require.NotNil(t, rs)
	assert.Equal(t, "v1", rs.Version)
}

func TestCollectRuleSetFilesFromFSNoYaml(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(`hello`), 0o600))

	_, err := LoadRuleSetFromDir(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no yaml files found")
}

func TestValidateRuleSet(t *testing.T) {
	tests := []struct {
		name    string
		rs      *RuleSet
		wantErr string
	}{
		{
			name:    "nil_ruleset",
			rs:      nil,
			wantErr: "rule set is nil",
		},
		{
			name:    "empty_version",
			rs:      &RuleSet{},
			wantErr: "rule set version is required",
		},
		{
			name: "duplicate_normalizer_name",
			rs: &RuleSet{
				Version: "v1",
				Normalizers: []NormalizerRule{
					{Name: "dup", Type: "builtin", Builtin: "basename"},
					{Name: "dup", Type: "builtin", Builtin: "trim_space"},
				},
			},
			wantErr: "duplicate normalizer name: dup",
		},
		{
			name: "unsupported_normalizer_builtin",
			rs: &RuleSet{
				Version: "v1",
				Normalizers: []NormalizerRule{
					{Name: "bad", Type: "builtin", Builtin: "nonexistent"},
				},
			},
			wantErr: "unsupported normalizer builtin",
		},
		{
			name: "empty_rewrite_pattern",
			rs: &RuleSet{
				Version:      "v1",
				RewriteRules: []RewriteRule{{Name: "rw", Pattern: ""}},
			},
			wantErr: "rewrite rule pattern is required",
		},
		{
			name: "invalid_rewrite_regexp",
			rs: &RuleSet{
				Version:      "v1",
				RewriteRules: []RewriteRule{{Name: "rw", Pattern: "[invalid"}},
			},
			wantErr: "compile rewrite rule regexp failed",
		},
		{
			name: "invalid_suffix_regexp",
			rs: &RuleSet{
				Version:     "v1",
				SuffixRules: []SuffixRule{{Name: "s", Type: "regex", Pattern: "[invalid", Canonical: "C"}},
			},
			wantErr: "compile suffix rule regexp failed",
		},
		{
			name: "suffix_missing_canonical",
			rs: &RuleSet{
				Version:     "v1",
				SuffixRules: []SuffixRule{{Name: "s", Type: "regex", Pattern: ".*"}},
			},
			wantErr: "suffix canonical or canonical_template is required",
		},
		{
			name: "suffix_unsupported_canonical",
			rs: &RuleSet{
				Version:     "v1",
				SuffixRules: []SuffixRule{{Name: "s", Type: "regex", Pattern: ".*", Canonical: "INVALID"}},
			},
			wantErr: "unsupported suffix canonical",
		},
		{
			name: "suffix_cd_prefix_canonical_allowed",
			rs: &RuleSet{
				Version:     "v1",
				SuffixRules: []SuffixRule{{Name: "s", Type: "regex", Pattern: ".*", Canonical: "CD5"}},
			},
			wantErr: "",
		},
		{
			name: "invalid_noise_regexp",
			rs: &RuleSet{
				Version:    "v1",
				NoiseRules: []NoiseRule{{Name: "n", Type: "regex", Pattern: "[invalid"}},
			},
			wantErr: "compile noise rule regexp failed",
		},
		{
			name: "matcher_missing_normalize_template",
			rs: &RuleSet{
				Version:  "v1",
				Matchers: []MatcherRule{{Name: "m", Pattern: ".*"}},
			},
			wantErr: "matcher normalize_template is required",
		},
		{
			name: "invalid_matcher_regexp",
			rs: &RuleSet{
				Version:  "v1",
				Matchers: []MatcherRule{{Name: "m", Pattern: "[invalid", NormalizeTemplate: "$1"}},
			},
			wantErr: "compile matcher rule regexp failed",
		},
		{
			name: "unsupported_post_processor",
			rs: &RuleSet{
				Version:        "v1",
				PostProcessors: []PostProcessRule{{Name: "p", Type: "builtin", Builtin: "nonexistent"}},
			},
			wantErr: "unsupported post processor builtin",
		},
		{
			name: "duplicate_post_processor_name",
			rs: &RuleSet{
				Version: "v1",
				PostProcessors: []PostProcessRule{
					{Name: "p1", Type: "builtin", Builtin: "normalize_hyphen"},
					{Name: "p1", Type: "builtin", Builtin: "reorder_suffix"},
				},
			},
			wantErr: "duplicate post processor name: p1",
		},
		{
			name: "duplicate_suffix_name",
			rs: &RuleSet{
				Version: "v1",
				SuffixRules: []SuffixRule{
					{Name: "s1", Type: "token", Canonical: "C"},
					{Name: "s1", Type: "token", Canonical: "UC"},
				},
			},
			wantErr: "duplicate suffix rule name: s1",
		},
		{
			name: "duplicate_noise_name",
			rs: &RuleSet{
				Version: "v1",
				NoiseRules: []NoiseRule{
					{Name: "n1", Type: "regex", Pattern: ".*"},
					{Name: "n1", Type: "regex", Pattern: ".*"},
				},
			},
			wantErr: "duplicate noise rule name: n1",
		},
		{
			name: "duplicate_matcher_name",
			rs: &RuleSet{
				Version: "v1",
				Matchers: []MatcherRule{
					{Name: "m1", Pattern: ".*", NormalizeTemplate: "$0"},
					{Name: "m1", Pattern: ".*", NormalizeTemplate: "$0"},
				},
			},
			wantErr: "duplicate matcher rule name: m1",
		},
		{
			name: "disabled_rules_skip_validation",
			rs: &RuleSet{
				Version: "v1",
				Normalizers: []NormalizerRule{
					{Name: "bad", Type: "builtin", Builtin: "nonexistent", Disabled: true},
				},
				RewriteRules: []RewriteRule{
					{Name: "bad", Pattern: "[invalid", Disabled: true},
				},
				SuffixRules: []SuffixRule{
					{Name: "bad", Type: "regex", Pattern: "[invalid", Disabled: true},
				},
				NoiseRules: []NoiseRule{
					{Name: "bad", Type: "regex", Pattern: "[invalid", Disabled: true},
				},
				Matchers: []MatcherRule{
					{Name: "bad", Pattern: "[invalid", Disabled: true},
				},
				PostProcessors: []PostProcessRule{
					{Name: "bad", Type: "builtin", Builtin: "nonexistent", Disabled: true},
				},
			},
			wantErr: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRuleSet(tc.rs)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateUniqueRuleNamesEmptyName(t *testing.T) {
	items := []NormalizerRule{
		{Name: "", Type: "builtin", Builtin: "basename"},
	}
	err := validateUniqueRuleNames(items, "normalizer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "normalizer name is required")
}

func TestMergeRuleSetFragments(t *testing.T) {
	t.Run("base_nil", func(t *testing.T) {
		part := &RuleSet{Version: "v1"}
		merged, err := mergeRuleSetFragments(nil, part)
		require.NoError(t, err)
		assert.Equal(t, "v1", merged.Version)
	})

	t.Run("part_nil", func(t *testing.T) {
		base := &RuleSet{Version: "v1"}
		merged, err := mergeRuleSetFragments(base, nil)
		require.NoError(t, err)
		assert.Equal(t, "v1", merged.Version)
	})

	t.Run("version_mismatch", func(t *testing.T) {
		base := &RuleSet{Version: "v1"}
		part := &RuleSet{Version: "v2"}
		_, err := mergeRuleSetFragments(base, part)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version mismatch")
	})

	t.Run("empty_part_version", func(t *testing.T) {
		base := &RuleSet{Version: "v1"}
		part := &RuleSet{Version: ""}
		_, err := mergeRuleSetFragments(base, part)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version is required")
	})

	t.Run("base_version_empty_uses_part", func(t *testing.T) {
		base := &RuleSet{Version: ""}
		part := &RuleSet{Version: "v1"}
		merged, err := mergeRuleSetFragments(base, part)
		require.NoError(t, err)
		assert.Equal(t, "v1", merged.Version)
	})

	t.Run("options_conflict", func(t *testing.T) {
		base := &RuleSet{Version: "v1", Options: Options{CaseMode: "upper"}}
		part := &RuleSet{Version: "v1", Options: Options{CaseMode: "lower"}}
		_, err := mergeRuleSetFragments(base, part)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "options conflict")
	})

	t.Run("options_from_part_when_base_zero", func(t *testing.T) {
		base := &RuleSet{Version: "v1"}
		part := &RuleSet{Version: "v1", Options: Options{CaseMode: "lower"}}
		merged, err := mergeRuleSetFragments(base, part)
		require.NoError(t, err)
		assert.Equal(t, "lower", merged.Options.CaseMode)
	})

	t.Run("duplicate_rules_across_fragments", func(t *testing.T) {
		base := &RuleSet{
			Version:  "v1",
			Matchers: []MatcherRule{{Name: "m1", Pattern: ".*", NormalizeTemplate: "$0", Score: 80}},
		}
		part := &RuleSet{
			Version:  "v1",
			Matchers: []MatcherRule{{Name: "m1", Pattern: ".*", NormalizeTemplate: "$0", Score: 90}},
		}
		_, err := mergeRuleSetFragments(base, part)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate rule name across fragments")
	})
}

func TestMergeNamedRules(t *testing.T) {
	t.Run("override_replaces_existing", func(t *testing.T) {
		base := []MatcherRule{
			{Name: "m1", Score: 10},
			{Name: "m2", Score: 20},
		}
		override := []MatcherRule{
			{Name: "m1", Score: 99},
		}
		merged := mergeNamedRules(base, override,
			func(v MatcherRule) string { return v.Name },
			func(v MatcherRule) bool { return v.Disabled },
		)
		assert.Len(t, merged, 2)
		assert.Equal(t, 99, merged[0].Score)
	})

	t.Run("override_disables_existing", func(t *testing.T) {
		base := []MatcherRule{
			{Name: "m1", Score: 10},
			{Name: "m2", Score: 20},
		}
		override := []MatcherRule{
			{Name: "m1", Disabled: true},
		}
		merged := mergeNamedRules(base, override,
			func(v MatcherRule) string { return v.Name },
			func(v MatcherRule) bool { return v.Disabled },
		)
		assert.Len(t, merged, 1)
		assert.Equal(t, "m2", merged[0].Name)
	})

	t.Run("override_disables_nonexistent_is_noop", func(t *testing.T) {
		base := []MatcherRule{{Name: "m1", Score: 10}}
		override := []MatcherRule{{Name: "nonexistent", Disabled: true}}
		merged := mergeNamedRules(base, override,
			func(v MatcherRule) string { return v.Name },
			func(v MatcherRule) bool { return v.Disabled },
		)
		assert.Len(t, merged, 1)
	})

	t.Run("empty_name_skipped", func(t *testing.T) {
		base := []MatcherRule{{Name: ""}}
		override := []MatcherRule{{Name: ""}}
		merged := mergeNamedRules(base, override,
			func(v MatcherRule) string { return v.Name },
			func(v MatcherRule) bool { return v.Disabled },
		)
		assert.Empty(t, merged)
	})

	t.Run("override_adds_new", func(t *testing.T) {
		base := []MatcherRule{{Name: "m1", Score: 10}}
		override := []MatcherRule{{Name: "m2", Score: 20}}
		merged := mergeNamedRules(base, override,
			func(v MatcherRule) string { return v.Name },
			func(v MatcherRule) bool { return v.Disabled },
		)
		assert.Len(t, merged, 2)
	})
}

func TestCloneRuleSetNil(t *testing.T) {
	assert.Nil(t, cloneRuleSet(nil))
}

func TestLoaderLoadInvalidYaml(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load([]byte(`not: [valid: yaml`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode yaml rule set failed")
}

func TestLoaderLoadValidationError(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load([]byte(`matchers: []`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule set version is required")
}

func TestLoadRuleSetFromFSReadError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-bad.yaml"), []byte(`{invalid`), 0o600))

	_, err := LoadRuleSetFromDir(dir)
	require.Error(t, err)
}

func TestValidateFragmentVersions(t *testing.T) {
	tests := []struct {
		name    string
		base    *RuleSet
		part    *RuleSet
		wantErr string
	}{
		{
			name:    "matching_versions",
			base:    &RuleSet{Version: "v1"},
			part:    &RuleSet{Version: "v1"},
			wantErr: "",
		},
		{
			name:    "mismatching_versions",
			base:    &RuleSet{Version: "v1"},
			part:    &RuleSet{Version: "v2"},
			wantErr: "version mismatch",
		},
		{
			name:    "empty_part_version",
			base:    &RuleSet{Version: "v1"},
			part:    &RuleSet{Version: ""},
			wantErr: "version is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFragmentVersions(tc.base, tc.part)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestAppendUniqueNamedRules(t *testing.T) {
	t.Run("empty_name_in_extra_is_appended", func(t *testing.T) {
		base := []NormalizerRule{{Name: "n1", Type: "builtin", Builtin: "basename"}}
		extra := []NormalizerRule{{Name: "", Type: "builtin", Builtin: "trim_space"}}
		out, err := appendUniqueNamedRules(base, extra, func(v NormalizerRule) string { return v.Name })
		require.NoError(t, err)
		assert.Len(t, out, 2)
	})

	t.Run("duplicate_name_errors", func(t *testing.T) {
		base := []NormalizerRule{{Name: "n1", Type: "builtin", Builtin: "basename"}}
		extra := []NormalizerRule{{Name: "n1", Type: "builtin", Builtin: "trim_space"}}
		_, err := appendUniqueNamedRules(base, extra, func(v NormalizerRule) string { return v.Name })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate rule name across fragments")
	})
}

func TestLoadRuleSetFromPathBadFileContent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(`{{{invalid yaml`), 0o600))

	_, err := LoadRuleSetFromPath(tmpFile)
	require.Error(t, err)
}
