package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxxsen/yamdc/internal/bootstrap/domain"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
)

type rulesetCaseFile struct {
	Cases []*rulesetCaseItem `json:"cases"`
}

type rulesetCaseItem struct {
	Name   string            `json:"name"`
	Input  string            `json:"input"`
	Output rulesetCaseOutput `json:"output"`
}

type rulesetCaseOutput struct {
	Number   string   `json:"number"`
	Uncensor *bool    `json:"uncensor"`
	Suffixes []string `json:"suffix-set"`
	Category string   `json:"category"`
	Status   string   `json:"status"`
}

func newRulesetTestCmd() *cobra.Command {
	var (
		ruleset  string
		casefile string
		output   string
	)

	cmd := &cobra.Command{
		Use:   "ruleset-test",
		Short: "Verify one ruleset bundle directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			res := verifyRulesetBundle(strings.TrimSpace(ruleset), strings.TrimSpace(casefile))
			return renderBundleVerifyResult(os.Stdout, res, output)
		},
	}
	cmd.Flags().StringVar(&ruleset, "ruleset", "", "path to ruleset file or directory")
	cmd.Flags().StringVar(&casefile, "casefile", "", "path to ruleset test case json file")
	cmd.Flags().StringVar(&output, "output", "json", "output format, currently only supports json")
	_ = cmd.MarkFlagRequired("ruleset")
	return cmd
}

func verifyRulesetBundle(ruleset, casefile string) bundleVerifyResult {
	if ruleset == "" {
		return bundleVerifyResult{Pass: false, Errmsg: "ruleset is required"}
	}
	resolved, err := domain.ResolveRuleSourcePath(".", ruleset)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	rs, err := movieidcleaner.LoadRuleSetFromPath(resolved)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	if _, err := movieidcleaner.NewCleaner(rs); err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	if casefile == "" {
		return bundleVerifyResult{Pass: true}
	}
	rawCases, err := loadRulesetCaseFile(casefile)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	cleaner, err := movieidcleaner.NewCleaner(rs)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	out := bundleVerifyResult{
		Pass:  true,
		Cases: make([]*bundleVerifyCaseItem, 0, len(rawCases.Cases)),
	}
	for index, item := range rawCases.Cases {
		res := verifyRulesetCase(cleaner, index, item)
		out.Cases = append(out.Cases, res)
		if !res.Pass {
			if out.Pass {
				out.Errmsg = res.Errmsg
			}
			out.Pass = false
		}
	}
	if !out.Pass && out.Errmsg == "" {
		out.Errmsg = "ruleset case verification failed"
	}
	return out
}

func loadRulesetCaseFile(path string) (*rulesetCaseFile, error) {
	if path == "" {
		return nil, errCasefileRequired
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat case file %s failed: %w", path, err)
	}
	if info.IsDir() {
		return loadRulesetCaseDir(path)
	}
	return loadRulesetCaseJSONFile(path)
}

func loadRulesetCaseDir(dir string) (*rulesetCaseFile, error) {
	return loadCaseDir(dir, loadRulesetCaseJSONFile,
		func(f *rulesetCaseFile) int { return len(f.Cases) },
		func(dst, src *rulesetCaseFile) { dst.Cases = append(dst.Cases, src.Cases...) },
	)
}

func loadRulesetCaseJSONFile(path string) (*rulesetCaseFile, error) {
	return loadJSONCaseFile[rulesetCaseFile](path, nil)
}

func assertRulesetCaseOutput(name string, item *rulesetCaseItem, res *movieidcleaner.Result) *bundleVerifyCaseItem {
	if expected := strings.TrimSpace(item.Output.Number); expected != "" {
		if !strings.EqualFold(res.NumberID, expected) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected number=%s but got %s", expected, res.NumberID)}
		}
	}
	if item.Output.Uncensor != nil && res.Uncensor != *item.Output.Uncensor {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
			"expected uncensor=%t but got %t", *item.Output.Uncensor, res.Uncensor)}
	}
	if expected := strings.TrimSpace(item.Output.Category); expected != "" {
		if !strings.EqualFold(res.Category, expected) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected category=%s but got %s", expected, res.Category)}
		}
	}
	if expected := strings.TrimSpace(item.Output.Status); expected != "" {
		if !strings.EqualFold(string(res.Status), expected) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected status=%s but got %s", expected, res.Status)}
		}
	}
	if len(item.Output.Suffixes) != 0 {
		exp := normalizeStringSet(item.Output.Suffixes)
		got := normalizeStringSet(res.Suffixes)
		if !slices.Equal(exp, got) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected suffix-set=%v but got %v", exp, got)}
		}
	}
	return nil
}

func verifyRulesetCase(cleaner movieidcleaner.Cleaner, index int, item *rulesetCaseItem) *bundleVerifyCaseItem {
	name := fmt.Sprintf("case-%d", index+1)
	if item != nil && strings.TrimSpace(item.Name) != "" {
		name = strings.TrimSpace(item.Name)
	}
	if item == nil {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: "case is null"}
	}
	input := strings.TrimSpace(item.Input)
	if input == "" {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: "input is required"}
	}
	res, err := cleaner.Clean(input)
	if err != nil {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: err.Error()}
	}
	if fail := assertRulesetCaseOutput(name, item, res); fail != nil {
		return fail
	}
	return &bundleVerifyCaseItem{Name: name, Pass: true}
}

func normalizeStringSet(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.ToUpper(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	slices.Sort(out)
	return out
}
