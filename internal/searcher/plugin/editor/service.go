package editor

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/xxxsen/yamdc/internal/client"
	plugyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
)

var errHTTPClientRequired = errors.New("http client is required")

type Service struct {
	cli client.IHTTPClient
}

func NewService(cli client.IHTTPClient) (*Service, error) {
	if cli == nil {
		return nil, errHTTPClientRequired
	}
	return &Service{cli: cli}, nil
}

func (s *Service) Compile(_ context.Context, draft *plugyaml.PluginSpec) (*plugyaml.CompileResult, error) {
	res, err := plugyaml.CompileDraft(draft)
	if err != nil {
		return nil, fmt.Errorf("compile draft: %w", err)
	}
	return res, nil
}

func (s *Service) RequestDebug(ctx context.Context, draft *plugyaml.PluginSpec, number string) (
	*plugyaml.RequestDebugResult,
	error,
) {
	res, err := plugyaml.DebugRequest(ctx, s.cli, draft, number)
	if err != nil {
		return nil, fmt.Errorf("debug request: %w", err)
	}
	return res, nil
}

func (s *Service) ScrapeDebug(ctx context.Context, draft *plugyaml.PluginSpec, number string) (
	*plugyaml.ScrapeDebugResult,
	error,
) {
	res, err := plugyaml.DebugScrape(ctx, s.cli, draft, number)
	if err != nil {
		return nil, fmt.Errorf("debug scrape: %w", err)
	}
	return res, nil
}

func (s *Service) WorkflowDebug(ctx context.Context, draft *plugyaml.PluginSpec, number string) (
	*plugyaml.WorkflowDebugResult,
	error,
) {
	res, err := plugyaml.DebugWorkflow(ctx, s.cli, draft, number)
	if err != nil {
		return nil, fmt.Errorf("debug workflow: %w", err)
	}
	return res, nil
}

func (s *Service) CaseDebug(ctx context.Context, draft *plugyaml.PluginSpec, spec plugyaml.CaseSpec) (
	*plugyaml.CaseDebugResult,
	error,
) {
	res, err := plugyaml.DebugCase(ctx, s.cli, draft, spec)
	if err != nil {
		return nil, fmt.Errorf("debug case: %w", err)
	}
	return res, nil
}

func (s *Service) ImportYAML(_ context.Context, raw string) (*plugyaml.PluginSpec, error) {
	var draft plugyaml.PluginSpec
	if err := yaml.Unmarshal([]byte(raw), &draft); err != nil {
		return nil, fmt.Errorf("decode yaml draft failed, err:%w", err)
	}
	if _, err := plugyaml.CompileDraft(&draft); err != nil {
		return nil, fmt.Errorf("compile imported draft: %w", err)
	}
	return &draft, nil
}
