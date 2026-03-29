package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
)

func newSearcherTestCmd() *cobra.Command {
	var (
		configPath string
		plugins    string
		output     string
		useCleaner bool
		ruleset    string
		override   string
	)

	cmd := &cobra.Command{
		Use:   "searcher-test <input>",
		Short: "Debug searcher plugins for one number input",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			if input == "" {
				return fmt.Errorf("input is required")
			}
			if output != "text" && output != "json" {
				return fmt.Errorf("unsupported output format: %s", output)
			}
			c, err := config.Parse(configPath)
			if err != nil {
				return fmt.Errorf("parse config failed: %w", err)
			}
			cli, err := buildHTTPClient(c)
			if err != nil {
				return fmt.Errorf("build http client failed: %w", err)
			}
			var cleaner numbercleaner.Cleaner
			if strings.TrimSpace(ruleset) != "" {
				cleaner, err = buildRulesetTestCleaner("", ruleset, override)
			} else {
				cleaner, _, err = buildNumberCleaner(cli, c)
			}
			if err != nil {
				return fmt.Errorf("build number cleaner failed: %w", err)
			}
			debugger := buildSearcherDebugger(cli, store.NewMemStorage(), cleaner, c)
			result, err := debugger.DebugSearch(cmd.Context(), searcher.DebugSearchOptions{
				Input:      input,
				Plugins:    strings.Split(plugins, ","),
				UseCleaner: useCleaner,
			})
			if err != nil {
				return err
			}
			return renderSearcherDebug(os.Stdout, result, output)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./config.json", "config file")
	cmd.Flags().StringVar(&plugins, "plugins", "", "comma-separated plugin names; empty means use configured chain")
	cmd.Flags().StringVar(&output, "output", "text", "output format: text or json")
	cmd.Flags().BoolVar(&useCleaner, "use-cleaner", true, "run number cleaner before search")
	cmd.Flags().StringVar(&ruleset, "ruleset", "", "override number cleaner ruleset path for the debug command")
	cmd.Flags().StringVar(&override, "override", "", "override number cleaner override path for the debug command")
	return cmd
}

func renderSearcherDebug(out *os.File, result *searcher.DebugSearchResult, format string) error {
	if format == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal searcher debug result failed: %w", err)
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}
	if _, err := fmt.Fprintf(out, "Input: %s\n", result.Input); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "NumberID: %s\n", result.NumberID); err != nil {
		return err
	}
	if result.CleanerResult != nil {
		if _, err := fmt.Fprintf(out, "Cleaner: normalized=%s status=%s category=%s uncensor=%t\n",
			result.CleanerResult.Normalized,
			result.CleanerResult.Status,
			result.CleanerResult.Category,
			result.CleanerResult.Uncensor,
		); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "Plugins: %s\n", strings.Join(result.UsedPlugins, ", ")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "MatchedPlugin: %s\n", result.MatchedPlugin); err != nil {
		return err
	}
	if result.Meta != nil {
		if _, err := fmt.Fprintf(out, "Meta: title=%s release_date=%d source=%s\n", result.Meta.Title, result.Meta.ReleaseDate, result.Meta.ExtInfo.ScrapeInfo.Source); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "PluginSteps:"); err != nil {
		return err
	}
	for _, item := range result.PluginResults {
		if _, err := fmt.Fprintf(out, "- %s found=%t error=%s\n", item.Plugin, item.Found, item.Error); err != nil {
			return err
		}
		for _, step := range item.Steps {
			if _, err := fmt.Fprintf(out, "  [%s] ok=%t status=%d url=%s %s\n", step.Stage, step.OK, step.StatusCode, step.URL, step.Message); err != nil {
				return err
			}
		}
	}
	return nil
}
