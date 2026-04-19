package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/movieidcleaner"
)

func TestLoadRulesetScoreTXTFileSkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.txt")
	require.NoError(t, os.WriteFile(path, []byte("\nabc.com@aaa-0030.mp4\n\nzzz@zzzz.mp4\n"), 0o600))

	out, err := loadRulesetScoreTXTFile(path)
	require.NoError(t, err)
	require.Equal(t, path, out.File)
	require.Equal(t, []string{"abc.com@aaa-0030.mp4", "zzz@zzzz.mp4"}, out.Lines)
}

func TestLoadRulesetScoreCaseDirScansTXT(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.json"), []byte("{}"), 0o600))

	out, err := loadRulesetScoreCaseFile(dir)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, []string{"one"}, out[0].Lines)
	require.Equal(t, []string{"two"}, out[1].Lines)
}

func TestBuildScoredMovieIDKeepsFileExt(t *testing.T) {
	res := buildScoredMovieID("abc.com@aaa-0030.mp4", nil)
	require.Empty(t, res)

	resolved := buildScoredMovieID("abc.com@aaa-0030.mp4", &movieidcleaner.Result{NumberID: "AAA-0030"})
	require.Equal(t, "AAA-0030.mp4", resolved)
}

func TestScoreRulesetBundleOutputsConfidence(t *testing.T) {
	rulesetDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rulesetDir, "ruleset"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesetDir, "manifest.yaml"), []byte("entry: ruleset\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rulesetDir, "ruleset", "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rulesetDir, "ruleset", "006-matchers.yaml"), []byte(`
version: v1
matchers:
  - name: film
    pattern: '(?i)FILM[-_\\s]?([0-9]{3,})'
    normalize_template: 'FILM-$1'
    score: 100
`), 0o600))

	casePath := filepath.Join(t.TempDir(), "cases.txt")
	require.NoError(t, os.WriteFile(casePath, []byte("abc.com@film-123.mp4\nzzz@unknown.mp4\n"), 0o600))

	out, err := scoreRulesetBundle(rulesetDir, casePath)
	require.NoError(t, err)
	require.Len(t, out.Cases, 1)
	require.Equal(t, casePath, out.Cases[0].File)
	require.Len(t, out.Cases[0].Result, 2)
	require.Equal(t, "abc.com@film-123.mp4", out.Cases[0].Result[0].Name)
	require.Equal(t, "FILM-123.mp4", out.Cases[0].Result[0].MovieID)
	require.Equal(t, "high", out.Cases[0].Result[0].Score)
	require.Equal(t, "zzz@unknown.mp4", out.Cases[0].Result[1].Name)
	require.Equal(t, "zzz@unknown.mp4", out.Cases[0].Result[1].MovieID)
	require.Equal(t, "low", out.Cases[0].Result[1].Score)
}

func TestRenderRulesetScoreResultJSON(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "score-*.json")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	result := &rulesetScoreOutput{
		Cases: []*rulesetScoreFileResult{
			{
				File: "a.txt",
				Result: []*rulesetScoreLineResult{
					{Name: "film-1.mp4", MovieID: "FILM-1.mp4", Score: "high"},
				},
			},
		},
	}
	require.NoError(t, renderRulesetScoreResult(file, result, "json"))

	raw, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	decoded := &rulesetScoreOutput{}
	require.NoError(t, json.Unmarshal(raw, decoded))
	require.Len(t, decoded.Cases, 1)
	require.Equal(t, "a.txt", decoded.Cases[0].File)
	require.Equal(t, "FILM-1.mp4", decoded.Cases[0].Result[0].MovieID)
}
