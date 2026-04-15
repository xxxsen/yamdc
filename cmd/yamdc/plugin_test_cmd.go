package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/searcher"
	pluginbundle "github.com/xxxsen/yamdc/internal/searcher/plugin/bundle"
	pluginyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"github.com/xxxsen/yamdc/internal/store"
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

type pluginCaseFile struct {
	Plugin string            `json:"plugin"`
	Cases  []*pluginCaseItem `json:"cases"`
}

type pluginCaseItem struct {
	Plugin string           `json:"plugin"`
	Name   string           `json:"name"`
	Input  string           `json:"input"`
	Output pluginCaseOutput `json:"output"`
}

type pluginCaseOutput struct {
	Title    string   `json:"title"`
	TagSet   []string `json:"tag_set"`
	ActorSet []string `json:"actor_set"`
	Status   string   `json:"status"`
}

func newPluginTestCmd() *cobra.Command {
	var (
		pluginDir string
		casefile  string
		output    string
	)
	cmd := &cobra.Command{
		Use:   "plugin-test",
		Short: "Verify one plugin bundle directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			res := verifyPluginBundle(strings.TrimSpace(pluginDir), strings.TrimSpace(casefile))
			return renderBundleVerifyResult(os.Stdout, res, output)
		},
	}
	cmd.Flags().StringVar(&pluginDir, "plugin", "", "path to plugin bundle directory")
	cmd.Flags().StringVar(&casefile, "casefile", "", "path to plugin test case json file or directory")
	cmd.Flags().StringVar(&output, "output", "json", "output format, currently only supports json")
	_ = cmd.MarkFlagRequired("plugin")
	return cmd
}

func verifyPluginBundle(pluginDir, casefile string) bundleVerifyResult {
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
	if casefile == "" {
		return bundleVerifyResult{Pass: true}
	}
	rawCases, err := loadPluginCaseFile(casefile)
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	cli, err := client.NewClient(client.WithTimeout(30 * time.Second))
	if err != nil {
		return bundleVerifyResult{Pass: false, Errmsg: err.Error()}
	}
	creators := pluginyaml.BuildRegisterContext(resolved.Plugins).Snapshot()
	debugger := searcher.NewDebugger(cli, store.NewMemStorage(), movieidcleaner.NewPassthroughCleaner(), nil, nil)
	debugger.SwapState(nil, nil, creators)
	out := bundleVerifyResult{
		Pass:  true,
		Cases: make([]*bundleVerifyCaseItem, 0, len(rawCases.Cases)),
	}
	for index, item := range rawCases.Cases {
		res := verifyPluginCase(context.Background(), debugger, resolved, index, item)
		out.Cases = append(out.Cases, res)
		if !res.Pass {
			if out.Pass {
				out.Errmsg = res.Errmsg
			}
			out.Pass = false
		}
	}
	if !out.Pass && out.Errmsg == "" {
		out.Errmsg = "plugin case verification failed"
	}
	return out
}

func loadPluginCaseFile(path string) (*pluginCaseFile, error) {
	if path == "" {
		return nil, errCasefileRequired
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat case file %s failed: %w", path, err)
	}
	if info.IsDir() {
		return loadPluginCaseDir(path)
	}
	return loadPluginCaseJSONFile(path)
}

func loadPluginCaseDir(dir string) (*pluginCaseFile, error) {
	return loadCaseDir(dir, loadPluginCaseJSONFile,
		func(f *pluginCaseFile) int { return len(f.Cases) },
		func(dst, src *pluginCaseFile) { dst.Cases = append(dst.Cases, src.Cases...) },
	)
}

func loadPluginCaseJSONFile(path string) (*pluginCaseFile, error) {
	return loadJSONCaseFile[pluginCaseFile](path, func(out *pluginCaseFile, p string) {
		if strings.TrimSpace(out.Plugin) == "" {
			out.Plugin = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		for _, item := range out.Cases {
			if item == nil {
				continue
			}
			if strings.TrimSpace(item.Plugin) == "" {
				item.Plugin = out.Plugin
			}
		}
	})
}

func verifyPluginCasePrecheck(name string, item *pluginCaseItem) (string, string, *bundleVerifyCaseItem) {
	if item == nil {
		return "", "", &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: "case is null"}
	}
	input := strings.TrimSpace(item.Input)
	if input == "" {
		return "", "", &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: "input is required"}
	}
	pluginName := strings.TrimSpace(item.Plugin)
	if pluginName == "" {
		return "", "", &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: "plugin is required"}
	}
	return input, pluginName, nil
}

