package utils

import (
	"path/filepath"
	"strings"
)

func IsVideoFile(f string) bool {
	ext := strings.ToLower(filepath.Ext(f))
	switch ext {
	case ".mp4", ".wav", ".avi", ".mkv", ".rmvb", ".m4a", ".ts":
		return true
	default:
		return false
	}
}

func GetExtName(f string, def string) string {
	if v := filepath.Ext(f); v != "" {
		return v
	}
	return def
}
