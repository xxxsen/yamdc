package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
)

type rulesetScoreFileResult struct {
	File   string                    `json:"file"`
	Result []*rulesetScoreLineResult `json:"result"`
}

type rulesetScoreLineResult struct {
	Name    string `json:"name"`
	MovieID string `json:"movieid"`
	Score   string `json:"score"`
}

type rulesetScoreOutput struct {
	Cases []*rulesetScoreFileResult `json:"cases"`
}

func newRulesetScoreCmd() *cobra.Command {
	var (
		ruleset  string
		casefile string
		output   string
	)

	cmd := &cobra.Command{
		Use:     "ruleset-score",
		Aliases: []string{"ruleset_score"},
		Short:   "Score movieid cleaner confidence for txt case files",
		RunE: func(_ *cobra.Command, _ []string) error {
			res, err := scoreRulesetBundle(strings.TrimSpace(ruleset), strings.TrimSpace(casefile))
			if err != nil {
				return err
			}
			return renderRulesetScoreResult(os.Stdout, res, output)
		},
	}
	cmd.Flags().StringVar(&ruleset, "ruleset", "", "path to ruleset file or directory")
	cmd.Flags().StringVar(&casefile, "casefile", "", "path to txt case file or directory")
	cmd.Flags().StringVar(&output, "output", "json", "output format, currently only supports json")
	_ = cmd.MarkFlagRequired("ruleset")
	_ = cmd.MarkFlagRequired("casefile")
	return cmd
}

func scoreRulesetBundle(ruleset, casefile string) (*rulesetScoreOutput, error) {
	if ruleset == "" {
		return nil, errRulesetRequired
	}
	if casefile == "" {
		return nil, errCasefileRequired
	}
	resolved, err := domain.ResolveRuleSourcePath(".", ruleset)
	if err != nil {
		return nil, fmt.Errorf("resolve ruleset path: %w", err)
	}
	rs, err := movieidcleaner.LoadRuleSetFromPath(resolved)
	if err != nil {
		return nil, fmt.Errorf("load ruleset from path failed: %w", err)
	}
	cleaner, err := movieidcleaner.NewCleaner(rs)
	if err != nil {
		return nil, fmt.Errorf("create cleaner failed: %w", err)
	}
	files, err := loadRulesetScoreCaseFile(casefile)
	if err != nil {
		return nil, err
	}
	out := &rulesetScoreOutput{
		Cases: make([]*rulesetScoreFileResult, 0, len(files)),
	}
	for _, item := range files {
		rows := make([]*rulesetScoreLineResult, 0, len(item.Lines))
		for _, line := range item.Lines {
			res, err := cleaner.Clean(line)
			if err != nil {
				return nil, fmt.Errorf("score ruleset case failed: %s: %w", line, err)
			}
			rows = append(rows, &rulesetScoreLineResult{
				Name:    line,
				MovieID: buildScoredMovieID(line, res),
				Score:   string(res.Confidence),
			})
		}
		out.Cases = append(out.Cases, &rulesetScoreFileResult{
			File:   item.File,
			Result: rows,
		})
	}
	return out, nil
}

type rulesetScoreInputFile struct {
	File  string
	Lines []string
}

func loadRulesetScoreCaseFile(path string) ([]*rulesetScoreInputFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat case file %s failed: %w", path, err)
	}
	if info.IsDir() {
		return loadRulesetScoreCaseDir(path)
	}
	item, err := loadRulesetScoreTXTFile(path)
	if err != nil {
		return nil, err
	}
	return []*rulesetScoreInputFile{item}, nil
}

func loadRulesetScoreCaseDir(dir string) ([]*rulesetScoreInputFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read case dir %s failed: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no txt case files found in dir: %s: %w", dir, errNoCaseFilesInDir)
	}
	out := make([]*rulesetScoreInputFile, 0, len(files))
	for _, file := range files {
		item, err := loadRulesetScoreTXTFile(file)
		if err != nil {
			return nil, fmt.Errorf("load txt case file failed: %s: %w", file, err)
		}
		out = append(out, item)
	}
	return out, nil
}

func loadRulesetScoreTXTFile(path string) (*rulesetScoreInputFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read case file %s failed: %w", path, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return &rulesetScoreInputFile{
		File:  path,
		Lines: out,
	}, nil
}

func buildScoredMovieID(raw string, res *movieidcleaner.Result) string {
	if res == nil {
		return ""
	}
	base := strings.TrimSpace(res.NumberID)
	if base == "" {
		base = strings.TrimSpace(res.Normalized)
	}
	if base == "" {
		base = strings.TrimSpace(res.InputNoExt)
	}
	ext := filepath.Ext(strings.TrimSpace(raw))
	if ext != "" && filepath.Ext(base) == "" {
		return base + ext
	}
	return base
}

func renderRulesetScoreResult(out *os.File, result *rulesetScoreOutput, format string) error {
	if format != "json" {
		return fmt.Errorf("unsupported output format: %s: %w", format, errUnsupportedOutputFormat)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal score result failed: %w", err)
	}
	if _, err = fmt.Fprintln(out, string(data)); err != nil {
		return fmt.Errorf("write score result failed: %w", err)
	}
	return nil
}
