package infra

import (
	"fmt"
	"path/filepath"
)

func NormalizeDirPaths(dirs ...*string) error {
	for _, d := range dirs {
		if d == nil || *d == "" {
			continue
		}
		abs, err := filepath.Abs(*d)
		if err != nil {
			return fmt.Errorf("resolve absolute path %q: %w", *d, err)
		}
		*d = abs
	}
	return nil
}
