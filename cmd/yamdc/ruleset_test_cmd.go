package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
)

func newRulesetTestCmd() *cobra.Command {
	var (
		ruleset string
		output  string
	)

	cmd := &cobra.Command{
		Use:   "ruleset-test",
		Short: "Verify one ruleset bundle directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			res := verifyRulesetBundle(strings.TrimSpace(ruleset))
			return renderBundleVerifyResult(os.Stdout, res, output)
		},
	}
	cmd.Flags().StringVar(&ruleset, "ruleset", "", "path to ruleset file or directory")
	cmd.Flags().StringVar(&output, "output", "json", "output format, currently only supports json")
	_ = cmd.MarkFlagRequired("ruleset")
	return cmd
}

func verifyRulesetBundle(ruleset string) bundleVerifyResult {
	if ruleset == "" {
		return bundleVerifyResult{Pass: false, Errmsg: "ruleset is required"}
	}
	resolved, err := resolveRuleSourcePath(".", ruleset)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	rs, err := numbercleaner.LoadRuleSetFromPath(resolved)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	if _, err := numbercleaner.NewCleaner(rs); err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	return bundleVerifyResult{Pass: true}
}
