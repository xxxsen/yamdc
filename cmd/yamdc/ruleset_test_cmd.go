package main

import (
	"encoding/json"
	"fmt"
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
		Use:   "ruleset-test <input>",
		Short: "Explain how the number cleaner transforms one input",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			if input == "" {
				return fmt.Errorf("input is required")
			}
			if output != "text" && output != "json" {
				return fmt.Errorf("unsupported output format: %s", output)
			}

			cleaner, err := buildRulesetTestCleaner(ruleset)
			if err != nil {
				return fmt.Errorf("build number cleaner failed: %w", err)
			}
			explain, err := cleaner.Explain(input)
			if err != nil {
				return err
			}
			return renderRulesetExplain(os.Stdout, explain, output)
		},
	}
	cmd.Flags().StringVar(&ruleset, "ruleset", "", "path to ruleset file or directory")
	cmd.Flags().StringVar(&output, "output", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("ruleset")
	return cmd
}

func buildRulesetTestCleaner(ruleset string) (numbercleaner.Cleaner, error) {
	if strings.TrimSpace(ruleset) == "" {
		return nil, fmt.Errorf("ruleset is required")
	}
	resolved, err := resolveRuleSourcePath(".", strings.TrimSpace(ruleset))
	if err != nil {
		return nil, err
	}
	rs, err := numbercleaner.LoadRuleSetFromPath(resolved)
	if err != nil {
		return nil, err
	}
	return numbercleaner.NewCleaner(rs)
}

func renderRulesetExplain(out *os.File, explain *numbercleaner.ExplainResult, format string) error {
	if format == "json" {
		data, err := json.MarshalIndent(explain, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal explain result failed: %w", err)
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}

	if _, err := fmt.Fprintf(out, "Input: %s\n", explain.Input); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "InputNoExt: %s\n", explain.InputNoExt); err != nil {
		return err
	}
	if explain.Final != nil {
		if _, err := fmt.Fprintf(out, "Final: normalized=%s number_id=%s status=%s confidence=%s category=%s uncensor=%t\n",
			explain.Final.Normalized,
			explain.Final.NumberID,
			explain.Final.Status,
			explain.Final.Confidence,
			explain.Final.Category,
			explain.Final.Uncensor,
		); err != nil {
			return err
		}
		if len(explain.Final.Warnings) != 0 {
			if _, err := fmt.Fprintf(out, "Warnings: %s\n", strings.Join(explain.Final.Warnings, ", ")); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(out, "Steps:"); err != nil {
		return err
	}
	for idx, step := range explain.Steps {
		if _, err := fmt.Fprintf(out, "%02d. [%s] %s matched=%t selected=%t\n", idx+1, step.Stage, step.Rule, step.Matched, step.Selected); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "    in : %s\n", step.Input); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "    out: %s\n", step.Output); err != nil {
			return err
		}
		if step.Summary != "" {
			if _, err := fmt.Fprintf(out, "    note: %s\n", step.Summary); err != nil {
				return err
			}
		}
		if len(step.Values) != 0 {
			if _, err := fmt.Fprintf(out, "    values: %s\n", strings.Join(step.Values, ", ")); err != nil {
				return err
			}
		}
		if step.Candidate != nil {
			if _, err := fmt.Fprintf(out, "    candidate: number_id=%s score=%d matcher=%s category=%s uncensor=%t\n",
				step.Candidate.NumberID,
				step.Candidate.Score,
				step.Candidate.Matcher,
				step.Candidate.Category,
				step.Candidate.Uncensor,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
