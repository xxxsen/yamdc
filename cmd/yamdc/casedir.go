package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func collectJSONCaseFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read case dir %s failed: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no json case files found in dir: %s: %w", dir, errNoCaseFilesInDir)
	}
	return files, nil
}

func loadCaseDir[T any](
	dir string, loadFile func(string) (*T, error), _ func(*T) int, appendCases func(dst, src *T),
) (*T, error) {
	files, err := collectJSONCaseFiles(dir)
	if err != nil {
		return nil, err
	}
	var out *T
	for _, file := range files {
		item, err := loadFile(file)
		if err != nil {
			return nil, fmt.Errorf("load case file failed: %s: %w", file, err)
		}
		if out == nil {
			out = item
			continue
		}
		appendCases(out, item)
	}
	return out, nil
}

func loadJSONCaseFile[T any](path string, postprocess func(*T, string)) (*T, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read case file %s failed: %w", path, err)
	}
	out := new(T)
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("unmarshal case file %s failed: %w", path, err)
	}
	if postprocess != nil {
		postprocess(out, path)
	}
	return out, nil
}
