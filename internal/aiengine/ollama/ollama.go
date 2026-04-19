package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/xxxsen/common/utils"

	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/client"
)

const (
	defaultOllamaEngineName = "ollama"
)

var (
	errOllamaResponseErr = errors.New("ollama response error")
	errOllamaNoResult    = errors.New("no result returned from ollama")
	errOllamaHostEmpty   = errors.New("host is empty")
	errOllamaModelEmpty  = errors.New("model is empty")
)

type ollamaEngine struct {
	c *config
}

func (g *ollamaEngine) Name() string {
	return defaultOllamaEngineName
}

func (g *ollamaEngine) Complete(ctx context.Context, prompt string, args map[string]any) (string, error) {
	bodyReq := buildRequest(prompt, args, g.c.Model)
	raw, err := json.Marshal(bodyReq)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request failed: %w", err)
	}
	host := strings.TrimRight(g.c.Host, "/")
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/api/generate", host),
		bytes.NewReader(raw),
	)
	if err != nil {
		return "", fmt.Errorf("create ollama request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	rsp, err := g.c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama http request failed: %w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama response err, code:%d: %w", rsp.StatusCode, errOllamaResponseErr)
	}
	var res Response
	if err := json.NewDecoder(rsp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decode ollama response failed: %w", err)
	}
	if len(res.Error) > 0 {
		return "", fmt.Errorf("ollama response err:%s: %w", res.Error, errOllamaResponseErr)
	}
	content := strings.TrimSpace(res.Response)
	if len(content) == 0 {
		return "", errOllamaNoResult
	}
	return content, nil
}

func New(opts ...Option) (aiengine.IAIEngine, error) {
	c := applyOpts(opts...)
	return newOllamaEngine(c)
}

func newOllamaEngine(c *config) (*ollamaEngine, error) {
	if len(c.Host) == 0 {
		return nil, errOllamaHostEmpty
	}
	if len(c.Model) == 0 {
		return nil, errOllamaModelEmpty
	}
	if c.HTTPClient == nil {
		c.HTTPClient = client.MustNewClient()
	}
	return &ollamaEngine{c: c}, nil
}

func createOllamaEngine(args any, opts ...aiengine.CreateOption) (aiengine.IAIEngine, error) {
	c := &config{}
	if err := utils.ConvStructJson(args, c); err != nil {
		return nil, fmt.Errorf("parse ollama config failed: %w", err)
	}
	createCfg := aiengine.ResolveCreateConfig(opts...)
	if createCfg.HTTPClient != nil {
		c.HTTPClient = createCfg.HTTPClient
	}
	return newOllamaEngine(c)
}

func init() {
	aiengine.Register(defaultOllamaEngineName, createOllamaEngine)
}
