package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
)

type bundleVerifyResult struct {
	Pass   bool                    `json:"pass"`
	Errmsg string                  `json:"errmsg"`
	Cases  []*bundleVerifyCaseItem `json:"cases,omitempty"`
}

type bundleVerifyCaseItem struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Errmsg string `json:"errmsg"`
}

func newPluginTestCmd() *cobra.Command {
	var (
		pluginDir string
		output    string
	)
	cmd := &cobra.Command{
		Use:   "plugin-test",
		Short: "Verify one plugin bundle directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			res := verifyPluginBundle(strings.TrimSpace(pluginDir))
			return renderBundleVerifyResult(os.Stdout, res, output)
		},
	}
	cmd.Flags().StringVar(&pluginDir, "plugin", "", "path to plugin bundle directory")
	cmd.Flags().StringVar(&output, "output", "json", "output format, currently only supports json")
	_ = cmd.MarkFlagRequired("plugin")
	return cmd
}

func verifyPluginBundle(pluginDir string) bundleVerifyResult {
	if pluginDir == "" {
		return bundleVerifyResult{Pass: false, Errmsg: "plugin is required"}
	}
	resolved, _, err := pluginbundle.LoadResolvedBundleFromDir(pluginDir)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	if len(resolved.Warnings) != 0 {
		return bundleVerifyResult{Pass: false, Errmsg: strings.Join(resolved.Warnings, "; ")}
	}
	return bundleVerifyResult{Pass: true}
}

func renderBundleVerifyResult(out *os.File, result bundleVerifyResult, format string) error {
	if format != "json" {
		return fmt.Errorf("unsupported output format: %s", format)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}
