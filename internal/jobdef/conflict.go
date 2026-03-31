package jobdef

import (
	"path/filepath"
	"strings"

	"github.com/xxxsen/yamdc/internal/number"
)

func BuildConflictKey(numberText string, fileExt string, fileName string) string {
	numberText = strings.TrimSpace(numberText)
	if numberText == "" {
		return ""
	}
	base := strings.ToUpper(numberText)
	parsed, err := number.Parse(numberText)
	if err == nil && parsed != nil {
		base = strings.ToUpper(parsed.GenerateFileName())
	}
	ext := strings.ToLower(strings.TrimSpace(fileExt))
	if ext == "" {
		ext = strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	}
	if ext == "" {
		return base
	}
	return base + ext
}
