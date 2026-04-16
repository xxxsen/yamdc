package config

import "errors"

var (
	ErrNoDataDir    = errors.New("no data dir")
	ErrNoScanDir    = errors.New("no scan dir")
	ErrNoSaveDir    = errors.New("no save dir")
	ErrNoLibraryDir = errors.New("no library dir")
)

// ValidateForCapture checks that all directories required for capture mode are set.
func ValidateForCapture(c *Config) error {
	if len(c.DataDir) == 0 {
		return ErrNoDataDir
	}
	if len(c.ScanDir) == 0 {
		return ErrNoScanDir
	}
	if len(c.SaveDir) == 0 {
		return ErrNoSaveDir
	}
	return nil
}

// ValidateForServer checks all directories required for server mode (superset of capture).
func ValidateForServer(c *Config) error {
	if err := ValidateForCapture(c); err != nil {
		return err
	}
	if len(c.LibraryDir) == 0 {
		return ErrNoLibraryDir
	}
	return nil
}
