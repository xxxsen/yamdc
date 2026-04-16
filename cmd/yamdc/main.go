package main

import (
	"errors"
	"log"

	_ "github.com/xxxsen/yamdc/internal/aiengine/gemini"
	_ "github.com/xxxsen/yamdc/internal/aiengine/ollama"

	"github.com/spf13/cobra"
)

var (
	errCasefileRequired        = errors.New("casefile is required")
	errRulesetRequired         = errors.New("ruleset is required")
	errPluginRequired          = errors.New("plugin is required")
	errResolvedBundleNil       = errors.New("resolved plugin bundle is nil")
	errNoCaseFilesInDir        = errors.New("no case files found in dir")
	errUnsupportedOutputFormat = errors.New("unsupported output format")
	errPluginNotFoundInBundle  = errors.New("plugin not found in bundle")
	errMultipleRuntimeChains   = errors.New("plugin resolves to multiple runtime chains")
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatalf("execute command failed, err:%v", err)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "yamdc",
		Short:         "YAMDC server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newRunCmd(), newServerCmd(), newRulesetTestCmd(), newPluginTestCmd(), newRulesetScoreCmd())
	return cmd
}
