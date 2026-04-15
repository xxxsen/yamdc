package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRulesetCaseFileFromJSONFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "cases": [
    {"name":"a","input":"foo","output":{"number":"FOO-1"}}
  ]
}`), 0o600))

	out, err := loadRulesetCaseFile(path)
	require.NoError(t, err)
	require.Len(t, out.Cases, 1)
	require.Equal(t, "a", out.Cases[0].Name)
}

func TestLoadRulesetCaseFileFromDirScansJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001-basic.json"), []byte(`{
  "cases": [
    {"name":"a","input":"foo","output":{"number":"FOO-1"}}
  ]
}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "002-extra.json"), []byte(`{
  "cases": [
    {"name":"b","input":"bar","output":{"status":"no_match"}}
  ]
}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte(`{"cases":[]}`), 0o600))

	out, err := loadRulesetCaseFile(dir)
	require.NoError(t, err)
	require.Len(t, out.Cases, 2)
	require.Equal(t, "a", out.Cases[0].Name)
	require.Equal(t, "b", out.Cases[1].Name)
}

func TestLoadRulesetCaseFileFromDirRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "default.txt"), []byte(`{"cases":[]}`), 0o600))

	_, err := loadRulesetCaseFile(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no json case files found")
}
