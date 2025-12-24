package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/client"

	"github.com/xxxsen/common/utils"
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

func (g *geminiEngine) Complete(ctx context.Context, prompt string, args map[string]interface{}) (string, error) {
	bodyRes := buildRequest(prompt, args)
	raw, err := json.Marshal(bodyRes)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.c.Model, g.c.Key), bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rsp, err := client.DefaultClient().Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini response err, code:%d", rsp.StatusCode)
	}
	var res Response
	if err := json.NewDecoder(rsp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res.Candidates) == 0 {
		return "", fmt.Errorf("no translate result found, maybe blocked, prompt feedback:%s", res.PromptFeedback.BlockReason)
	}
	if len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no translate result part found, reason:%s", res.Candidates[0].FinishReason)
	}
	content := strings.TrimSpace(res.Candidates[0].Content.Parts[0].Text)
	if len(content) == 0 {
		return "", fmt.Errorf("no translate result text found")
	}
	return content, nil
}

func New(opts ...Option) (aiengine.IAIEngine, error) {
	c := applyOpts(opts...)
	return newGeminiEngine(c)
}

func newGeminiEngine(c *config) (*geminiEngine, error) {
	if c.Key == "" {
		return nil, fmt.Errorf("key is empty")
	}
	if c.Model == "" {
		return nil, fmt.Errorf("model is empty")
	}
	return &geminiEngine{c: c}, nil
}

func createGeminiEngine(args interface{}) (aiengine.IAIEngine, error) {
	c := &config{}
	if err := utils.ConvStructJson(args, c); err != nil {
		return nil, err
	}
	return newGeminiEngine(c)
}

func init() {
	aiengine.Register(defaultGeminiEngineName, createGeminiEngine)
}
