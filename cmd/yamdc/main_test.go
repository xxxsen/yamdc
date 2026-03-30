package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
)

func TestPrecheckCaptureDirDoesNotRequireLibraryDir(t *testing.T) {
	c := &config.Config{
		DataDir: "/tmp/data",
		ScanDir: "/tmp/scan",
		SaveDir: "/tmp/save",
	}
	require.NoError(t, precheckCaptureDir(c))
}

func TestPrecheckServerDirRequiresLibraryDir(t *testing.T) {
	c := &config.Config{
		DataDir: "/tmp/data",
		ScanDir: "/tmp/scan",
		SaveDir: "/tmp/save",
	}
	require.EqualError(t, precheckServerDir(c), "no library dir")
}

func TestBuildNumberCleanerReturnsNonNilManagerOnSuccess(t *testing.T) {
	dataDir := t.TempDir()
	ruleDir := filepath.Join(t.TempDir(), "rules")
	require.NoError(t, os.MkdirAll(ruleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ruleDir, "001-base.yaml"), []byte(`
version: v1
options:
  case_mode: upper
`), 0o644))

	c := &config.Config{
		DataDir: dataDir,
		NumberCleanerConfig: config.NumberCleanerConfig{
			SourceType: numbercleaner.SourceTypeLocal,
			Location:   ruleDir,
		},
	}
	cleaner, manager, err := buildNumberCleaner(context.Background(), client.MustNewClient(), c)
	require.NoError(t, err)
	require.NotNil(t, cleaner)
	require.NotNil(t, manager)
}