func assertPluginCaseOutput(
	name string, item *pluginCaseItem, result *searcher.DebugSearchResult,
) *bundleVerifyCaseItem {
	actualStatus := debugSearchStatus(result)
	if expected := strings.TrimSpace(item.Output.Status); expected != "" && !strings.EqualFold(expected, actualStatus) {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
			"expected status=%s but got %s (%s)", expected, actualStatus, pluginDebugError(result))}
	}
	if expected := strings.TrimSpace(item.Output.Title); expected != "" {
		got := ""
		if result.Meta != nil {
			got = strings.TrimSpace(result.Meta.Title)
		}
		if got != expected {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected title=%s but got %s", expected, got)}
		}
	}
	if len(item.Output.TagSet) != 0 {
		var got []string
		if result.Meta != nil {
			got = result.Meta.Genres
		}
		if !slices.Equal(normalizeStringSet(item.Output.TagSet), normalizeStringSet(got)) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected tag_set=%v but got %v", normalizeStringSet(item.Output.TagSet), normalizeStringSet(got))}
		}
	}
	if len(item.Output.ActorSet) != 0 {
		var got []string
		if result.Meta != nil {
			got = result.Meta.Actors
		}
		if !slices.Equal(normalizeStringSet(item.Output.ActorSet), normalizeStringSet(got)) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf(
				"expected actor_set=%v but got %v", normalizeStringSet(item.Output.ActorSet), normalizeStringSet(got))}
		}
	}
	return nil
}

func verifyPluginCase(
	ctx context.Context,
	debugger *searcher.Debugger,
	resolved *pluginbundle.ResolvedBundle,
	index int,
	item *pluginCaseItem,
) *bundleVerifyCaseItem {
	name := fmt.Sprintf("case-%d", index+1)
	if item != nil && strings.TrimSpace(item.Name) != "" {
		name = strings.TrimSpace(item.Name)
	}
	input, pluginName, fail := verifyPluginCasePrecheck(name, item)
	if fail != nil {
		return fail
	}
	runtimeName, err := resolvePluginRuntimeName(resolved, pluginName)
	if err != nil {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: err.Error()}
	}
	result, err := debugger.DebugSearch(ctx, searcher.DebugSearchOptions{
		Input: input, Plugins: []string{runtimeName}, UseCleaner: false, SkipAssets: true,
	})
	if err != nil {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: err.Error()}
	}
	if fail := assertPluginCaseOutput(name, item, result); fail != nil {
		return fail
	}
	return &bundleVerifyCaseItem{Name: name, Pass: true}
}

func resolvePluginRuntimeName(resolved *pluginbundle.ResolvedBundle, pluginName string) (string, error) {
	if resolved == nil {
		return "", errResolvedBundleNil
	}
	name := strings.TrimSpace(pluginName)
	if name == "" {
		return "", errPluginRequired
	}
	if _, ok := resolved.Plugins[name]; ok {
		return name, nil
	}
	matched := make([]string, 0, 2)
	suffix := "__" + name
	for runtimeName := range resolved.Plugins {
		if strings.HasSuffix(runtimeName, suffix) {
			matched = append(matched, runtimeName)
		}
	}
	slices.Sort(matched)
	if len(matched) == 1 {
		return matched[0], nil
	}
	if len(matched) == 0 {
		return "", fmt.Errorf("plugin %s not found in bundle: %w", name, errPluginNotFoundInBundle)
	}
	return "", fmt.Errorf("plugin %s resolves to multiple runtime chains: %v: %w", name, matched, errMultipleRuntimeChains)
}

func debugSearchStatus(result *searcher.DebugSearchResult) string {
	if result == nil {
		return "error"
	}
	if len(result.PluginResults) != 0 {
		trace := result.PluginResults[0]
		if strings.TrimSpace(trace.Error) != "" {
			return "error"
		}
		if trace.Found {
			return "success"
		}
		return "not_found"
	}
	if result.Found {
		return "success"
	}
	return "not_found"
}

func pluginDebugError(result *searcher.DebugSearchResult) string {
	if result == nil {
		return "nil result"
	}
	for _, trace := range result.PluginResults {
		if strings.TrimSpace(trace.Error) != "" {
			return trace.Error
		}
		for _, step := range trace.Steps {
			if !step.OK && strings.TrimSpace(step.Message) != "" {
				return step.Message
			}
		}
	}
	return "no detail"
}

func renderBundleVerifyResult(out *os.File, result bundleVerifyResult, format string) error {
	if format != "json" {
		return fmt.Errorf("unsupported output format: %s: %w", format, errUnsupportedOutputFormat)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal verify result failed: %w", err)
	}
	if _, err = fmt.Fprintln(out, string(data)); err != nil {
		return fmt.Errorf("write verify result failed: %w", err)
	}
	return nil
}
