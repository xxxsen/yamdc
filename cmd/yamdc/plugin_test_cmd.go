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
		RunE: func(cmd *cobra.Command, args []string) error {
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

func verifyPluginBundle(pluginDir string, casefile string) bundleVerifyResult {
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
		return nil, fmt.Errorf("casefile is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return loadPluginCaseDir(path)
	}
	return loadPluginCaseJSONFile(path)
}

func loadPluginCaseDir(dir string) (*pluginCaseFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("no json case files found in dir: %s", dir)
	}
	out := &pluginCaseFile{
		Cases: make([]*pluginCaseItem, 0, len(files)),
	}
	for _, file := range files {
		item, err := loadPluginCaseJSONFile(file)
		if err != nil {
			return nil, fmt.Errorf("load case file failed: %s: %w", file, err)
		}
		out.Cases = append(out.Cases, item.Cases...)
	}
	return out, nil
}

func loadPluginCaseJSONFile(path string) (*pluginCaseFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := &pluginCaseFile{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Plugin) == "" {
		out.Plugin = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	for _, item := range out.Cases {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.Plugin) == "" {
			item.Plugin = out.Plugin
		}
	}
	return out, nil
}

func verifyPluginCase(ctx context.Context, debugger *searcher.Debugger, resolved *pluginbundle.ResolvedBundle, index int, item *pluginCaseItem) *bundleVerifyCaseItem {
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
	pluginName := strings.TrimSpace(item.Plugin)
	if pluginName == "" {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: "plugin is required"}
	}
	runtimeName, err := resolvePluginRuntimeName(resolved, pluginName)
	if err != nil {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: err.Error()}
	}
	result, err := debugger.DebugSearch(ctx, searcher.DebugSearchOptions{
		Input:      input,
		Plugins:    []string{runtimeName},
		UseCleaner: false,
		SkipAssets: true,
	})
	if err != nil {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: err.Error()}
	}
	actualStatus := debugSearchStatus(result)
	if expected := strings.TrimSpace(item.Output.Status); expected != "" && !strings.EqualFold(expected, actualStatus) {
		return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf("expected status=%s but got %s (%s)", expected, actualStatus, pluginDebugError(result))}
	}
	if expected := strings.TrimSpace(item.Output.Title); expected != "" {
		got := ""
		if result.Meta != nil {
			got = strings.TrimSpace(result.Meta.Title)
		}
		if got != expected {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf("expected title=%s but got %s", expected, got)}
		}
	}
	if len(item.Output.TagSet) != 0 {
		var got []string
		if result.Meta != nil {
			got = result.Meta.Genres
		}
		if !slices.Equal(normalizeStringSet(item.Output.TagSet), normalizeStringSet(got)) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf("expected tag_set=%v but got %v", normalizeStringSet(item.Output.TagSet), normalizeStringSet(got))}
		}
	}
	if len(item.Output.ActorSet) != 0 {
		var got []string
		if result.Meta != nil {
			got = result.Meta.Actors
		}
		if !slices.Equal(normalizeStringSet(item.Output.ActorSet), normalizeStringSet(got)) {
			return &bundleVerifyCaseItem{Name: name, Pass: false, Errmsg: fmt.Sprintf("expected actor_set=%v but got %v", normalizeStringSet(item.Output.ActorSet), normalizeStringSet(got))}
		}
	}
	return &bundleVerifyCaseItem{Name: name, Pass: true}
}

func resolvePluginRuntimeName(resolved *pluginbundle.ResolvedBundle, pluginName string) (string, error) {
	if resolved == nil {
		return "", fmt.Errorf("resolved plugin bundle is nil")
	}
	name := strings.TrimSpace(pluginName)
	if name == "" {
		return "", fmt.Errorf("plugin is required")
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
		return "", fmt.Errorf("plugin %s not found in bundle", name)
	}
	return "", fmt.Errorf("plugin %s resolves to multiple runtime chains: %v", name, matched)
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
		return fmt.Errorf("unsupported output format: %s", format)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}
