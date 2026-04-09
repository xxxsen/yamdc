package editor

import (
	"context"
	"fmt"

	"github.com/xxxsen/yamdc/internal/client"
	plugyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"gopkg.in/yaml.v3"
)

type Service struct {
	cli client.IHTTPClient
}

func NewService(cli client.IHTTPClient) (*Service, error) {
	if cli == nil {
		return nil, fmt.Errorf("http client is required")
	}
	return &Service{cli: cli}, nil
}

func (s *Service) Compile(_ context.Context, draft *plugyaml.PluginSpec) (*plugyaml.CompileResult, error) {
	return plugyaml.CompileDraft(draft)
}

func (s *Service) RequestDebug(ctx context.Context, draft *plugyaml.PluginSpec, number string) (*plugyaml.RequestDebugResult, error) {
	return plugyaml.DebugRequest(ctx, s.cli, draft, number)
}

func (s *Service) ScrapeDebug(ctx context.Context, draft *plugyaml.PluginSpec, number string) (*plugyaml.ScrapeDebugResult, error) {
	return plugyaml.DebugScrape(ctx, s.cli, draft, number)
}

func (s *Service) WorkflowDebug(ctx context.Context, draft *plugyaml.PluginSpec, number string) (*plugyaml.WorkflowDebugResult, error) {
	return plugyaml.DebugWorkflow(ctx, s.cli, draft, number)
}

func (s *Service) CaseDebug(ctx context.Context, draft *plugyaml.PluginSpec, spec plugyaml.CaseSpec) (*plugyaml.CaseDebugResult, error) {
	return plugyaml.DebugCase(ctx, s.cli, draft, spec)
}

func (s *Service) ImportYAML(_ context.Context, raw string) (*plugyaml.PluginSpec, error) {
	var draft plugyaml.PluginSpec
	if err := yaml.Unmarshal([]byte(raw), &draft); err != nil {
		return nil, fmt.Errorf("decode yaml draft failed, err:%w", err)
	}
	if _, err := plugyaml.CompileDraft(&draft); err != nil {
		return nil, err
	}
	return &draft, nil
}
