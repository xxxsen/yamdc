package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"yamdc/aiengine"
)

type geminiEngine struct {
	c *config
}

func (g *geminiEngine) Name() string {
	return "gemini"
}

func (g *geminiEngine) Complete(ctx context.Context, prompt string, args map[string]interface{}) (string, error) {
	bodyRes := buildRequest(prompt, args)
	raw, err := json.Marshal(bodyRes)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.c.model, g.c.key), bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rsp, err := g.c.c.Do(req)
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
		return "", fmt.Errorf("no translate result found")
	}
	if len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no translate result part found")
	}
	if len(res.Candidates[0].Content.Parts[0].Text) == 0 {
		return "", fmt.Errorf("no translate result text found")
	}
	return res.Candidates[0].Content.Parts[0].Text, nil
}

func NewGeminiEngine(opts ...Option) (aiengine.IAIEngine, error) {
	c := applyOpts(opts...)
	if c.c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	if c.key == "" {
		return nil, fmt.Errorf("key is empty")
	}
	if c.model == "" {
		return nil, fmt.Errorf("model is empty")
	}
	return &geminiEngine{c: c}, nil
}
