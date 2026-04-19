package gemini

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

var (
	errGeminiResponseErr     = errors.New("gemini response error")
	errNoTranslateResult     = errors.New("no translate result found")
	errNoTranslateResultPart = errors.New("no translate result part found")
	errNoTranslateResultText = errors.New("no translate result text found")
	errKeyEmpty              = errors.New("key is empty")
	errModelEmpty            = errors.New("model is empty")
)

const (
	defaultGeminiEngineName = "gemini"
)

type geminiEngine struct {
	c *config
}

func (g *geminiEngine) Name() string {
	return defaultGeminiEngineName
}

func (g *geminiEngine) Complete(ctx context.Context, prompt string, args map[string]any) (string, error) {
	bodyRes := buildRequest(prompt, args)
	raw, err := json.Marshal(bodyRes)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
			g.c.Model, g.c.Key,
		),
		bytes.NewReader(raw),
	)
	if err != nil {
		return "", fmt.Errorf("create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rsp, err := g.c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute http request: %w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w, code: %d", errGeminiResponseErr, rsp.StatusCode)
	}
	var res Response
	if err := json.NewDecoder(rsp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(res.Candidates) == 0 {
		return "", fmt.Errorf("%w, maybe blocked, prompt feedback: %s",
			errNoTranslateResult, res.PromptFeedback.BlockReason)
	}
	if len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("%w, reason: %s", errNoTranslateResultPart, res.Candidates[0].FinishReason)
	}
	content := strings.TrimSpace(res.Candidates[0].Content.Parts[0].Text)
	if len(content) == 0 {
		return "", errNoTranslateResultText
	}
	return content, nil
}

func New(opts ...Option) (aiengine.IAIEngine, error) {
	c := applyOpts(opts...)
	return newGeminiEngine(c)
}

func newGeminiEngine(c *config) (*geminiEngine, error) {
	if c.Key == "" {
		return nil, errKeyEmpty
	}
	if c.Model == "" {
		return nil, errModelEmpty
	}
	if c.HTTPClient == nil {
		c.HTTPClient = client.MustNewClient()
	}
	return &geminiEngine{c: c}, nil
}

func createGeminiEngine(args any, opts ...aiengine.CreateOption) (aiengine.IAIEngine, error) {
	c := &config{}
	if err := utils.ConvStructJson(args, c); err != nil {
		return nil, fmt.Errorf("convert gemini config: %w", err)
	}
	createCfg := aiengine.ResolveCreateConfig(opts...)
	if createCfg.HTTPClient != nil {
		c.HTTPClient = createCfg.HTTPClient
	}
	return newGeminiEngine(c)
}

func init() {
	aiengine.Register(defaultGeminiEngineName, createGeminiEngine)
}
