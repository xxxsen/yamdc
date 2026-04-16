package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateForCaptureHappyPath(t *testing.T) {
	c := &Config{DataDir: "/data", ScanDir: "/scan", SaveDir: "/save"}
	require.NoError(t, ValidateForCapture(c))
}

func TestValidateForCaptureMissingDataDir(t *testing.T) {
	c := &Config{ScanDir: "/scan", SaveDir: "/save"}
	require.ErrorIs(t, ValidateForCapture(c), ErrNoDataDir)
}

func TestValidateForCaptureMissingScanDir(t *testing.T) {
	c := &Config{DataDir: "/data", SaveDir: "/save"}
	require.ErrorIs(t, ValidateForCapture(c), ErrNoScanDir)
}

func TestValidateForCaptureMissingSaveDir(t *testing.T) {
	c := &Config{DataDir: "/data", ScanDir: "/scan"}
	require.ErrorIs(t, ValidateForCapture(c), ErrNoSaveDir)
}

func TestValidateForCaptureDoesNotRequireLibraryDir(t *testing.T) {
	c := &Config{DataDir: "/data", ScanDir: "/scan", SaveDir: "/save"}
	require.NoError(t, ValidateForCapture(c))
}

func TestValidateForServerHappyPath(t *testing.T) {
	c := &Config{DataDir: "/data", ScanDir: "/scan", SaveDir: "/save", LibraryDir: "/lib"}
	require.NoError(t, ValidateForServer(c))
}

func TestValidateForServerMissingLibraryDir(t *testing.T) {
	c := &Config{DataDir: "/data", ScanDir: "/scan", SaveDir: "/save"}
	require.ErrorIs(t, ValidateForServer(c), ErrNoLibraryDir)
}

func TestValidateForServerMissingCaptureDir(t *testing.T) {
	c := &Config{LibraryDir: "/lib"}
	err := ValidateForServer(c)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoDataDir)
}
