package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/config"
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
